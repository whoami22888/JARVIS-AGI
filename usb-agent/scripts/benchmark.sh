#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BASE_URL="${USB_AGENT_URL:-http://127.0.0.1:8787}"
OUT_DIR="${ROOT}/data/benchmarks"
mkdir -p "${OUT_DIR}"
OUT_FILE="${OUT_DIR}/benchmark-$(date -u +%Y%m%dT%H%M%SZ).json"

health_start=$(date +%s%3N)
health=$(curl --silent --show-error --fail "${BASE_URL}/health")
health_end=$(date +%s%3N)

status_start=$(date +%s%3N)
status=$(curl --silent --show-error --fail "${BASE_URL}/api/status")
status_end=$(date +%s%3N)

native_start=$(date +%s%3N)
task=$(curl --silent --show-error --fail -X POST "${BASE_URL}/api/tasks" \
  -H 'Content-Type: application/json' \
  --data '{"prompt":"system status"}')
native_end=$(date +%s%3N)

task_id=$(printf '%s' "${task}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
for _ in {1..20}; do
  detail=$(curl --silent --show-error --fail "${BASE_URL}/api/tasks/${task_id}")
  state=$(printf '%s' "${detail}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["state"])')
  [[ "${state}" == "COMPLETED" || "${state}" == "FAILED" ]] && break
  sleep .25
done

python3 - <<PY > "${OUT_FILE}"
import json
payload = {
  "health_latency_ms": ${health_end} - ${health_start},
  "status_latency_ms": ${status_end} - ${status_start},
  "native_task_submit_latency_ms": ${native_end} - ${native_start},
  "health": json.loads('''${health}'''),
  "status": json.loads('''${status}'''),
  "native_task": json.loads('''${detail}''')
}
print(json.dumps(payload, indent=2))
PY
sha256sum "${OUT_FILE}" > "${OUT_FILE}.sha256"
echo "Benchmark written to ${OUT_FILE}"
