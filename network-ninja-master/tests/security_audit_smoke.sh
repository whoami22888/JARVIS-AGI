#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
EVIDENCE="${ROOT}/evidence/security-audit-v020"
DATA="/tmp/network-ninja-audit-smoke"
PORT="18062"
BINARY="${ROOT}/dist/linux-amd64/network-ninja"

mkdir -p "${EVIDENCE}"
rm -rf "${DATA}"
"${BINARY}" serve --addr "127.0.0.1:${PORT}" --data-root "${DATA}" > /tmp/network-ninja-audit-server.log 2>&1 &
pid=$!
trap 'kill "${pid}" 2>/dev/null || true' EXIT

ready=0
for _ in 1 2 3 4 5 6 7 8 9 10; do
  if curl -fsS "http://127.0.0.1:${PORT}/api/agent/status" >/dev/null; then
    ready=1
    break
  fi
  sleep 1
done
[ "${ready}" -eq 1 ]

curl -fsS "http://127.0.0.1:${PORT}/api/agent/status" > "${EVIDENCE}/token-status.json"
token="$(jq -r '.write_token' "${EVIDENCE}/token-status.json")"
test "${#token}" -ge 32

missing_status="$(curl -sS -o "${EVIDENCE}/token-missing.json" -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data '{"message":"without token"}' "http://127.0.0.1:${PORT}/api/agent/chat")"
test "${missing_status}" = "403"

oversize="${EVIDENCE}/oversized-request.json"
printf '{"message":"' > "${oversize}"
head -c 70000 /dev/zero | tr '\0' 'x' >> "${oversize}"
printf '"}' >> "${oversize}"
oversize_status="$(curl -sS -o "${EVIDENCE}/token-oversize.json" -w '%{http_code}' -X POST -H "X-Network-Ninja-Write-Token: ${token}" -H 'Content-Type: application/json' --data-binary "@${oversize}" "http://127.0.0.1:${PORT}/api/agent/chat")"
test "${oversize_status}" = "400"

for _ in $(seq 1 300); do
  curl -fsS -X POST -H "X-Network-Ninja-Write-Token: ${token}" -H 'Content-Type: application/json' --data '{"message":"bounded audit traffic"}' "http://127.0.0.1:${PORT}/api/agent/chat" >/dev/null
done
curl -fsS "http://127.0.0.1:${PORT}/api/agent/messages" > "${EVIDENCE}/postfix-messages.json"
jq -e '(.messages | length) <= 80' "${EVIDENCE}/postfix-messages.json" >/dev/null

curl -fsS -X POST -H "X-Network-Ninja-Write-Token: ${token}" "http://127.0.0.1:${PORT}/api/agent/commands/host_inventory/propose" > "${EVIDENCE}/token-proposal.json"
task="$(jq -r '.id' "${EVIDENCE}/token-proposal.json")"
: > "${EVIDENCE}/concurrent-approval-statuses.txt"
worker_pids=()
for worker in $(seq 1 12); do
  (
    curl -sS -o "/tmp/network-ninja-audit-approve-${worker}.json" -w '%{http_code}' -X POST -H "X-Network-Ninja-Write-Token: ${token}" "http://127.0.0.1:${PORT}/api/agent/tasks/${task}/approve" >> "${EVIDENCE}/concurrent-approval-statuses.txt"
  ) &
  worker_pids+=("$!")
done
for worker_pid in "${worker_pids[@]}"; do
  wait "${worker_pid}"
done
[ "$(grep -o '200' "${EVIDENCE}/concurrent-approval-statuses.txt" | wc -l)" = "1" ]
curl -fsS "http://127.0.0.1:${PORT}/api/agent/tasks" > "${EVIDENCE}/concurrent-approval-tasks.json"
jq -e --arg id "${task}" '[.tasks[] | select(.id == $id and .status == "COMPLETED")] | length == 1' "${EVIDENCE}/concurrent-approval-tasks.json" >/dev/null

printf 'Security audit smoke test passed.\n'
