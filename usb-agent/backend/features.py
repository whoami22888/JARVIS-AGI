from __future__ import annotations

import hashlib
import json
import os
import secrets
import sqlite3
from dataclasses import asdict, dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any


def now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def root_path() -> Path:
    return Path(os.environ.get("USB_AGENT_HOME", Path(__file__).resolve().parents[1])).resolve()


def data_path() -> Path:
    target = Path(os.environ.get("USB_AGENT_DATA", root_path() / "data")).resolve()
    target.mkdir(parents=True, exist_ok=True)
    return target


class FeatureError(RuntimeError):
    """Raised for a safe, owner-visible feature error."""


def _connection(path: Path) -> sqlite3.Connection:
    connection = sqlite3.connect(path)
    connection.row_factory = sqlite3.Row
    connection.execute("PRAGMA journal_mode=WAL")
    connection.execute("PRAGMA foreign_keys=ON")
    return connection


@dataclass(frozen=True)
class SkillManifest:
    id: str
    name: str
    version: str
    description: str
    capabilities: tuple[str, ...]
    latency_budget_ms: int
    self_test: str | None = None


class SkillRegistry:
    """Loads owner-installed JSON manifests; never executes a manifest command automatically."""

    def __init__(self, directory: Path | None = None) -> None:
        self.directory = directory or root_path() / "skills"
        self.directory.mkdir(parents=True, exist_ok=True)

    def manifests(self) -> list[dict[str, Any]]:
        result: list[dict[str, Any]] = []
        for path in sorted(self.directory.glob("*.json")):
            try:
                raw = path.read_bytes()
                payload = json.loads(raw.decode("utf-8"))
                manifest = SkillManifest(
                    id=str(payload["id"]),
                    name=str(payload["name"]),
                    version=str(payload["version"]),
                    description=str(payload.get("description", "")),
                    capabilities=tuple(sorted(set(payload.get("capabilities", [])))),
                    latency_budget_ms=int(payload.get("latency_budget_ms", 3_000)),
                    self_test=str(payload["self_test"]) if payload.get("self_test") else None,
                )
                result.append({**asdict(manifest), "integrity": hashlib.sha256(raw).hexdigest(), "status": "READY"})
            except (UnicodeDecodeError, json.JSONDecodeError, KeyError, TypeError, ValueError) as exc:
                result.append({"id": path.stem, "status": "INVALID", "error": str(exc), "path": str(path)})
        return result


class ReleaseManifest:
    """Hash manifest verification; a missing manifest is reported, never silently accepted."""

    def __init__(self, path: Path | None = None) -> None:
        self.path = path or root_path() / "release-manifest.json"

    def verify(self) -> dict[str, Any]:
        if not self.path.exists():
            return {"status": "MISSING", "checked": 0, "failed": []}
        try:
            payload = json.loads(self.path.read_text(encoding="utf-8"))
            files = payload.get("files", {})
            if not isinstance(files, dict):
                raise ValueError("files must be an object")
        except (OSError, UnicodeDecodeError, json.JSONDecodeError, ValueError) as exc:
            return {"status": "INVALID", "checked": 0, "failed": [{"manifest": str(exc)}]}
        failures: list[dict[str, str]] = []
        root = root_path()
        for relative, expected in files.items():
            candidate = (root / relative).resolve()
            if root not in candidate.parents and candidate != root:
                failures.append({"path": str(relative), "error": "path escapes project root"})
                continue
            if not candidate.is_file():
                failures.append({"path": str(relative), "error": "missing"})
                continue
            actual = hashlib.sha256(candidate.read_bytes()).hexdigest()
            if not secrets.compare_digest(actual, str(expected)):
                failures.append({"path": str(relative), "error": "hash mismatch"})
        return {"status": "VERIFIED" if not failures else "FAILED", "checked": len(files), "failed": failures}


class CapabilityStore:
    VALID = {"read_files", "write_files", "network_read", "run_tests", "elevated_diagnostics"}

    def __init__(self, path: Path | None = None) -> None:
        self.path = path or data_path() / "capabilities.sqlite3"
        with _connection(self.path) as conn:
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS grants (
                    id TEXT PRIMARY KEY,
                    capability TEXT NOT NULL,
                    target TEXT NOT NULL,
                    reason TEXT NOT NULL,
                    state TEXT NOT NULL,
                    expires_at TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                );
                CREATE INDEX IF NOT EXISTS grants_lookup ON grants(capability, target, state, expires_at);
                CREATE TABLE IF NOT EXISTS audit (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    event TEXT NOT NULL,
                    payload_json TEXT NOT NULL,
                    created_at TEXT NOT NULL
                );
                """
            )

    def grant(self, capability: str, target: str, reason: str, ttl_minutes: int = 30) -> dict[str, Any]:
        if capability not in self.VALID:
            raise FeatureError(f"Unsupported capability: {capability}")
        if not target or len(target) > 1_000:
            raise FeatureError("A valid capability target is required.")
        if not reason or len(reason) > 1_000:
            raise FeatureError("A clear capability reason is required.")
        ttl = max(1, min(int(ttl_minutes), 24 * 60))
        created = datetime.now(timezone.utc)
        item = {
            "id": secrets.token_urlsafe(18),
            "capability": capability,
            "target": target,
            "reason": reason,
            "state": "APPROVED",
            "created_at": created.isoformat(),
            "updated_at": created.isoformat(),
            "expires_at": (created + timedelta(minutes=ttl)).isoformat(),
        }
        with _connection(self.path) as conn:
            conn.execute(
                "INSERT INTO grants VALUES(?,?,?,?,?,?,?,?)",
                tuple(item[key] for key in ("id", "capability", "target", "reason", "state", "expires_at", "created_at", "updated_at")),
            )
            conn.execute("INSERT INTO audit(event,payload_json,created_at) VALUES(?,?,?)", ("capability_granted", json.dumps(item), now_iso()))
        return item

    def active(self, capability: str, target: str) -> bool:
        with _connection(self.path) as conn:
            row = conn.execute(
                "SELECT id FROM grants WHERE capability=? AND target=? AND state='APPROVED' AND expires_at>? ORDER BY expires_at DESC LIMIT 1",
                (capability, target, now_iso()),
            ).fetchone()
        return row is not None

    def revoke(self, grant_id: str) -> bool:
        with _connection(self.path) as conn:
            updated = conn.execute("UPDATE grants SET state='REVOKED',updated_at=? WHERE id=? AND state='APPROVED'", (now_iso(), grant_id)).rowcount
        return bool(updated)

    def status(self) -> list[dict[str, Any]]:
        with _connection(self.path) as conn:
            rows = conn.execute("SELECT * FROM grants WHERE state='APPROVED' AND expires_at>? ORDER BY created_at DESC", (now_iso(),)).fetchall()
        return [dict(row) for row in rows]


class KnowledgeVault:
    """Small SQLite FTS5 index for owner-approved local text; no background crawling."""

    MAX_BYTES = 8 * 1024 * 1024

    def __init__(self, path: Path | None = None) -> None:
        self.path = path or data_path() / "knowledge.sqlite3"
        with _connection(self.path) as conn:
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS documents (
                    path TEXT PRIMARY KEY,
                    content_hash TEXT NOT NULL,
                    modified_ns INTEGER NOT NULL,
                    size_bytes INTEGER NOT NULL,
                    indexed_at TEXT NOT NULL
                );
                CREATE VIRTUAL TABLE IF NOT EXISTS document_search USING fts5(path UNINDEXED, chunk UNINDEXED, content);
                """
            )

    @staticmethod
    def _safe_path(path: Path, roots: list[Path]) -> Path:
        candidate = path.resolve()
        if not any(root.resolve() in candidate.parents or candidate == root.resolve() for root in roots):
            raise FeatureError("Path is outside approved knowledge roots.")
        if not candidate.is_file():
            raise FeatureError("Knowledge item must be a regular file.")
        return candidate

    def index_file(self, path: Path, roots: list[Path]) -> dict[str, Any]:
        candidate = self._safe_path(path, roots)
        stat = candidate.stat()
        if stat.st_size > self.MAX_BYTES:
            raise FeatureError("File exceeds the configured knowledge indexing size limit.")
        try:
            text = candidate.read_text(encoding="utf-8", errors="replace")
        except OSError as exc:
            raise FeatureError(f"Could not read knowledge file: {exc}") from exc
        digest = hashlib.sha256(text.encode("utf-8")).hexdigest()
        with _connection(self.path) as conn:
            existing = conn.execute("SELECT content_hash FROM documents WHERE path=?", (str(candidate),)).fetchone()
            if existing and existing["content_hash"] == digest:
                return {"path": str(candidate), "status": "UNCHANGED", "chunks": 0, "content_hash": digest}
            conn.execute("DELETE FROM document_search WHERE path=?", (str(candidate),))
            chunks = [text[index:index + 2_000] for index in range(0, len(text), 2_000)] or [""]
            for number, chunk in enumerate(chunks):
                conn.execute("INSERT INTO document_search(path,chunk,content) VALUES(?,?,?)", (str(candidate), number, chunk))
            conn.execute(
                "INSERT INTO documents(path,content_hash,modified_ns,size_bytes,indexed_at) VALUES(?,?,?,?,?) ON CONFLICT(path) DO UPDATE SET content_hash=excluded.content_hash,modified_ns=excluded.modified_ns,size_bytes=excluded.size_bytes,indexed_at=excluded.indexed_at",
                (str(candidate), digest, stat.st_mtime_ns, stat.st_size, now_iso()),
            )
        return {"path": str(candidate), "status": "INDEXED", "chunks": len(chunks), "content_hash": digest}

    def search(self, query: str, limit: int = 8) -> list[dict[str, Any]]:
        if not query.strip():
            raise FeatureError("Knowledge search query cannot be empty.")
        with _connection(self.path) as conn:
            rows = conn.execute(
                "SELECT path,chunk,snippet(document_search, 2, '[', ']', '…', 16) AS excerpt FROM document_search WHERE document_search MATCH ? LIMIT ?",
                (query, max(1, min(int(limit), 20))),
            ).fetchall()
        return [dict(row) for row in rows]


class SessionStore:
    """Local gateway pairing sessions. Only a hash of the pairing code is stored."""

    def __init__(self, path: Path | None = None) -> None:
        self.path = path or data_path() / "sessions.sqlite3"
        with _connection(self.path) as conn:
            conn.executescript(
                """
                CREATE TABLE IF NOT EXISTS sessions (
                    id TEXT PRIMARY KEY,
                    token_hash TEXT NOT NULL UNIQUE,
                    label TEXT NOT NULL,
                    expires_at TEXT NOT NULL,
                    revoked INTEGER NOT NULL DEFAULT 0,
                    created_at TEXT NOT NULL
                );
                CREATE TABLE IF NOT EXISTS pairing_config (
                    id INTEGER PRIMARY KEY CHECK(id=1),
                    code_salt TEXT NOT NULL,
                    code_hash TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                );
                """
            )

    @staticmethod
    def _digest(value: str) -> str:
        return hashlib.sha256(value.encode("utf-8")).hexdigest()

    def configure_pairing_code(self, code: str) -> None:
        if len(code) < 8:
            raise FeatureError("Pairing code must contain at least eight characters.")
        salt = secrets.token_urlsafe(18)
        digest = self._digest(f"{salt}:{code}")
        with _connection(self.path) as conn:
            conn.execute(
                "INSERT INTO pairing_config(id,code_salt,code_hash,updated_at) VALUES(1,?,?,?) ON CONFLICT(id) DO UPDATE SET code_salt=excluded.code_salt,code_hash=excluded.code_hash,updated_at=excluded.updated_at",
                (salt, digest, now_iso()),
            )

    def ensure_configured_from_environment(self) -> bool:
        code = os.environ.get("USB_AGENT_PAIRING_CODE", "")
        if not code:
            return False
        with _connection(self.path) as conn:
            exists = conn.execute("SELECT id FROM pairing_config WHERE id=1").fetchone()
        if not exists:
            self.configure_pairing_code(code)
        return True

    def issue(self, code: str, label: str = "paired-device", ttl_minutes: int = 60) -> dict[str, Any]:
        with _connection(self.path) as conn:
            config = conn.execute("SELECT code_salt,code_hash FROM pairing_config WHERE id=1").fetchone()
        if not config:
            raise FeatureError("Pairing is not configured on this gateway.")
        actual = self._digest(f"{config['code_salt']}:{code}")
        if not secrets.compare_digest(actual, config["code_hash"]):
            raise FeatureError("Invalid pairing code.")
        token = secrets.token_urlsafe(32)
        created = datetime.now(timezone.utc)
        session = {
            "id": secrets.token_urlsafe(18),
            "access_token": token,
            "token_type": "Bearer",
            "label": label[:120] or "paired-device",
            "created_at": created.isoformat(),
            "expires_at": (created + timedelta(minutes=max(5, min(int(ttl_minutes), 24 * 60)))).isoformat(),
        }
        with _connection(self.path) as conn:
            conn.execute(
                "INSERT INTO sessions(id,token_hash,label,expires_at,revoked,created_at) VALUES(?,?,?,?,0,?)",
                (session["id"], self._digest(token), session["label"], session["expires_at"], session["created_at"]),
            )
        return session

    def verify(self, token: str) -> bool:
        if not token:
            return False
        with _connection(self.path) as conn:
            row = conn.execute(
                "SELECT id FROM sessions WHERE token_hash=? AND revoked=0 AND expires_at>?",
                (self._digest(token), now_iso()),
            ).fetchone()
        return row is not None

    def revoke(self, session_id: str) -> bool:
        with _connection(self.path) as conn:
            changed = conn.execute("UPDATE sessions SET revoked=1 WHERE id=? AND revoked=0", (session_id,)).rowcount
        return bool(changed)

    def active(self) -> list[dict[str, Any]]:
        with _connection(self.path) as conn:
            rows = conn.execute(
                "SELECT id,label,expires_at,created_at FROM sessions WHERE revoked=0 AND expires_at>? ORDER BY created_at DESC",
                (now_iso(),),
            ).fetchall()
        return [dict(row) for row in rows]
