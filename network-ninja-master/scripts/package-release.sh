#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.2.0}"
STAGE="${ROOT}/dist/release/network-ninja-master-${VERSION}"
BINARY="${ROOT}/dist/linux-amd64/network-ninja"

[ -x "${BINARY}" ] || { echo "Missing Linux binary. Run scripts/build.sh first." >&2; exit 1; }
rm -rf "${STAGE}"
mkdir -p "${STAGE}"/{linux/x64,ventoy,documentation,evidence,licenses}
install -m 0755 "${BINARY}" "${STAGE}/linux/x64/network-ninja"
install -m 0644 "${ROOT}/dist/linux-amd64/network-ninja.sha256" "${STAGE}/linux/x64/"
cp -a "${ROOT}/build/ventoy/ventoy.json" "${STAGE}/ventoy/"
cp -a "${ROOT}/scripts/create-persistence.sh" "${STAGE}/ventoy/"
cp -a "${ROOT}/docs/." "${STAGE}/documentation/"
cp -a "${ROOT}/README.md" "${STAGE}/documentation/README.md"
install -m 0644 "${ROOT}/build-state.json" "${STAGE}/build-state.json"
cp -a "${ROOT}/licenses/." "${STAGE}/licenses/" 2>/dev/null || true

if [ -f "${ROOT}/dist/network-ninja-master-${VERSION}-linux-amd64.iso" ]; then
  cp "${ROOT}/dist/network-ninja-master-${VERSION}-linux-amd64.iso" "${STAGE}/ventoy/"
  cp "${ROOT}/dist/network-ninja-master-${VERSION}-linux-amd64.iso.sha256" "${STAGE}/ventoy/"
fi
if [ -d "${ROOT}/evidence" ]; then cp -a "${ROOT}/evidence/." "${STAGE}/evidence/" 2>/dev/null || true; fi
VERSION="${VERSION}" "${ROOT}/scripts/generate-sbom.sh" "${STAGE}/sbom.json"

cat > "${STAGE}/release-manifest.json" <<EOF
{
  "name": "Network Ninja Master",
  "version": "${VERSION}",
  "release_status": "validated_software_baseline",
  "architecture": ["linux-amd64"],
  "supported_boot_deployment": "Ventoy (physical validation required)",
  "data_persistence": "Ventoy persistence backend configured in ventoy/ventoy.json",
  "verified_capabilities": ["scope validation", "local host inventory", "local device inventory", "bounded TCP diagnostics", "bounded TLS metadata diagnostics", "bounded HTTP metadata diagnostics", "append-only audit", "SQLite state", "contextual local chat", "visible activity history", "approval-gated local command catalog", "operator takeover", "local memory", "inert chat-added skill drafts", "manual endpoint snapshots"],
  "blocked_capabilities": ["physical Ventoy boot validation", "UEFI boot validation", "physical USB device testing", "ADB/Fastboot", "firmware operations", "Windows/macOS native validation", "vendor driver workflows"],
  "integrity": "See SHA256SUMS"
}
EOF
(
  cd "${STAGE}"
  find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS
)
printf 'Release staged at %s\n' "${STAGE}"
