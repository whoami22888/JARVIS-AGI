from __future__ import annotations

import json
import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
os.environ.setdefault("USB_AGENT_HOME", str(ROOT))
os.environ.setdefault("USB_AGENT_DATA", str(ROOT / "data"))
os.environ.setdefault("USB_AGENT_CONFIG", str(ROOT / "config"))

from backend.engine import HostProfiler, MemoryStore


def main() -> None:
    sample = HostProfiler.sample()
    store = MemoryStore()
    store.record_metric("host_profile", sample)
    output = ROOT / "data" / "host-profile.json"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(sample, indent=2), encoding="utf-8")
    print(json.dumps(sample, indent=2))


if __name__ == "__main__":
    main()
