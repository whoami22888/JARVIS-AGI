from __future__ import annotations

import json
import os
import tempfile
from pathlib import Path

import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "backend"))
from features import CapabilityStore, KnowledgeVault, ReleaseManifest, SessionStore, SkillRegistry


def run() -> None:
    with tempfile.TemporaryDirectory() as temp:
        root = Path(temp)
        data = root / "data"
        data.mkdir()

        sessions = SessionStore(data / "sessions.sqlite3")
        sessions.configure_pairing_code("correct-horse-battery-staple")
        issued = sessions.issue("correct-horse-battery-staple", "test-device", 5)
        assert sessions.verify(issued["access_token"])
        assert sessions.revoke(issued["id"])
        assert not sessions.verify(issued["access_token"])

        capabilities = CapabilityStore(data / "capabilities.sqlite3")
        grant = capabilities.grant("read_files", str(root), "Test knowledge indexing", 5)
        assert capabilities.active("read_files", str(root))
        assert capabilities.revoke(grant["id"])
        assert not capabilities.active("read_files", str(root))

        skills_dir = root / "skills"
        skills_dir.mkdir()
        (skills_dir / "status.json").write_text(json.dumps({
            "id": "status", "name": "System Status", "version": "1.0.0", "description": "Read host status", "capabilities": [], "latency_budget_ms": 500
        }), encoding="utf-8")
        manifest = SkillRegistry(skills_dir).manifests()
        assert len(manifest) == 1 and manifest[0]["status"] == "READY"

        document = root / "note.txt"
        document.write_text("Portable agent knowledge is stored locally.", encoding="utf-8")
        vault = KnowledgeVault(data / "knowledge.sqlite3")
        indexed = vault.index_file(document, [root])
        assert indexed["status"] == "INDEXED"
        assert vault.search("portable")[0]["path"] == str(document)

        old_home = os.environ.get("USB_AGENT_HOME")
        os.environ["USB_AGENT_HOME"] = str(root)
        release_file = root / "release.txt"
        release_file.write_text("verified", encoding="utf-8")
        digest = __import__("hashlib").sha256(release_file.read_bytes()).hexdigest()
        (root / "release-manifest.json").write_text(json.dumps({"files": {"release.txt": digest}}), encoding="utf-8")
        assert ReleaseManifest().verify()["status"] == "VERIFIED"
        release_file.write_text("changed", encoding="utf-8")
        assert ReleaseManifest().verify()["status"] == "FAILED"
        if old_home is None:
            os.environ.pop("USB_AGENT_HOME", None)
        else:
            os.environ["USB_AGENT_HOME"] = old_home


if __name__ == "__main__":
    run()
    print("test_features: PASS")
