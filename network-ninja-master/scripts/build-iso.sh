#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.2.0}"
# Dot-prefixed work directories are ignored by Go package discovery, preventing
# privileged live-build artifacts from interfering with `go test ./...`.
WORK="${WORK_DIR:-${ROOT}/build/live/.work}"
DIST="${ROOT}/dist"
BINARY="${DIST}/linux-amd64/network-ninja"
OUTPUT="${DIST}/network-ninja-master-${VERSION}-linux-amd64.iso"

for command in lb rsync sha256sum; do command -v "${command}" >/dev/null || { echo "${command} is required." >&2; exit 1; }; done
[ -x "${BINARY}" ] || { echo "Missing compiled binary: ${BINARY}. Run scripts/build.sh first." >&2; exit 1; }

# A prior privileged live-build can leave root-owned files in this isolated work
# directory, so cleanup is deliberately limited to that exact configured path.
sudo rm -rf "${WORK}"
mkdir -p "${WORK}"
rsync -a --delete "${ROOT}/build/live/config/" "${WORK}/config/"
install -D -m 0755 "${BINARY}" "${WORK}/config/includes.chroot/usr/local/bin/network-ninja"
install -D -m 0644 "${ROOT}/README.md" "${WORK}/config/includes.chroot/usr/share/doc/network-ninja/README.md"
install -D -m 0644 "${ROOT}/docs/ARCHITECTURE.md" "${WORK}/config/includes.chroot/usr/share/doc/network-ninja/ARCHITECTURE.md"
install -D -m 0644 "${ROOT}/docs/SAFETY_AND_FEASIBILITY.md" "${WORK}/config/includes.chroot/usr/share/doc/network-ninja/SAFETY_AND_FEASIBILITY.md"
install -D -m 0644 "${ROOT}/docs/AGENT_CONSOLE_ARCHITECTURE.md" "${WORK}/config/includes.chroot/usr/share/doc/network-ninja/AGENT_CONSOLE_ARCHITECTURE.md"
ln -sf /etc/systemd/system/network-ninja.service "${WORK}/config/includes.chroot/etc/systemd/system/multi-user.target.wants/network-ninja.service"

pushd "${WORK}" >/dev/null
lb config --mode debian --distribution bookworm --architectures amd64 --linux-packages "linux-image" --firmware-chroot false --firmware-binary false --bootloader grub --binary-images iso --archive-areas "main" --security false --keyring-packages "debian-archive-keyring" --mirror-bootstrap "https://deb.debian.org/debian" --mirror-chroot "https://deb.debian.org/debian" --mirror-chroot-security "https://security.debian.org/debian-security" --mirror-binary "https://deb.debian.org/debian" --mirror-binary-security "https://security.debian.org/debian-security" --bootappend-live "boot=live components persistence" --iso-application "Network Ninja Master" --iso-volume "NETWORK_NINJA_${VERSION}"
# live-build requires privileged filesystem operations. The wrapper intentionally
# uses sudo only for the isolated build directory under this repository.
sudo lb build
ISO="$(find . -maxdepth 1 -name '*.iso' -type f | head -n 1)"
[ -n "${ISO}" ] || { echo "live-build did not produce an ISO." >&2; exit 1; }
cp "${ISO}" "${OUTPUT}"
popd >/dev/null

sha256sum "${OUTPUT}" > "${OUTPUT}.sha256"
printf 'Built ISO %s\n' "${OUTPUT}"
