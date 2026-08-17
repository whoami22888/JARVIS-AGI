from __future__ import annotations

import os
from pathlib import Path
from typing import Annotated, Literal

from fastapi import Depends, FastAPI, Header, HTTPException, Query, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel, Field

from engine import AgentService, EventHub, HostProfiler, MemoryStore, ModelRegistry
from features import CapabilityStore, FeatureError, KnowledgeVault, ReleaseManifest, SessionStore, SkillRegistry
from performance import BenchmarkStore, IndexQueue, StorageInspector, working_set_policy

ROOT = Path(os.environ.get("USB_AGENT_HOME", Path(__file__).resolve().parents[1])).resolve()
FRONTEND_DIR = ROOT / "frontend"

store = MemoryStore()
registry = ModelRegistry()
events = EventHub()
agent = AgentService(store, registry, events)
skills = SkillRegistry()
capabilities = CapabilityStore()
knowledge = KnowledgeVault()
integrity = ReleaseManifest()
sessions = SessionStore()
storage = StorageInspector()
index_queue = IndexQueue()
benchmarks = BenchmarkStore()
AUTH_REQUIRED = os.environ.get("USB_AGENT_REQUIRE_TOKEN", "0") == "1"
if os.environ.get("USB_AGENT_PAIRING_CODE"):
    sessions.ensure_configured_from_environment()

app = FastAPI(title="USB Agent Gateway", version="0.2.0")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://127.0.0.1", "http://localhost"],
    allow_credentials=False,
    allow_methods=["GET", "POST", "DELETE"],
    allow_headers=["Content-Type", "Authorization"],
)
app.mount("/assets", StaticFiles(directory=FRONTEND_DIR), name="assets")


def model_payload(value: BaseModel) -> dict:
    return value.model_dump() if hasattr(value, "model_dump") else value.dict()


def bearer_token(authorization: str | None) -> str:
    if not authorization or not authorization.startswith("Bearer "):
        return ""
    return authorization.removeprefix("Bearer ").strip()


def require_token(authorization: str | None = Header(default=None)) -> None:
    if not AUTH_REQUIRED:
        return
    if not sessions.verify(bearer_token(authorization)):
        raise HTTPException(status_code=401, detail="A valid, unexpired gateway session is required.")


def approved_roots() -> list[Path]:
    configured = [part.strip() for part in os.environ.get("USB_AGENT_APPROVED_ROOTS", "").split(os.pathsep) if part.strip()]
    return [Path(item).resolve() for item in configured if Path(item).exists()]


class TaskRequest(BaseModel):
    prompt: str = Field(min_length=1, max_length=20_000)
    requested_model: str | None = Field(default=None, max_length=100)
    template_id: str | None = Field(default=None, max_length=100)


class ApprovalRequest(BaseModel):
    kind: Literal["elevated_windows_action", "android_action", "capability_request"]
    title: str = Field(min_length=3, max_length=160)
    reason: str = Field(min_length=3, max_length=500)
    details: dict[str, str] = Field(default_factory=dict)


class ApprovalDecision(BaseModel):
    accept: bool


class PairingRequest(BaseModel):
    code: str = Field(min_length=8, max_length=128)
    label: str = Field(default="paired-device", min_length=1, max_length=120)
    ttl_minutes: int = Field(default=60, ge=5, le=1440)


class CapabilityRequest(BaseModel):
    capability: Literal["read_files", "write_files", "network_read", "run_tests", "elevated_diagnostics"]
    target: str = Field(min_length=1, max_length=1000)
    reason: str = Field(min_length=3, max_length=1000)
    ttl_minutes: int = Field(default=30, ge=1, le=1440)


class KnowledgeIndexRequest(BaseModel):
    path: str = Field(min_length=1, max_length=2000)


class IndexQueueRequest(BaseModel):
    root: str = Field(min_length=1, max_length=2000)
    max_files: int = Field(default=500, ge=1, le=5000)


@app.get("/", include_in_schema=False)
async def dashboard() -> FileResponse:
    return FileResponse(FRONTEND_DIR / "index.html")


@app.get("/health")
async def health() -> dict:
    return {"status": "ok", "profile": HostProfiler.sample(), "data_path": str(store.path), "auth_required": AUTH_REQUIRED}


@app.post("/api/pair")
async def pair(request: PairingRequest) -> dict:
    try:
        return sessions.issue(request.code, request.label, request.ttl_minutes)
    except FeatureError as exc:
        raise HTTPException(status_code=401 if "Invalid" in str(exc) else 503, detail=str(exc)) from exc


@app.get("/api/status", dependencies=[Depends(require_token)])
async def status() -> dict:
    profile = HostProfiler.profile()
    return {
        "host": HostProfiler.sample(),
        "working_set": working_set_policy(profile.memory_total_mb, profile.name).__dict__,
        "storage": storage.snapshot(),
        "index_queue": index_queue.summary(),
        "benchmarks": benchmarks.recent(8),
        "models": await registry.status(),
        "tasks": store.list_tasks(limit=30),
        "skills": skills.manifests(),
        "integrity": integrity.verify(),
        "capabilities": capabilities.status(),
        "sessions": sessions.active() if AUTH_REQUIRED else [],
        "offline_only": os.environ.get("USB_AGENT_OFFLINE_ONLY", "0") == "1",
        "approved_roots": [str(item) for item in approved_roots()],
    }


@app.post("/api/tasks", status_code=202, dependencies=[Depends(require_token)])
async def create_task(request: TaskRequest) -> dict:
    try:
        return await agent.start_task(request.prompt, request.requested_model)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc


@app.get("/api/tasks/{task_id}", dependencies=[Depends(require_token)])
async def get_task(task_id: str) -> dict:
    task = store.get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    return task


@app.post("/api/approvals", status_code=201, dependencies=[Depends(require_token)])
async def create_approval(request: ApprovalRequest) -> dict:
    approval = store.create_approval(model_payload(request))
    await events.broadcast({"type": "approval", **approval})
    return approval


@app.post("/api/approvals/{approval_id}", dependencies=[Depends(require_token)])
async def decide_approval(approval_id: str, decision: ApprovalDecision) -> dict:
    approval = store.decide_approval(approval_id, decision.accept)
    if approval is None:
        raise HTTPException(status_code=404, detail="Approval request not found")
    await events.broadcast({"type": "approval", **approval})
    return approval


@app.get("/api/capabilities", dependencies=[Depends(require_token)])
async def list_capabilities() -> dict:
    return {"grants": capabilities.status()}


@app.post("/api/capabilities", status_code=201, dependencies=[Depends(require_token)])
async def grant_capability(request: CapabilityRequest) -> dict:
    try:
        grant = capabilities.grant(**model_payload(request))
    except FeatureError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    await events.broadcast({"type": "capability", "state": "APPROVED", "grant": grant})
    return grant


@app.delete("/api/capabilities/{grant_id}", dependencies=[Depends(require_token)])
async def revoke_capability(grant_id: str) -> dict:
    if not capabilities.revoke(grant_id):
        raise HTTPException(status_code=404, detail="Active capability grant not found")
    await events.broadcast({"type": "capability", "state": "REVOKED", "grant_id": grant_id})
    return {"id": grant_id, "state": "REVOKED"}


@app.get("/api/knowledge/search", dependencies=[Depends(require_token)])
async def search_knowledge(query: Annotated[str, Query(min_length=1, max_length=500)]) -> dict:
    try:
        return {"results": knowledge.search(query)}
    except FeatureError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc


@app.post("/api/knowledge/index", dependencies=[Depends(require_token)])
async def index_knowledge(request: KnowledgeIndexRequest) -> dict:
    roots = approved_roots()
    if not roots:
        raise HTTPException(status_code=409, detail="Configure USB_AGENT_APPROVED_ROOTS and approve the selected root before indexing.")
    target = Path(request.path).resolve()
    matching_root = next((root for root in roots if root in target.parents or target == root), None)
    if matching_root is None or not capabilities.active("read_files", str(matching_root)):
        raise HTTPException(status_code=403, detail="The approved root requires an active read_files capability grant.")
    try:
        return knowledge.index_file(target, roots)
    except FeatureError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc


@app.get("/api/integrity", dependencies=[Depends(require_token)])
async def verify_integrity() -> dict:
    return integrity.verify()


@app.post("/api/index/queue", dependencies=[Depends(require_token)])
async def queue_indexing(request: IndexQueueRequest) -> dict:
    roots = approved_roots()
    root = Path(request.root).resolve()
    if root not in roots or not capabilities.active("read_files", str(root)):
        raise HTTPException(status_code=403, detail="The selected root requires an active read_files capability grant.")
    try:
        result = index_queue.enqueue_tree(root, request.max_files)
    except FeatureError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    await events.broadcast({"type": "index_queue", **result, "summary": index_queue.summary()})
    return {**result, "summary": index_queue.summary()}


@app.post("/api/index/run", dependencies=[Depends(require_token)])
async def run_indexing_batch() -> dict:
    roots = approved_roots()
    processed = []
    for item in index_queue.next_batch():
        candidate = Path(item["path"])
        try:
            result = knowledge.index_file(candidate, roots)
            index_queue.complete(item["path"], content_hash=result.get("content_hash", ""))
            processed.append(result)
        except (FeatureError, OSError) as exc:
            index_queue.complete(item["path"], error=str(exc))
    payload = {"processed": processed, "summary": index_queue.summary()}
    benchmarks.record("index_batch", {"count": len(processed), "summary": payload["summary"]})
    await events.broadcast({"type": "index_queue", **payload})
    return payload


@app.websocket("/ws/events")
async def event_stream(websocket: WebSocket, token: str = Query(default="")) -> None:
    if AUTH_REQUIRED and not sessions.verify(token):
        await websocket.close(code=1008)
        return
    await events.connect(websocket)
    try:
        await websocket.send_json({"type": "connected", "host": HostProfiler.sample(), "integrity": integrity.verify()})
        while True:
            await websocket.receive_text()
    except WebSocketDisconnect:
        events.disconnect(websocket)
    except Exception:
        events.disconnect(websocket)
