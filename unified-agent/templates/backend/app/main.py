import secrets

from fastapi import Depends, FastAPI, Header, HTTPException

from .config import settings
from .db import Store
from .discovery import GatewayAdvertiser
from .models import AutomationProposal, PairRequest, PairResponse, RegistryCreate, TaskRequest
from .policy import PolicyError, validate_registry_definition
from .provider import ProviderError, complete
from .security import issue_token, pairing_matches, verify_token

cfg = settings()
store = Store(cfg.data_dir)
advertiser = GatewayAdvertiser(cfg)
app = FastAPI(title="WhiteRabbit Neo Unified Agent Gateway", version="0.2.0")


@app.on_event("startup")
def start_gateway_advertisement() -> None:
    advertiser.start()


@app.on_event("shutdown")
def stop_gateway_advertisement() -> None:
    advertiser.stop()


def authenticated_device(authorization: str | None = Header(default=None)) -> str:
    if not authorization or not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="missing device token")
    try:
        return verify_token(authorization.removeprefix("Bearer "), cfg.token_secret)
    except ValueError as exc:
        raise HTTPException(status_code=401, detail=str(exc)) from exc


@app.get("/health")
def health():
    return {
        "status": "ok",
        "local_ollama_enabled": cfg.local_ollama_enabled,
        "local_ollama_model": cfg.local_ollama_model,
        "authorized_cloud_fallback_enabled": cfg.cloud_fallback_enabled and bool(cfg.deepseek_api_key),
        "cloud_model": cfg.deepseek_model,
        "local_discovery_enabled": cfg.gateway_discovery_enabled,
    }


@app.post("/v1/pair", response_model=PairResponse)
def pair(request: PairRequest):
    if not pairing_matches(request.pairing_code, cfg.pairing_code):
        raise HTTPException(status_code=401, detail="invalid pairing code")
    device_id = secrets.token_urlsafe(16)
    store.upsert_device(device_id, request.device_name)
    return PairResponse(device_id=device_id, token=issue_token(device_id, cfg.token_secret), expires_in_seconds=604800)


@app.post("/v1/tasks")
async def create_task(request: TaskRequest, device_id: str = Depends(authenticated_device)):
    task_id = store.create_task(device_id, request.prompt, request.model or cfg.deepseek_model)
    resolved_route: str | None = None
    store.set_task(task_id, "running")
    store.event(task_id, "routing.selected", {"requested_model": request.model or cfg.deepseek_model, "capabilities": request.enabled_capabilities})
    try:
        completion = await complete(cfg, request.prompt, request.model or cfg.deepseek_model)
        resolved_route = completion.route
        store.set_task(task_id, "completed", completion.content)
        store.event(task_id, "routing.resolved", {"route": completion.route, "model": completion.model})
        store.event(task_id, "task.completed", {"answer_length": len(completion.content)})
    except ProviderError as exc:
        answer = str(exc)
        store.set_task(task_id, "blocked", answer)
        store.event(task_id, "task.blocked", {"reason": answer})
    task = store.get_task(task_id, device_id)
    if task:
        task["route"] = resolved_route
    return task


@app.get("/v1/tasks/{task_id}")
def get_task(task_id: str, device_id: str = Depends(authenticated_device)):
    task = store.get_task(task_id, device_id)
    if not task:
        raise HTTPException(status_code=404, detail="task not found")
    task["events"] = store.events(task_id)
    return task


@app.get("/v1/registry")
def list_registry(kind: str | None = None, device_id: str = Depends(authenticated_device)):
    return store.registry(device_id, kind)


@app.post("/v1/registry")
def add_registry_item(request: RegistryCreate, device_id: str = Depends(authenticated_device)):
    try:
        definition = validate_registry_definition(request.kind, request.definition)
    except PolicyError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return store.add_registry_item(device_id, request.kind, request.name, definition)


@app.post("/v1/automation/proposals")
def propose_automation(request: AutomationProposal, device_id: str = Depends(authenticated_device)):
    return {"status": "proposed", "requires_local_approval": True, "proposal": request.model_dump(), "device_id": device_id}
