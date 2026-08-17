#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${ROOT}/dist"

for tool in lb xorriso mksquashfs; do
  if ! command -v "${tool}" >/dev/null 2>&1; then
    echo "Missing required build tool: ${tool}. Install live-build, xorriso, and squashfs-tools first." >&2
    exit 1
  fi
done

"${SCRIPT_DIR}/prepare_live_payload.sh"
mkdir -p "${DIST_DIR}"
cd "${SCRIPT_DIR}"
sudo lb clean --purge || true
lb config
sudo lb build

ISO="${SCRIPT_DIR}/live-image-amd64.hybrid.iso"
if [[ ! -f "${ISO}" ]]; then
  echo "live-build completed without the expected ISO: ${ISO}" >&2
  exit 1
fi

TARGET="${DIST_DIR}/usb-agent-live-amd64.iso"
cp "${ISO}" "${TARGET}"
sha256sum "${TARGET}" > "${TARGET}.sha256"
printf 'Bootable image: %s\nChecksum: %s.sha256\n' "${TARGET}" "${TARGET}"
