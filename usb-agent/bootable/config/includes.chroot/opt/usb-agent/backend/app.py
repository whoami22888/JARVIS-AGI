from __future__ import annotations

import os
from pathlib import Path
from typing import Annotated, Literal

from fastapi import Depends, FastAPI, Header, HTTPException, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel, Field

from engine import AgentService, EventHub, HostProfiler, MemoryStore, ModelRegistry

ROOT = Path(os.environ.get("USB_AGENT_HOME", Path(__file__).resolve().parents[1])).resolve()
FRONTEND_DIR = ROOT / "frontend"

store = MemoryStore()
registry = ModelRegistry()
events = EventHub()
agent = AgentService(store, registry, events)

app = FastAPI(title="USB Agent Gateway", version="0.1.0")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://127.0.0.1", "http://localhost"],
    allow_credentials=False,
    allow_methods=["GET", "POST"],
    allow_headers=["Content-Type"],
)
app.mount("/assets", StaticFiles(directory=FRONTEND_DIR), name="assets")

AUTH_REQUIRED = os.environ.get("USB_AGENT_REQUIRE_TOKEN", "0") == "1"
ACCESS_TOKEN = os.environ.get("USB_AGENT_ACCESS_TOKEN", "")
PAIRING_CODE = os.environ.get("USB_AGENT_PAIRING_CODE", "")


def require_token(authorization: str | None = Header(default=None)) -> None:
    if not AUTH_REQUIRED:
        return
    expected = f"Bearer {ACCESS_TOKEN}"
    if not ACCESS_TOKEN or authorization != expected:
        raise HTTPException(status_code=401, detail="A valid bearer token is required for this gateway.")


class TaskRequest(BaseModel):
    prompt: str = Field(min_length=1, max_length=20_000)
    requested_model: str | None = Field(default=None, max_length=100)


class ApprovalRequest(BaseModel):
    kind: Literal["elevated_windows_action", "android_action"]
    title: str = Field(min_length=3, max_length=160)
    reason: str = Field(min_length=3, max_length=500)
    details: dict[str, str] = Field(default_factory=dict)


class ApprovalDecision(BaseModel):
    accept: bool


class PairingRequest(BaseModel):
    code: str = Field(min_length=8, max_length=128)


@app.get("/", include_in_schema=False)
async def dashboard() -> FileResponse:
    return FileResponse(FRONTEND_DIR / "index.html")


@app.get("/health")
async def health() -> dict:
    return {"status": "ok", "profile": HostProfiler.sample(), "data_path": str(store.path)}


@app.post("/api/pair")
async def pair(request: PairingRequest) -> dict:
    if not PAIRING_CODE or not ACCESS_TOKEN:
        raise HTTPException(status_code=503, detail="Pairing is not configured on this gateway.")
    if request.code != PAIRING_CODE:
        raise HTTPException(status_code=401, detail="Invalid pairing code.")
    return {"access_token": ACCESS_TOKEN, "token_type": "Bearer"}


@app.get("/api/status", dependencies=[Depends(require_token)])
async def status() -> dict:
    return {
        "host": HostProfiler.sample(),
        "models": await registry.status(),
        "tasks": store.list_tasks(limit=20),
    }


@app.post("/api/tasks", status_code=202, dependencies=[Depends(require_token)])
async def create_task(request: TaskRequest) -> dict:
    return await agent.start_task(request.prompt, request.requested_model)


@app.get("/api/tasks/{task_id}", dependencies=[Depends(require_token)])
async def get_task(task_id: str) -> dict:
    task = store.get_task(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail="Task not found")
    return task


@app.post("/api/approvals", status_code=201, dependencies=[Depends(require_token)])
async def create_approval(request: ApprovalRequest) -> dict:
    approval = store.create_approval(request.model_dump())
    await events.broadcast({"type": "approval", **approval})
    return approval


@app.post("/api/approvals/{approval_id}", dependencies=[Depends(require_token)])
async def decide_approval(approval_id: str, decision: ApprovalDecision) -> dict:
    approval = store.decide_approval(approval_id, decision.accept)
    if approval is None:
        raise HTTPException(status_code=404, detail="Approval request not found")
    await events.broadcast({"type": "approval", **approval})
    return approval


@app.websocket("/ws/events")
async def event_stream(websocket: WebSocket) -> None:
    await events.connect(websocket)
    try:
        await websocket.send_json({"type": "connected", "host": HostProfiler.sample()})
        while True:
            # Keep the socket responsive while all task updates are server-pushed.
            await websocket.receive_text()
    except WebSocketDisconnect:
        events.disconnect(websocket)
    except Exception:
        events.disconnect(websocket)
