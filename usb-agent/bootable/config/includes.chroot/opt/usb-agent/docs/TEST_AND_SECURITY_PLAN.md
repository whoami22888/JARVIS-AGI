# USB Agent — Test and Security Plan

## Security Boundary

The gateway is a local-first orchestration service, not an authority to control a host or mobile device. Every filesystem, network, code-execution, elevated-diagnostic, or mobile-device action must be checked against a named capability, a current owner approval, and an allowlisted target. A model response cannot create or broaden access. The UI must display the requested target, reason, capability, and expiry before confirmation.

| Boundary | Default | Required approval | Test |
|---|---|---|---|
| Gateway network binding | Loopback only | Explicit pairing and LAN enablement | External address rejected by default; paired token required for expanded binding. |
| Filesystem | No access | `read_files` or `write_files`, approved roots | Path traversal, symlink escape, and unapproved root attempts fail. |
| Network retrieval | Disabled for private/local targets | `network_read` plus approved target policy | Loopback, link-local, private subnet, and unsupported schemes fail. |
| Code execution | Disabled | `run_tests` in approved workspace | AST/command allowlist, timeout, resource cap, output cap, and subprocess isolation are tested. |
| Elevated diagnostics | Disabled | `elevated_diagnostics`, UAC prompt | Non-allowlisted action fails; audit record is written on success/failure. |
| Android action | Proposal only | Device-local confirmation | Action cannot execute from gateway API alone. |

## Automated Validation Layers

| Layer | Scope | Required checks |
|---|---|---|
| Unit | SQLite, router, capability store, manifest verifier, parser, scheduler | Success, malformed input, missing configuration, expiry, idempotency, and error paths. |
| API integration | FastAPI REST/WebSocket endpoints | Authentication, pairing, task lifecycle, status schema, capability denials, approval lifecycle, event fan-out. |
| Filesystem/tool integration | Workspace connector, knowledge index, code tests | Allowlist enforcement, read-only default, hashing, change detection, size caps, redaction, cleanup. |
| Dashboard smoke | Browser-facing static UI and API contract | Setup, login/pair, offline mode, template dispatch, timeline, alerts, permission state, error rendering. |
| Launcher/configuration | Linux and Windows scripts | Missing runtime, unavailable port, invalid data path, corrupt manifest, insufficient space, restart, log creation. |
| Android | Gradle lint/tests/debug build plus manifest inspection | HTTPS-only endpoint check, pairing storage, disconnect, error state, approval UX, no embedded secret. |
| Release/package | Archive/ISO/manifest | Checksums, no tracked secrets, reproducible source archive, boot payload inventory, update rollback metadata. |

## Test Gate

The release gate is green only when all automated tests pass, `git diff --check` passes, static compilation succeeds, source and release manifests verify, the dashboard returns an authenticated health/status flow, the launcher failure paths are exercised, and the Android debug build succeeds if the Android toolchain is available.

Physical hardware boot, local-model inference, USB encryption setup, and Android device testing are separately recorded as **owner-environment validation**. They cannot be asserted from the build sandbox without the physical USB/device and the user-controlled credential or encryption setup.
