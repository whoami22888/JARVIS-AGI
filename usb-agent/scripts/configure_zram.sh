#!/usr/bin/env bash
set -euo pipefail

# Owner-invoked performance setup. It never enables USB-backed swap.
if [[ "${1:-}" != "--enable" ]]; then
  cat <<'USAGE'
Usage: sudo ./configure_zram.sh --enable

Creates a bounded zram swap device for temporary memory pressure only. It does
not configure disk/USB swap and records the chosen configuration in data/zram.json.
USAGE
  exit 0
fi

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run this command through sudo after reviewing the requested zram configuration." >&2
  exit 1
fi

ROOT="${USB_AGENT_HOME:-$(cd "$(dirname "$0")/.." && pwd)}"
DATA="${USB_AGENT_DATA:-$ROOT/data}"
mkdir -p "$DATA"
MEM_MB="$(awk '/MemTotal/ {print int($2/1024)}' /proc/meminfo)"
if (( MEM_MB < 4000 )); then
  echo "Refusing zram on hosts below 4 GB RAM; keep the workload single-model instead." >&2
  exit 1
fi

# Cap at roughly one third of physical memory, with a 1 GB minimum and 24 GB maximum.
SIZE_MB=$(( MEM_MB / 3 ))
(( SIZE_MB < 1024 )) && SIZE_MB=1024
(( SIZE_MB > 24576 )) && SIZE_MB=24576

modprobe zram
if [[ ! -b /dev/zram0 ]]; then
  echo "zram0 device is not available after loading the zram module." >&2
  exit 1
fi
swapoff /dev/zram0 2>/dev/null || true
echo $(( SIZE_MB * 1024 * 1024 )) > /sys/block/zram0/disksize
mkswap /dev/zram0 >/dev/null
swapon --priority 100 /dev/zram0
printf '{"enabled_at":"%s","zram_mb":%s,"usb_swap":false}\n' "$(date -u +%FT%TZ)" "$SIZE_MB" > "$DATA/zram.json"
echo "Enabled ${SIZE_MB} MB zram. USB-backed swap remains disabled."
