from __future__ import annotations

import asyncio
import json
import os
import tempfile
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
os.environ["USB_AGENT_HOME"] = str(ROOT)
os.environ["USB_AGENT_CONFIG"] = str(ROOT / "config")

from backend.engine import AgentService, EventHub, HostProfiler, MemoryStore, ModelRegistry, RequestRouter


async def main() -> None:
    with tempfile.TemporaryDirectory() as temp_dir:
        os.environ["USB_AGENT_DATA"] = temp_dir
        store = MemoryStore(Path(temp_dir) / "test.sqlite3")
        registry = ModelRegistry(ROOT / "config" / "models.json")
        router = RequestRouter(registry)

        profile = HostProfiler.profile()
        assert profile.memory_total_mb > 0
        assert profile.context_tokens >= 2048

        route = await router.select("system status")
        assert route.provider == "native"

        events = EventHub()
        service = AgentService(store, registry, events)
        task = await service.start_task("system status")
        for _ in range(20):
            await asyncio.sleep(0.05)
            current = store.get_task(task["id"])
            if current and current["state"] in {"COMPLETED", "FAILED"}:
                break
        current = store.get_task(task["id"])
        assert current is not None
        assert current["state"] == "COMPLETED", current
        assert "host" in json.loads(current["answer"])

        approval = store.create_approval({"kind": "test", "title": "test"})
        resolved = store.decide_approval(approval["id"], True)
        assert resolved and resolved["state"] == "APPROVED"

    print("smoke_test: PASS")


if __name__ == "__main__":
    asyncio.run(main())
