from __future__ import annotations

import hashlib
import json
import os
import sqlite3
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import psutil

from features import FeatureError, data_path, now_iso


@dataclass(frozen=True)
class WorkingSetPolicy:
    profile: str
    hot_memory_budget_mb: int
    zram_allowed: bool
    zram_max_mb: int
    max_background_workers: int
    max_active_models: int
    usb_swap_allowed: bool = False


def working_set_policy(memory_total_mb: int, profile: str) -> WorkingSetPolicy:
    if memory_total_mb < 4_000:
        return WorkingSetPolicy(profile, 1_500, False, 0, 0, 1)
    if memory_total_mb < 8_000:
        return WorkingSetPolicy(profile, 3_000, True, 1_500, 1, 1)
    if memory_total_mb < 16_000:
        return WorkingSetPolicy(profile, 6_000, True, 3_000, 1, 1)
    if memory_total_mb < 32_000:
        return WorkingSetPolicy(profile, 12_000, True, 6_000, 2, 1)
    return WorkingSetPolicy(profile, max(16_000, memory_total_mb // 2), True, min(memory_total_mb // 3, 24_000), 3, 1)


class StorageInspector:
    def __init__(self, root: Path | None = None) -> None:
        self.root = root or data_path()

    @staticmethod
    def _tree_size(path: Path) -> int:
        if not path.exists():
            return 0
        total = 0
        for item in path.rglob("*"):
            try:
                if item.is_file():
                    total += item.stat().st_size
            except OSError:
                continue
        return total

    def snapshot(self) -> dict[str, Any]:
        disk = psutil.disk_usage(self.root)
        root = self.root
        sizes = {
            "models_bytes": self._tree_size(root.parent / "models"),
            "index_bytes": self._tree_size(root / "knowledge.sqlite3"),
            "state_bytes": self._tree_size(root),
            "logs_bytes": self._tree_size(root / "logs"),
        }
        return {
            "data_path": str(root),
            "free_bytes": disk.free,
            "total_bytes": disk.total,
            "used_percent": disk.percent,
            **sizes,
            "cleanup_recommendation": "Clear staged downloads and old diagnostics first." if disk.percent >= 85 else "Storage headroom is acceptable.",
        }


class IndexQueue:
    """A resumable, explicit file-index queue; callers choose files and scheduling."""

    EXCLUDED_PARTS = {"node_modules", ".git", ".venv", "venv", "build", "dist", "__pycache__"}
    MAX_BYTES = 8 * 1024 * 1024

    def __init__(self, path: Path | None = None) -> None:
        self.path = path or data_path() / "index_queue.sqlite3"
        with self._connection() as conn:
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS queue (
                    path TEXT PRIMARY KEY,
                    root TEXT NOT NULL,
                    status TEXT NOT NULL,
                    content_hash TEXT,
                    error TEXT,
                    updated_at TEXT NOT NULL
                );
                """
            )

    def _connection(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.path)
        connection.row_factory = sqlite3.Row
        connection.execute("PRAGMA journal_mode=WAL")
        return connection

    @classmethod
    def eligible(cls, path: Path) -> bool:
        return path.is_file() and path.stat().st_size <= cls.MAX_BYTES and not any(part in cls.EXCLUDED_PARTS for part in path.parts)

    def enqueue_tree(self, root: Path, max_files: int = 500) -> dict[str, int]:
        root = root.resolve()
        if not root.is_dir():
            raise FeatureError("Index queue root must be a directory.")
        queued = 0
        skipped = 0
        with self._connection() as conn:
            for candidate in root.rglob("*"):
                if queued >= max(1, min(max_files, 5_000)):
                    break
                try:
                    if not self.eligible(candidate):
                        skipped += 1
                        continue
                    conn.execute(
                        "INSERT INTO queue(path,root,status,updated_at) VALUES(?,?, 'PENDING', ?) ON CONFLICT(path) DO UPDATE SET status=CASE WHEN queue.status='INDEXED' THEN queue.status ELSE 'PENDING' END,updated_at=excluded.updated_at",
                        (str(candidate.resolve()), str(root), now_iso()),
                    )
                    queued += 1
                except OSError:
                    skipped += 1
        return {"queued": queued, "skipped": skipped}

    def next_batch(self, max_items: int = 5) -> list[dict[str, Any]]:
        vm = psutil.virtual_memory()
        if psutil.cpu_percent(interval=None) > 75 or vm.percent > 80:
            return []
        with self._connection() as conn:
            rows = conn.execute("SELECT * FROM queue WHERE status='PENDING' ORDER BY updated_at ASC LIMIT ?", (max(1, min(max_items, 20)),)).fetchall()
            for row in rows:
                conn.execute("UPDATE queue SET status='RUNNING',updated_at=? WHERE path=?", (now_iso(), row["path"]))
        return [dict(row) for row in rows]

    def complete(self, path: str, content_hash: str | None = None, error: str | None = None) -> None:
        with self._connection() as conn:
            conn.execute(
                "UPDATE queue SET status=?,content_hash=?,error=?,updated_at=? WHERE path=?",
                ("FAILED" if error else "INDEXED", content_hash, error, now_iso(), path),
            )

    def summary(self) -> dict[str, int]:
        with self._connection() as conn:
            rows = conn.execute("SELECT status,COUNT(*) AS count FROM queue GROUP BY status").fetchall()
        output = {"PENDING": 0, "RUNNING": 0, "INDEXED": 0, "FAILED": 0}
        output.update({row["status"]: row["count"] for row in rows})
        return output


class BenchmarkStore:
    def __init__(self, path: Path | None = None) -> None:
        self.path = path or data_path() / "benchmarks.sqlite3"
        with self._connection() as conn:
            conn.execute(
                "CREATE TABLE IF NOT EXISTS records(id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, payload_json TEXT NOT NULL, created_at TEXT NOT NULL)"
            )

    def _connection(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.path)
        connection.row_factory = sqlite3.Row
        connection.execute("PRAGMA journal_mode=WAL")
        return connection

    def record(self, kind: str, payload: dict[str, Any]) -> None:
        with self._connection() as conn:
            conn.execute("INSERT INTO records(kind,payload_json,created_at) VALUES(?,?,?)", (kind, json.dumps(payload), now_iso()))

    def recent(self, limit: int = 20) -> list[dict[str, Any]]:
        with self._connection() as conn:
            rows = conn.execute("SELECT kind,payload_json,created_at FROM records ORDER BY id DESC LIMIT ?", (max(1, min(limit, 100)),)).fetchall()
        return [{"kind": row["kind"], "payload": json.loads(row["payload_json"]), "created_at": row["created_at"]} for row in rows]
