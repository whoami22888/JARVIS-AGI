"""No-Docker regression tests for the user-controlled Pockr deployment script."""
from __future__ import annotations

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "backend" / "run-in-pockr.sh"


class PockrRunnerTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.envfile = self.root / "gateway.env"
        self.log = self.root / "docker.log"
        docker = self.root / "docker"
        docker.write_text(
            '#!/bin/sh\n'
            'printf "%s\\n" "$*" >> "$FAKE_DOCKER_LOG"\n'
            'if [ "$1" = "container" ] && [ "$2" = "inspect" ]; then exit 1; fi\n'
            'exit 0\n'
        )
        docker.chmod(0o755)
        self.env = {**os.environ, "JARVIS_ENV_FILE": str(self.envfile),
                    "FAKE_DOCKER_LOG": str(self.log),
                    "PATH": str(self.root) + os.pathsep + os.environ["PATH"]}

    def execute(self) -> subprocess.CompletedProcess[str]:
        return subprocess.run(["bash", str(SCRIPT)], capture_output=True, text=True,
                              env=self.env, check=False)

    def test_refuses_placeholder_secrets_before_docker(self) -> None:
        self.envfile.write_text("PAIRING_CODE=replace-with-your-code\nTOKEN_SECRET=replace-with-your-secret\n")
        result = self.execute()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.log.exists())

    def test_launches_local_host_network_without_clobbering_storage(self) -> None:
        self.envfile.write_text(
            "PAIRING_CODE=abcde12345abcde12345abcde12345\n"
            "TOKEN_SECRET=12345abcdef12345abcdef12345abcdef\n"
            "DATA_DIR=/srv/data\n"
        )
        result = self.execute()
        self.assertEqual(result.returncode, 0, result.stderr)
        commands = self.log.read_text()
        self.assertIn("build --platform linux/arm64", commands)
        self.assertIn("--network host", commands)
        self.assertIn("--read-only", commands)
        self.assertIn("source=jarvis-agent-data,target=/srv/data", commands)
        self.assertIn("container inspect jarvis-gateway", commands)

    def test_rejects_unmounted_data_dir(self) -> None:
        self.envfile.write_text(
            "PAIRING_CODE=abcde12345abcde12345abcde12345\n"
            "TOKEN_SECRET=12345abcdef12345abcdef12345abcdef\n"
            "DATA_DIR=/tmp/unpersisted\n"
        )
        result = self.execute()
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.log.exists())


if __name__ == "__main__":
    unittest.main()
