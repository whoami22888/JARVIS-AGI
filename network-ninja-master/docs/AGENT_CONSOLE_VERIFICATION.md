# Network Ninja Master 0.2.0 — Agent Console Verification

## Release status

**Status:** `VALIDATED_SOFTWARE_BASELINE`

Network Ninja Master 0.2.0 adds an original local agent console to the previously validated scope-enforced diagnostic runtime. The console was built and tested as a loopback-only service. Its chat, contextual memory, activity history, typed local command proposals, approval flow, operator takeover, skill-draft workflow, endpoint snapshot, and ISO-embedded runtime passed the checks recorded below.

> This status validates the software artifact and local smoke paths. It does **not** claim a successful physical Ventoy USB boot, a physical device-control workflow, or UEFI boot compatibility.

## Passing validation record

| Check | Result | Evidence |
|---|---|---|
| Format, complete Go test suite, and static analysis | `PASSED` | `make fmt test vet build` output; `internal/agent/agent_test.go` |
| Versioned Linux binary build | `PASSED` | `dist/linux-amd64/network-ninja` reports `0.2.0` |
| Contextual local chat persistence | `PASSED` | `evidence/agent-console-chat.json` |
| Chat-added skill draft and approval | `PASSED` | `evidence/agent-console-skill-draft.json`, `evidence/agent-console-skill-approved.json` |
| Approval-gated host inventory | `PASSED` | `evidence/agent-console-proposal-approved.json`, `evidence/agent-console-task-completed.json` |
| Takeover pauses pending approval | `PASSED` | `evidence/agent-console-takeover-start.json`, `evidence/agent-console-takeover-refusal.json`, `evidence/agent-console-takeover-end.json` |
| Cancellation prevents task execution | `PASSED` | `evidence/agent-console-cancelled.json` |
| Manual endpoint snapshot and visible timeline | `PASSED` | `evidence/agent-console-monitor-completed.json`, `evidence/agent-console-activity.json` |
| Loopback-only enforcement | `PASSED` | `evidence/agent-console-nonloopback-refusal.txt` |
| New original console asset is served by the runtime | `PASSED` | Local smoke test checked `Network Ninja Console` and `Take control` in the served response. |
| 0.2.0 live ISO build and checksum | `PASSED` | `dist/network-ninja-master-0.2.0-linux-amd64.iso`, adjacent checksum |
| Embedded ISO runtime and agent console | `PASSED` | `evidence/v020-iso-boot-metadata.txt`, `evidence/v020-iso-extract.log`, `evidence/v020-iso-inspection.sha256` |

## Defects found and corrected during validation

| Defect | Correction | Regression result |
|---|---|---|
| Context construction could block against SQLite because a history query opened a nested query while one connection was available. | Reworked task and skill list methods to scan full rows from the original query. | Agent orchestration tests pass. |
| Nullable task error values were decoded into a non-nullable Go string. | Added null-aware SQLite decoding for task error fields. | Cancellation, approval, and completion tests pass. |
| Initial ISO inspection used a listing form that did not expose paths as expected. | Replaced it with direct `xorriso` live-directory inspection and extracted-runtime validation. | ISO binary reports `0.2.0`, serves the original console, and accepts a tested skill draft. |

## Capability boundaries

The console provides **human-approved local orchestration**, not unrestricted autonomy. Chat may prepare a typed collector proposal but never turns user text into a shell command. The command catalog currently contains read-only host inventory, read-only device inventory, and a visible manual endpoint snapshot. Chat-added skills are local data records, remain inert, and require review before their status can become `APPROVED`; approval does not install or execute supplied content.

An optional text-only model endpoint can enrich the chat response when an operator configures it. The default accelerator remains a deterministic local context builder. Neither mode grants a model command-execution authority.

## Remaining release gates

| Gate | Status | Required next evidence |
|---|---|---|
| Physical Ventoy USB boot and persistence across reboot | `BLOCKED` | Test on an authorized disposable USB with documented firmware mode and persistence read/write proof. |
| UEFI boot | `BLOCKED` | Add an EFI boot image and validate it in OVMF plus physical UEFI firmware. |
| Remote enrolled endpoint agent | `NOT_IMPLEMENTED` | Device identity, encrypted pairing, explicit remote session approval, and physical multi-host testing. |
| UI-driven mouse, keyboard, or screen-control takeover | `NOT_IMPLEMENTED` | A separate local-desktop permission model, visible live session, emergency stop, and target-machine testing. |
| Automated remediation or patching | `NOT_IMPLEMENTED` | Separate change approval, preflight, backup, health check, and rollback evidence. |

## Operator start

Build and start the local console on the same host:

```bash
make fmt test vet build
./dist/linux-amd64/network-ninja serve --addr 127.0.0.1:8000
```

Open `http://127.0.0.1:8000`. Use the visible command cards or chat to create a proposal, review the task, and approve it only when ready. Run `make iso` to create the 0.2.0 Ventoy candidate and `make package` to stage the release materials.
