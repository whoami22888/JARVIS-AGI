#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ROOT}/.env"
DATA_DIR="${ROOT}/data"
LOG_DIR="${DATA_DIR}/logs"
PID_DIR="${DATA_DIR}/run"

mkdir -p "${LOG_DIR}" "${PID_DIR}"
if [[ -f "${ENV_FILE}" ]]; then
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
fi

export USB_AGENT_HOME="${USB_AGENT_HOME:-${ROOT}}"
export USB_AGENT_DATA="${USB_AGENT_DATA:-${DATA_DIR}}"
export USB_AGENT_CONFIG="${USB_AGENT_CONFIG:-${ROOT}/config}"
export USB_AGENT_HOST="${USB_AGENT_HOST:-127.0.0.1}"
export USB_AGENT_PORT="${USB_AGENT_PORT:-8787}"
export PYTHONPATH="${ROOT}/backend${PYTHONPATH:+:${PYTHONPATH}}"

if [[ -z "${VIRTUAL_ENV:-}" ]]; then
  if [[ ! -d "${ROOT}/.venv" ]]; then
    python3 -m venv "${ROOT}/.venv"
  fi
  # shellcheck disable=SC1091
  source "${ROOT}/.venv/bin/activate"
fi
python3 -m pip install --quiet --upgrade -r "${ROOT}/backend/requirements.txt"

if command -v ollama >/dev/null 2>&1 && ! curl --silent --fail http://127.0.0.1:11434/api/tags >/dev/null 2>&1; then
  export OLLAMA_MODELS="${OLLAMA_MODELS:-${ROOT}/models/store}"
  mkdir -p "${OLLAMA_MODELS}"
  nohup ollama serve >"${LOG_DIR}/ollama.log" 2>&1 &
  echo $! >"${PID_DIR}/ollama.pid"
fi

if curl --silent --fail "http://${USB_AGENT_HOST}:${USB_AGENT_PORT}/health" >/dev/null 2>&1; then
  echo "USB Agent is already running at http://${USB_AGENT_HOST}:${USB_AGENT_PORT}"
  exit 0
fi

nohup python3 -m uvicorn app:app --app-dir "${ROOT}/backend" --host "${USB_AGENT_HOST}" --port "${USB_AGENT_PORT}" >"${LOG_DIR}/gateway.log" 2>&1 &
echo $! >"${PID_DIR}/gateway.pid"

for _ in {1..30}; do
  if curl --silent --fail "http://${USB_AGENT_HOST}:${USB_AGENT_PORT}/health" >/dev/null 2>&1; then
    echo "USB Agent started: http://${USB_AGENT_HOST}:${USB_AGENT_PORT}"
    if command -v xdg-open >/dev/null 2>&1; then xdg-open "http://${USB_AGENT_HOST}:${USB_AGENT_PORT}" >/dev/null 2>&1 || true; fi
    exit 0
  fi
  sleep 1
done

echo "Gateway did not become healthy. See ${LOG_DIR}/gateway.log" >&2
exit 1
