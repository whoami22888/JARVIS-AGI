from __future__ import annotations

import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
INCLUDE = ("backend", "frontend", "config", "scripts", "windows", "skills")
EXCLUDE_PARTS = {"__pycache__", ".git", "data", "dist", "build"}


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> None:
    files: dict[str, str] = {}
    for name in INCLUDE:
        directory = ROOT / name
        if not directory.exists():
            continue
        for candidate in sorted(directory.rglob("*")):
            if not candidate.is_file() or any(part in EXCLUDE_PARTS for part in candidate.parts):
                continue
            files[str(candidate.relative_to(ROOT))] = digest(candidate)
    (ROOT / "release-manifest.json").write_text(json.dumps({"version": 1, "files": files}, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(f"Wrote release manifest for {len(files)} files.")


if __name__ == "__main__":
    main()
