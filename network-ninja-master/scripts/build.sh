#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${OUT_DIR:-${ROOT}/dist/linux-amd64}"
VERSION="${VERSION:-0.1.0}"

command -v go >/dev/null || { echo "Go is required." >&2; exit 1; }
mkdir -p "${OUT}"
cd "${ROOT}"

go mod download
go vet ./...
go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=${VERSION}" -o "${OUT}/network-ninja" ./cmd/network-ninja
"${OUT}/network-ninja" version
sha256sum "${OUT}/network-ninja" > "${OUT}/network-ninja.sha256"
printf 'Built %s\n' "${OUT}/network-ninja"
