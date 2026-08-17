import json
import sqlite3
import time
import uuid
from contextlib import contextmanager
from pathlib import Path

class Store:
    def __init__(self, data_dir: Path):
        self.path = data_dir / "agent.sqlite3"
        self._init()

    @contextmanager
    def connection(self):
        con = sqlite3.connect(self.path)
        con.row_factory = sqlite3.Row
        try:
            yield con
            con.commit()
        finally:
            con.close()

    def _init(self):
        with self.connection() as con:
            con.executescript('''
            CREATE TABLE IF NOT EXISTS devices (
              id TEXT PRIMARY KEY, name TEXT NOT NULL, created_at INTEGER NOT NULL
            );
            CREATE TABLE IF NOT EXISTS tasks (
              id TEXT PRIMARY KEY, device_id TEXT NOT NULL, prompt TEXT NOT NULL,
              model TEXT NOT NULL, status TEXT NOT NULL, answer TEXT, created_at INTEGER NOT NULL
            );
            CREATE TABLE IF NOT EXISTS events (
              id TEXT PRIMARY KEY, task_id TEXT NOT NULL, type TEXT NOT NULL,
              payload TEXT NOT NULL, created_at INTEGER NOT NULL
            );
            CREATE TABLE IF NOT EXISTS registry_items (
              id TEXT PRIMARY KEY, device_id TEXT NOT NULL, kind TEXT NOT NULL,
              name TEXT NOT NULL, definition TEXT NOT NULL, enabled INTEGER NOT NULL, created_at INTEGER NOT NULL
            );
            ''')

    def upsert_device(self, device_id: str, name: str):
        with self.connection() as con:
            con.execute("INSERT OR REPLACE INTO devices(id, name, created_at) VALUES (?, ?, ?)", (device_id, name, int(time.time())))

    def create_task(self, device_id: str, prompt: str, model: str) -> str:
        task_id = str(uuid.uuid4())
        with self.connection() as con:
            con.execute("INSERT INTO tasks(id, device_id, prompt, model, status, answer, created_at) VALUES (?, ?, ?, ?, 'queued', NULL, ?)", (task_id, device_id, prompt, model, int(time.time())))
        self.event(task_id, "task.accepted", {"model": model})
        return task_id

    def set_task(self, task_id: str, status: str, answer: str | None = None):
        with self.connection() as con:
            con.execute("UPDATE tasks SET status = ?, answer = COALESCE(?, answer) WHERE id = ?", (status, answer, task_id))

    def event(self, task_id: str, event_type: str, payload: dict):
        with self.connection() as con:
            con.execute("INSERT INTO events(id, task_id, type, payload, created_at) VALUES (?, ?, ?, ?, ?)", (str(uuid.uuid4()), task_id, event_type, json.dumps(payload), int(time.time())))

    def get_task(self, task_id: str, device_id: str):
        with self.connection() as con:
            row = con.execute("SELECT * FROM tasks WHERE id = ? AND device_id = ?", (task_id, device_id)).fetchone()
            return dict(row) if row else None

    def events(self, task_id: str):
        with self.connection() as con:
            rows = con.execute("SELECT type, payload, created_at FROM events WHERE task_id = ? ORDER BY created_at", (task_id,)).fetchall()
            return [{"type": row["type"], "payload": json.loads(row["payload"]), "created_at": row["created_at"]} for row in rows]

    def add_registry_item(self, device_id: str, kind: str, name: str, definition: dict) -> dict:
        item = {"id": str(uuid.uuid4()), "kind": kind, "name": name, "definition": definition, "enabled": True, "created_at": int(time.time())}
        with self.connection() as con:
            con.execute("INSERT INTO registry_items(id, device_id, kind, name, definition, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)", (item["id"], device_id, kind, name, json.dumps(definition), 1, item["created_at"]))
        return item

    def registry(self, device_id: str, kind: str | None = None):
        query = "SELECT * FROM registry_items WHERE device_id = ?"
        args: list[str] = [device_id]
        if kind:
            query += " AND kind = ?"
            args.append(kind)
        query += " ORDER BY created_at DESC"
        with self.connection() as con:
            rows = con.execute(query, args).fetchall()
            return [{"id": r["id"], "kind": r["kind"], "name": r["name"], "definition": json.loads(r["definition"]), "enabled": bool(r["enabled"]), "created_at": r["created_at"]} for r in rows]
