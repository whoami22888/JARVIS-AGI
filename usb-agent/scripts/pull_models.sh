#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROFILE="${1:-balanced}"
ENV_FILE="${ROOT}/.env"
if [[ -f "${ENV_FILE}" ]]; then
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
fi

if ! command -v ollama >/dev/null 2>&1; then
  echo "Ollama is required. Install the supported runtime first, then re-run this script." >&2
  exit 1
fi

export OLLAMA_MODELS="${OLLAMA_MODELS:-${ROOT}/models/store}"
mkdir -p "${OLLAMA_MODELS}" "${ROOT}/checksums"
RESERVE_GB="${MODEL_FREE_SPACE_RESERVE_GB:-12}"
AVAILABLE_GB=$(( $(df -Pk "${OLLAMA_MODELS}" | awk 'NR==2 {print $4}') / 1024 / 1024 ))
if (( AVAILABLE_GB <= RESERVE_GB )); then
  echo "Only ${AVAILABLE_GB} GB is free. The configured reserve is ${RESERVE_GB} GB; refusing model download." >&2
  exit 1
fi

case "${PROFILE}" in
  starter)
    MODELS=("deepseek-r1:1.5b" "qwen2.5-coder:3b")
    ;;
  balanced)
    MODELS=("WhiteRabbitNeo/Llama-3.1-WhiteRabbitNeo-2-8B" "qwen2.5-coder:7b" "deepseek-r1:8b" "llama3.1:8b")
    ;;
  expanded)
    MODELS=("WhiteRabbitNeo/Llama-3.1-WhiteRabbitNeo-2-8B" "qwen2.5-coder:7b" "deepseek-r1:8b" "llama3.1:8b" "deepseek-r1:14b" "qwen2.5-coder:14b")
    ;;
  *)
    echo "Usage: $0 {starter|balanced|expanded}" >&2
    exit 64
    ;;
esac

printf 'Selected profile: %s\nFree capacity before download: %s GB\n' "${PROFILE}" "${AVAILABLE_GB}"
printf 'Models:\n'; printf ' - %s\n' "${MODELS[@]}"
read -r -p "Proceed with these model downloads? [y/N] " CONFIRM
if [[ "${CONFIRM}" != "y" && "${CONFIRM}" != "Y" ]]; then
  echo "Model download cancelled."
  exit 0
fi

if ! curl --silent --fail http://127.0.0.1:11434/api/tags >/dev/null 2>&1; then
  nohup ollama serve >"${ROOT}/data/ollama-pull.log" 2>&1 &
  sleep 2
fi

for model in "${MODELS[@]}"; do
  echo "Pulling ${model}"
  ollama pull "${model}"
done

MANIFEST="${ROOT}/models/installed-$(date -u +%Y%m%dT%H%M%SZ).txt"
ollama list | tee "${MANIFEST}"
sha256sum "${MANIFEST}" > "${MANIFEST}.sha256"
echo "Model manifest written to ${MANIFEST}"
