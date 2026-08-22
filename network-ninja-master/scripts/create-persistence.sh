#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: create-persistence.sh --output /path/to/network-ninja-master-persistence.dat [--size-mb 4096]

Creates an ext4 Ventoy persistence backend image labeled "persistence" with a
Debian-live persistence.conf file containing "/ union". This script creates or
overwrites only the explicit --output file; it never formats a disk device.
EOF
}

OUTPUT=""
SIZE_MB=4096
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) OUTPUT="${2:-}"; shift 2 ;;
    --size-mb) SIZE_MB="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ -n "${OUTPUT}" ]] || { echo "--output is required" >&2; exit 2; }
[[ "${SIZE_MB}" =~ ^[0-9]+$ ]] && [ "${SIZE_MB}" -ge 256 ] && [ "${SIZE_MB}" -le 65536 ] || { echo "--size-mb must be an integer between 256 and 65536" >&2; exit 2; }
case "${OUTPUT}" in
  /dev/*) echo "Refusing block-device paths. Provide a regular image-file path." >&2; exit 2 ;;
esac
command -v truncate >/dev/null && command -v mkfs.ext4 >/dev/null && command -v debugfs >/dev/null || { echo "truncate, mkfs.ext4, and debugfs are required." >&2; exit 1; }

mkdir -p "$(dirname -- "${OUTPUT}")"
if [ -e "${OUTPUT}" ]; then
  read -r -p "Overwrite ${OUTPUT}? Type CREATE_PERSISTENCE_IMAGE to continue: " confirmation
  [ "${confirmation}" = "CREATE_PERSISTENCE_IMAGE" ] || { echo "Cancelled."; exit 1; }
fi

tmp="${OUTPUT}.tmp.$$"
config_tmp="${OUTPUT}.persistence.conf.$$"
trap 'rm -f "${tmp}" "${config_tmp}"' EXIT
truncate -s "$((SIZE_MB * 1024 * 1024))" "${tmp}"
mkfs.ext4 -F -L persistence "${tmp}" >/dev/null
printf '/ union\n' > "${config_tmp}"
debugfs -w -R "write ${config_tmp} persistence.conf" "${tmp}" >/dev/null 2>&1
mv -f "${tmp}" "${OUTPUT}"
rm -f "${config_tmp}"
trap - EXIT
printf 'Created %s\n' "${OUTPUT}"
sha256sum "${OUTPUT}" > "${OUTPUT}.sha256"
printf 'SHA-256 written to %s.sha256\n' "${OUTPUT}"
