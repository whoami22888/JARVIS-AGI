from __future__ import annotations

import asyncio
import json
import os
import platform
import sqlite3
import time
import uuid
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Literal

import httpx
import psutil

ROOT = Path(os.environ.get("USB_AGENT_HOME", Path(__file__).resolve().parents[1])).resolve()
DATA_DIR = Path(os.environ.get("USB_AGENT_DATA", ROOT / "data")).resolve()
CONFIG_DIR = Path(os.environ.get("USB_AGENT_CONFIG", ROOT / "config")).resolve()
DATA_DIR.mkdir(parents=True, exist_ok=True)


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


@dataclass(frozen=True)
class HostProfile:
    name: Literal["ULTRA_LOW", "LOW", "MEDIUM", "HIGH", "WORKSTATION", "SERVER"]
    os_name: str
    machine: str
    cpu_cores: int
    memory_total_mb: int
    memory_available_mb: int
    disk_free_mb: int
    context_tokens: int
    max_parallel_tasks: int
    max_tool_calls: int


class HostProfiler:
    """Collects non-sensitive local capacity data and derives a conservative profile."""

    @staticmethod
    def profile() -> HostProfile:
        memory = psutil.virtual_memory()
        disk = psutil.disk_usage(DATA_DIR)
        cores = psutil.cpu_count(logical=True) or 1
        total_mb = memory.total // 1_000_000
        available_mb = memory.available // 1_000_000
        free_mb = disk.free // 1_000_000

        if total_mb < 4_000:
            name, context, parallel, tools = "ULTRA_LOW", 2_048, 1, 1
        elif total_mb < 8_000:
            name, context, parallel, tools = "LOW", 4_096, 1, 1
        elif total_mb < 16_000:
            name, context, parallel, tools = "MEDIUM", 8_192, 1, 2
        elif total_mb < 32_000:
            name, context, parallel, tools = "HIGH", 16_384, 2, 3
        elif total_mb < 64_000:
            name, context, parallel, tools = "WORKSTATION", 32_768, 3, 4
        else:
            name, context, parallel, tools = "SERVER", 65_536, 4, 6

        return HostProfile(
            name=name,
            os_name=platform.system(),
            machine=platform.machine(),
            cpu_cores=cores,
            memory_total_mb=total_mb,
            memory_available_mb=available_mb,
            disk_free_mb=free_mb,
            context_tokens=context,
            max_parallel_tasks=parallel,
            max_tool_calls=tools,
        )

    @staticmethod
    def sample() -> dict[str, Any]:
        vm = psutil.virtual_memory()
        disk = psutil.disk_usage(DATA_DIR)
        return {
            "timestamp": now_iso(),
            "cpu_percent": psutil.cpu_percent(interval=None),
            "memory_percent": vm.percent,
            "memory_available_mb": vm.available // 1_000_000,
            "disk_percent": disk.percent,
            "disk_free_mb": disk.free // 1_000_000,
            "profile": asdict(HostProfiler.profile()),
        }


class MemoryStore:
    """Portable SQLite task and performance store. Every operation opens a short-lived connection."""

    def __init__(self, path: Path | None = None) -> None:
        self.path = path or DATA_DIR / "agent_state.sqlite3"
        self._initialize()

    def _connection(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.path)
        connection.row_factory = sqlite3.Row
        connection.execute("PRAGMA journal_mode=WAL")
        connection.execute("PRAGMA foreign_keys=ON")
        return connection

    def _initialize(self) -> None:
        with self._connection() as conn:
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS tasks (
                    id TEXT PRIMARY KEY,
                    prompt TEXT NOT NULL,
                    state TEXT NOT NULL,
                    route_json TEXT NOT NULL,
                    answer TEXT,
                    error TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                );
                CREATE TABLE IF NOT EXISTS events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    task_id TEXT NOT NULL,
                    kind TEXT NOT NULL,
                    payload_json TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    FOREIGN KEY(task_id) REFERENCES tasks(id)
                );
                CREATE TABLE IF NOT EXISTS metrics (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    kind TEXT NOT NULL,
                    payload_json TEXT NOT NULL,
                    created_at TEXT NOT NULL
                );
                CREATE TABLE IF NOT EXISTS approvals (
                    id TEXT PRIMARY KEY,
                    action_json TEXT NOT NULL,
                    state TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                );
                """
            )

    def create_task(self, task: dict[str, Any]) -> None:
        with self._connection() as conn:
            conn.execute(
                "INSERT INTO tasks(id,prompt,state,route_json,created_at,updated_at) VALUES(?,?,?,?,?,?)",
                (task["id"], task["prompt"], task["state"], json.dumps(task["route"]), task["created_at"], task["created_at"]),
            )

    def update_task(self, task_id: str, state: str, answer: str | None = None, error: str | None = None) -> None:
        with self._connection() as conn:
            conn.execute(
                "UPDATE tasks SET state=?,answer=COALESCE(?,answer),error=COALESCE(?,error),updated_at=? WHERE id=?",
                (state, answer, error, now_iso(), task_id),
            )

    def append_event(self, task_id: str, kind: str, payload: dict[str, Any]) -> None:
        with self._connection() as conn:
            conn.execute(
                "INSERT INTO events(task_id,kind,payload_json,created_at) VALUES(?,?,?,?)",
                (task_id, kind, json.dumps(payload), now_iso()),
            )

    def record_metric(self, kind: str, payload: dict[str, Any]) -> None:
        with self._connection() as conn:
            conn.execute(
                "INSERT INTO metrics(kind,payload_json,created_at) VALUES(?,?,?)",
                (kind, json.dumps(payload), now_iso()),
            )

    def get_task(self, task_id: str) -> dict[str, Any] | None:
        with self._connection() as conn:
            row = conn.execute("SELECT * FROM tasks WHERE id=?", (task_id,)).fetchone()
            if row is None:
                return None
            events = conn.execute(
                "SELECT kind,payload_json,created_at FROM events WHERE task_id=? ORDER BY id ASC", (task_id,)
            ).fetchall()
        task = dict(row)
        task["route"] = json.loads(task.pop("route_json"))
        task["events"] = [
            {"kind": item["kind"], "payload": json.loads(item["payload_json"]), "created_at": item["created_at"]}
            for item in events
        ]
        return task

    def list_tasks(self, limit: int = 30) -> list[dict[str, Any]]:
        with self._connection() as conn:
            rows = conn.execute("SELECT * FROM tasks ORDER BY created_at DESC LIMIT ?", (limit,)).fetchall()
        result = []
        for row in rows:
            item = dict(row)
            item["route"] = json.loads(item.pop("route_json"))
            result.append(item)
        return result

    def create_approval(self, action: dict[str, Any]) -> dict[str, Any]:
        approval = {"id": str(uuid.uuid4()), "action": action, "state": "PROPOSED", "created_at": now_iso()}
        with self._connection() as conn:
            conn.execute(
                "INSERT INTO approvals(id,action_json,state,created_at,updated_at) VALUES(?,?,?,?,?)",
                (approval["id"], json.dumps(action), approval["state"], approval["created_at"], approval["created_at"]),
            )
        return approval

    def decide_approval(self, approval_id: str, accepted: bool) -> dict[str, Any] | None:
        state = "APPROVED" if accepted else "REJECTED"
        with self._connection() as conn:
            row = conn.execute("SELECT * FROM approvals WHERE id=?", (approval_id,)).fetchone()
            if row is None:
                return None
            conn.execute("UPDATE approvals SET state=?,updated_at=? WHERE id=?", (state, now_iso(), approval_id))
        return {"id": approval_id, "action": json.loads(row["action_json"]), "state": state}


@dataclass(frozen=True)
class ModelEntry:
    id: str
    provider: Literal["ollama", "deepseek"]
    tag: str
    role: str
    minimum_profile: str
    enabled: bool


class ModelRegistry:
    def __init__(self, path: Path | None = None) -> None:
        self.path = path or CONFIG_DIR / "models.json"

    def entries(self) -> list[ModelEntry]:
        if not self.path.exists():
            return []
        payload = json.loads(self.path.read_text(encoding="utf-8"))
        return [ModelEntry(**entry) for entry in payload.get("models", []) if entry.get("enabled", True)]

    async def local_tags(self) -> set[str]:
        base = os.environ.get("OLLAMA_BASE_URL", "http://127.0.0.1:11434").rstrip("/")
        try:
            async with httpx.AsyncClient(timeout=2.0) as client:
                response = await client.get(f"{base}/api/tags")
                response.raise_for_status()
            return {model.get("name", "") for model in response.json().get("models", [])}
        except (httpx.HTTPError, ValueError):
            return set()

    async def status(self) -> list[dict[str, Any]]:
        installed = await self.local_tags()
        output = []
        for entry in self.entries():
            output.append(
                {
                    **asdict(entry),
                    "installed": entry.provider != "ollama" or entry.tag in installed,
                }
            )
        return output


@dataclass(frozen=True)
class RouteDecision:
    provider: Literal["native", "ollama", "deepseek"]
    model_id: str | None
    model_tag: str | None
    reason: str
    context_tokens: int


class RequestRouter:
    def __init__(self, registry: ModelRegistry) -> None:
        self.registry = registry

    async def select(self, prompt: str, requested_model: str | None = None) -> RouteDecision:
        profile = HostProfiler.profile()
        normalized = prompt.lower().strip()
        if normalized in {"status", "/status", "health", "/health"} or normalized.startswith("system status"):
            return RouteDecision("native", None, None, "deterministic host status request", profile.context_tokens)

        entries = self.registry.entries()
        if requested_model:
            entry = next((item for item in entries if item.id == requested_model), None)
            if entry is None:
                raise ValueError(f"Unknown model id: {requested_model}")
            return RouteDecision(entry.provider, entry.id, entry.tag, "user-selected route", profile.context_tokens)

        keywords = {
            "coding": ("code", "python", "javascript", "typescript", "kotlin", "java", "bug", "compile", "refactor"),
            "specialist": ("network", "packet", "log", "vulnerability", "security", "forensic", "reverse engineer"),
            "planner": ("plan", "reason", "compare", "architecture", "analyse", "analyze", "strategy"),
        }
        desired_role = "general"
        if any(word in normalized for word in keywords["coding"]):
            desired_role = "coder"
        elif any(word in normalized for word in keywords["specialist"]):
            desired_role = "specialist"
        elif any(word in normalized for word in keywords["planner"]):
            desired_role = "planner"

        installed = await self.registry.local_tags()
        compatible = [entry for entry in entries if entry.role == desired_role]
        local = [entry for entry in compatible if entry.provider == "ollama" and entry.tag in installed]
        if local:
            entry = local[0]
            return RouteDecision("ollama", entry.id, entry.tag, f"local {desired_role} route", profile.context_tokens)

        fallback_local = [entry for entry in entries if entry.provider == "ollama" and entry.tag in installed]
        if fallback_local:
            entry = fallback_local[0]
            return RouteDecision("ollama", entry.id, entry.tag, "installed local fallback", profile.context_tokens)

        cloud_allowed = os.environ.get("USB_AGENT_OFFLINE_ONLY", "0") != "1"
        cloud = [entry for entry in compatible if cloud_allowed and entry.provider == "deepseek" and os.environ.get("DEEPSEEK_API_KEY")]
        if cloud:
            entry = cloud[0]
            return RouteDecision("deepseek", entry.id, entry.tag, f"configured cloud {desired_role} route", profile.context_tokens)

        return RouteDecision("native", None, None, "no configured model provider is currently available", profile.context_tokens)


class EventHub:
    def __init__(self) -> None:
        self._connections: set[Any] = set()

    async def connect(self, websocket: Any) -> None:
        await websocket.accept()
        self._connections.add(websocket)

    def disconnect(self, websocket: Any) -> None:
        self._connections.discard(websocket)

    async def broadcast(self, event: dict[str, Any]) -> None:
        disconnected = []
        for socket in list(self._connections):
            try:
                await socket.send_json(event)
            except Exception:
                disconnected.append(socket)
        for socket in disconnected:
            self.disconnect(socket)


class AgentService:
    def __init__(self, store: MemoryStore, registry: ModelRegistry, events: EventHub) -> None:
        self.store = store
        self.registry = registry
        self.router = RequestRouter(registry)
        self.events = events
        self.semaphore = asyncio.Semaphore(HostProfiler.profile().max_parallel_tasks)

    async def start_task(self, prompt: str, requested_model: str | None = None) -> dict[str, Any]:
        route = await self.router.select(prompt, requested_model)
        task = {
            "id": str(uuid.uuid4()),
            "prompt": prompt.strip(),
            "state": "QUEUED",
            "route": asdict(route),
            "created_at": now_iso(),
        }
        self.store.create_task(task)
        self.store.append_event(task["id"], "queued", {"route": task["route"]})
        await self.events.broadcast({"type": "task", "task_id": task["id"], "state": "QUEUED", "route": task["route"]})
        asyncio.create_task(self._execute(task))
        return task

    async def _emit(self, task_id: str, state: str, detail: dict[str, Any]) -> None:
        self.store.append_event(task_id, state.lower(), detail)
        await self.events.broadcast({"type": "task", "task_id": task_id, "state": state, **detail})

    async def _execute(self, task: dict[str, Any]) -> None:
        started = time.perf_counter()
        async with self.semaphore:
            self.store.update_task(task["id"], "RUNNING")
            await self._emit(task["id"], "RUNNING", {"route": task["route"]})
            try:
                route = RouteDecision(**task["route"])
                if route.provider == "native":
                    answer = json.dumps({"host": HostProfiler.sample(), "message": route.reason}, indent=2)
                elif route.provider == "ollama":
                    answer = await self._ollama_chat(task["prompt"], route)
                else:
                    answer = await self._deepseek_chat(task["prompt"], route)
                duration_ms = round((time.perf_counter() - started) * 1000, 2)
                self.store.update_task(task["id"], "COMPLETED", answer=answer)
                self.store.record_metric("task", {"task_id": task["id"], "duration_ms": duration_ms, "route": task["route"]})
                await self._emit(task["id"], "COMPLETED", {"answer": answer, "duration_ms": duration_ms})
            except Exception as exc:
                duration_ms = round((time.perf_counter() - started) * 1000, 2)
                message = str(exc)
                self.store.update_task(task["id"], "FAILED", error=message)
                self.store.record_metric("task_failed", {"task_id": task["id"], "duration_ms": duration_ms, "error": message})
                await self._emit(task["id"], "FAILED", {"error": message, "duration_ms": duration_ms})

    async def _ollama_chat(self, prompt: str, route: RouteDecision) -> str:
        base = os.environ.get("OLLAMA_BASE_URL", "http://127.0.0.1:11434").rstrip("/")
        payload = {
            "model": route.model_tag,
            "stream": False,
            "messages": [
                {"role": "system", "content": "You are the local portable assistant. Be concise, accurate, and explain tool limits."},
                {"role": "user", "content": prompt},
            ],
            "options": {"num_ctx": route.context_tokens},
        }
        async with httpx.AsyncClient(timeout=300.0) as client:
            response = await client.post(f"{base}/api/chat", json=payload)
            response.raise_for_status()
        answer = response.json().get("message", {}).get("content", "")
        if not answer:
            raise RuntimeError("The local model returned no message content.")
        return answer

    async def _deepseek_chat(self, prompt: str, route: RouteDecision) -> str:
        key = os.environ.get("DEEPSEEK_API_KEY")
        if not key:
            raise RuntimeError("DEEPSEEK_API_KEY is not configured; select an installed local model or add the key locally.")
        payload = {
            "model": route.model_tag,
            "stream": False,
            "messages": [
                {"role": "system", "content": "You are the portable unified agent. Produce direct, careful answers."},
                {"role": "user", "content": prompt},
            ],
            "thinking": {"type": "disabled"},
        }
        async with httpx.AsyncClient(timeout=120.0) as client:
            response = await client.post(
                "https://api.deepseek.com/chat/completions",
                headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
                json=payload,
            )
            response.raise_for_status()
        answer = response.json().get("choices", [{}])[0].get("message", {}).get("content", "")
        if not answer:
            raise RuntimeError("The configured provider returned no message content.")
        return answer
