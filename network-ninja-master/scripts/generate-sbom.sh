#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT="${1:-${ROOT}/dist/sbom.json}"
mkdir -p "$(dirname -- "${OUTPUT}")"
cd "${ROOT}"

modules="$(go list -m -f '{{.Path}} {{.Version}}' all | awk 'BEGIN { separator="" } { printf "%s{\"name\":\"%s\",\"version\":\"%s\",\"type\":\"go-module\"}", separator, $1, $2; separator="," }')"
cat > "${OUTPUT}" <<EOF
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "serialNumber": "urn:uuid:network-ninja-master-${VERSION:-0.2.1}",
  "version": 1,
  "metadata": {
    "component": {
      "type": "application",
      "name": "Network Ninja Master",
      "version": "${VERSION:-0.2.1}",
      "description": "Authorized, scope-enforced portable diagnostics runtime"
    }
  },
  "components": [${modules}]
}
EOF
printf 'Wrote %s\n' "${OUTPUT}"
