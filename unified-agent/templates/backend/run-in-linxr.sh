#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${JARVIS_ENV_FILE:-$ROOT/.env}"
NAME="jarvis-gateway"
IMAGE="jarvis-gateway:linxr"
VOLUME="jarvis-agent-data"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Create $ENV_FILE from .env.example and set your own secrets first." >&2
  exit 1
fi

for key in PAIRING_CODE TOKEN_SECRET; do
  value="$(sed -n "s/^${key}=//p" "$ENV_FILE" | tail -n 1)"
  if [[ ${#value} -lt 24 || "$value" == *replace* || "$value" == *development* ]]; then
    echo "$key must be at least 24 characters and not a sample value." >&2
    exit 1
  fi
done
data_dir="$(sed -n 's/^DATA_DIR=//p' "$ENV_FILE" | tail -n 1)"
if [[ "$data_dir" != "/srv/data" ]]; then
  echo "DATA_DIR must be /srv/data so SQLite uses the persistent container volume." >&2
  exit 1
fi
if ! command -v podman >/dev/null 2>&1; then
  echo "Podman is not installed inside the LinxR guest; verify FastTrack mode." >&2
  exit 1
fi
case "$(uname -m)" in
  aarch64|arm64) ;;
  *) echo "Requires a native ARM64 LinxR Alpine guest." >&2; exit 1 ;;
esac
if ! podman info >/dev/null 2>&1; then
  echo "Podman is unavailable; inspect guest storage and container kernel support." >&2
  exit 1
fi
if podman container exists "$NAME"; then
  echo "Existing $NAME container found; starting it without overwriting data."
  podman start "$NAME"
  exit 0
fi

podman build --tag "$IMAGE" "$ROOT"
podman volume create "$VOLUME" >/dev/null
podman run -d \
  --name "$NAME" \
  --network host \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --memory 512m \
  --pids-limit 128 \
  --restart unless-stopped \
  --mount type=volume,source="$VOLUME",target=/srv/data \
  --env-file "$ENV_FILE" \
  "$IMAGE" >/dev/null

echo "Jarvis ARM64 gateway container created; check http://127.0.0.1:8080/health inside the VM."
echo "LinxR guest-to-Android port 8080 forwarding is unverified; test on the phone."
