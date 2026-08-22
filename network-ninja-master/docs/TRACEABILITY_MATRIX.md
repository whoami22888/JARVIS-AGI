# Requirement Traceability Matrix

**Release target:** Network Ninja Master `0.1.0`
**Evidence convention:** `PENDING` means evidence must be produced by the final validation phase; `BLOCKED` is an honest external limitation, not a pass.

| Requirement | Module / artifact | API or operator path | Test / evidence | Initial status |
|---|---|---|---|---|
| Explicit authorized scope before active network work | `internal/scope` | `scope validate`; all scan requests | Unit tests for expiry, targets, ports, operations; audit records | `IMPLEMENTING` |
| Scope refusal outside boundary | `internal/scope`, `internal/scan` | `scan tcp`, `scan tls`, `scan http` | Negative test of excluded/unauthorized target | `IMPLEMENTING` |
| Local host/system discovery | `internal/device` | `host inspect` / dashboard | JSON evidence + unit tests | `IMPLEMENTING` |
| USB/serial device discovery | `internal/device` | `device inventory` / dashboard | Local `/sys` inspection evidence | `IMPLEMENTING` |
| Bounded TCP service assessment | `internal/scan` | `scan tcp` | Loopback integration test; audit log | `IMPLEMENTING` |
| TLS metadata inspection | `internal/scan` | `scan tls` | Local TLS integration test | `IMPLEMENTING` |
| HTTP metadata inspection | `internal/scan` | `scan http` | Local HTTP integration test | `IMPLEMENTING` |
| Evidence-backed job/audit store | `internal/audit`, `internal/store` | all commands / API | SQLite integrity and evidence-digest tests | `IMPLEMENTING` |
| Tool registry and health gate | `internal/store` | `tools list`; manifest validation | Registry hash and allowlist tests | `IMPLEMENTING` |
| Local loopback dashboard | `cmd/network-ninja`, `web` | `serve` → `127.0.0.1` | HTTP API smoke test | `IMPLEMENTING` |
| Optional local AI backends | `internal/agent` (future integration point) | `agent status` | Backend-absent test | `PLANNED` |
| ADB/Fastboot discovery | future device adapter | future `device adb` | Needs installed tools and physical authorized device | `BLOCKED` |
| Supported file retrieval | future device/file adapter | future recovery workflow | Needs authorized source device and data set | `BLOCKED` |
| Firmware compatibility manager | future vendor profile layer | future firmware workflow | Needs manufacturer sources/device profiles/signatures | `BLOCKED` |
| Flash/recovery write operations | future vendor adapters | future guarded workflow | Needs physical device and separate destructive-operation approval | `BLOCKED` |
| Windows x64/ARM64 builds | cross-platform build scripts | `build.ps1` | Windows compiler and OS validation | `BLOCKED` |
| Linux x86_64 portable binary | `scripts/build.sh` | binary release asset | clean build + tests | `IMPLEMENTING` |
| Linux ARM64 and macOS binaries | cross-compilation targets | build scripts | Native test environments required before publishing | `NOT_STARTED` |
| Bootable Linux ISO | `build/live`, `scripts/build-iso.sh` | live boot → launcher | ISO inspection + QEMU boot attempt | `IMPLEMENTING` |
| Ventoy compatibility materials | `build/ventoy`, release manifest | copy ISO to Ventoy first partition | configuration JSON validation; physical Ventoy required | `IMPLEMENTING / PHYSICAL TEST BLOCKED` |
| Persistence strategy | `build/ventoy/ventoy.json`, docs | Ventoy persistence selection | config parse, actual USB test required | `IMPLEMENTING / PHYSICAL TEST BLOCKED` |
| SHA-256 release checksums | `scripts/package-release.sh` | release verification | `sha256sum -c` | `IMPLEMENTING` |
| SBOM and third-party notices | `scripts/generate-sbom.sh`, notices | release package | generated JSON review | `IMPLEMENTING` |
| Reproducible build documentation | `Makefile`, build scripts | documented build commands | clean rebuild comparison | `IMPLEMENTING` |
| Security self-tests | `tests`, `go test` | CI/local test | command/input/scope negative tests | `IMPLEMENTING` |
| Final verification report | `docs/VERIFICATION_REPORT.md` | release package | generated evidence references | `PENDING` |

## Status definitions

| Status | Meaning |
|---|---|
| `IMPLEMENTING` | Source is actively being developed and has no verification claim yet. |
| `PENDING` | Required artifact exists only after final execution/validation. |
| `BLOCKED` | An external environment, authorization, hardware device, or vendor resource is absent. |
| `NOT_STARTED` | Not yet implemented. |
| `VERIFIED` | Reserved for a real implementation with retained independent execution evidence. |
