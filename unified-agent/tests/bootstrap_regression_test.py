"""Regression coverage for non-destructive generated-project bootstrap scripts."""
from __future__ import annotations

import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


HERE = Path(__file__).resolve().parents[1]


class BootstrapRegressionTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.root = Path(self.temp_dir.name) / "unified-agent"
        self.root.mkdir()
        for kind in ("android", "backend"):
            shutil.copy2(HERE / f"bootstrap_{kind}.sh", self.root / f"bootstrap_{kind}.sh")
            (self.root / "templates" / kind / "nested").mkdir(parents=True)
            (self.root / "templates" / kind / "nested" / "source.txt").write_text("current")

    def run_bootstrap(self, kind: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["bash", str(self.root / f"bootstrap_{kind}.sh")],
            cwd=self.root,
            capture_output=True,
            text=True,
            check=False,
        )

    def test_first_bootstrap_copies_files(self) -> None:
        for kind in ("android", "backend"):
            with self.subTest(kind=kind):
                completed = self.run_bootstrap(kind)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertEqual((self.root / kind / "nested" / "source.txt").read_text(), "current")

    def test_bootstrap_preserves_existing_local_state(self) -> None:
        for kind, local_file in (("android", "local.properties"), ("backend", ".env")):
            with self.subTest(kind=kind):
                dest = self.root / kind
                (dest / "nested").mkdir(parents=True)
                (dest / local_file).write_text("local private state")
                (dest / "nested" / "source.txt").write_text("current")
                (dest / "build" / "outputs").mkdir(parents=True)
                (dest / "build" / "outputs" / "artifact.apk").write_text("keep")
                completed = self.run_bootstrap(kind)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertEqual((dest / local_file).read_text(), "local private state")
                self.assertEqual((dest / "build" / "outputs" / "artifact.apk").read_text(), "keep")

    def test_conflicts_fail_before_writing(self) -> None:
        for kind in ("android", "backend"):
            with self.subTest(kind=kind):
                dest = self.root / kind
                (dest / "nested").mkdir(parents=True)
                (dest / "nested" / "source.txt").write_text("newer generated code")
                (self.root / "templates" / kind / "would_copy.txt").write_text("not copied")
                completed = self.run_bootstrap(kind)
                self.assertNotEqual(completed.returncode, 0)
                self.assertIn("Refusing to overwrite", completed.stderr)
                self.assertEqual((dest / "nested" / "source.txt").read_text(), "newer generated code")
                self.assertFalse((dest / "would_copy.txt").exists())


if __name__ == "__main__":
    unittest.main()
