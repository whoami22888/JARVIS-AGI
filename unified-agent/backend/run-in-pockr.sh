#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${JARVIS_ENV_FILE:-$ROOT/.env}"
NAME="jarvis-gateway"
IMAGE="jarvis-gateway:pockr"
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
if [[ -n "$data_dir" && "$data_dir" != "/srv/data" ]]; then
  echo "DATA_DIR must be /srv/data so SQLite uses the persistent Docker volume." >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon unavailable inside the Pockr Alpine VM." >&2
  exit 1
fi
if docker container inspect "$NAME" >/dev/null 2>&1; then
  echo "Existing $NAME container found; starting it without overwriting data."
  docker start "$NAME"
  exit 0
fi

docker build --platform linux/arm64 --tag "$IMAGE" "$ROOT"
docker volume create "$VOLUME" >/dev/null
docker run -d \
  --name "$NAME" \
  --network host \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --memory 512m \
  --pids-limit 128 \
  --restart unless-stopped \
  --log-opt max-size=10m \
  --log-opt max-file=3 \
  --mount type=volume,source="$VOLUME",target=/srv/data \
  --env-file "$ENV_FILE" \
  "$IMAGE" >/dev/null

echo "Jarvis ARM64 gateway container created; check http://127.0.0.1:8080/health inside the VM."
echo "Pockr must forward guest port 8080 to the Android handset's 127.0.0.1:8080."
