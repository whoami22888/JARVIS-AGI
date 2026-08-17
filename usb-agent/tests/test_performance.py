from __future__ import annotations

import tempfile
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "backend"))
from performance import BenchmarkStore, IndexQueue, StorageInspector, working_set_policy


def run() -> None:
    low = working_set_policy(6_000, "LOW")
    assert low.max_active_models == 1
    assert low.zram_allowed and not low.usb_swap_allowed

    with tempfile.TemporaryDirectory() as temp:
        root = Path(temp)
        source = root / "workspace"
        source.mkdir()
        (source / "keep.txt").write_text("incremental agent knowledge", encoding="utf-8")
        ignored = source / "node_modules"
        ignored.mkdir()
        (ignored / "skip.txt").write_text("skip", encoding="utf-8")

        queue = IndexQueue(root / "queue.sqlite3")
        stats = queue.enqueue_tree(source)
        assert stats["queued"] == 1
        batch = queue.next_batch()
        assert len(batch) == 1 and batch[0]["path"].endswith("keep.txt")
        queue.complete(batch[0]["path"], content_hash="hash")
        assert queue.summary()["INDEXED"] == 1

        storage = StorageInspector(root)
        snapshot = storage.snapshot()
        assert snapshot["free_bytes"] > 0 and "cleanup_recommendation" in snapshot

        benchmarks = BenchmarkStore(root / "benchmarks.sqlite3")
        benchmarks.record("test", {"first_token_ms": 12})
        assert benchmarks.recent(1)[0]["payload"]["first_token_ms"] == 12


if __name__ == "__main__":
    run()
    print("test_performance: PASS")
