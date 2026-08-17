# USB Agent — 34-Item Improvement Acceptance Matrix

This matrix is the authoritative implementation backlog for the fresh USB Agent. An item can only be marked **implemented** after its acceptance test passes. Items that require owner-controlled secrets, a physical USB, an Android device, an Android SDK licence acceptance, or a compatible local model runtime will be implemented with a tested configuration path and marked **deployment prerequisite** until the owner supplies that prerequisite.

> The system will support host-device operations only through explicit, visible, capability-scoped approval. It will not receive universal unattended access merely because a permission was previously granted.

| ID | Area | Improvement | Acceptance criterion | Initial status |
|---:|---|---|---|---|
| 1 | Knowledge | Offline knowledge vault | Approved text/code documents can be indexed, searched, cited by local path/chunk, and removed from the index. | Planned |
| 2 | Skills | Skill manifest and health registry | Every registered skill has a manifest, integrity hash, capability list, self-test result, and HUD status. | Planned |
| 3 | Host files | Local network/workspace connector | Owner-approved paths are allowlisted, default read-only, audited, and disconnectable. | Planned |
| 4 | Coding | Code workspace assistant | Indexed workspace search, patch preview, explicit apply confirmation, and bounded test execution work end-to-end. | Planned |
| 5 | Models | Model evaluation and routing lab | Benchmark records persist model, profile, latency, memory, error rate, and route recommendation. | Planned |
| 6 | Security | Encrypted persistent data design | Release provides a separate encrypted-data setup path, detects an unlocked store, and never stores secrets in source. | Planned |
| 7 | Security | Expiring gateway sessions | Pairing stores verifier material, issues a bounded token, supports revocation, and rejects expired tokens. | Planned |
| 8 | Security | Signed release manifest | Runtime and skill manifest hashes are verified at launch; mismatch is visible and blocks affected execution. | Planned |
| 9 | Security | Capability-scoped tool runner | Tools require named capabilities, show owner approval state, and reject missing/expired approval. | Planned |
| 10 | Security | Elevated helper audit trail | Elevated diagnostics use an allowlist, visible UAC path, JSON audit event, and no inherited broad agent privilege. | Partial |
| 11 | Setup | First-run setup wizard | The HUD detects profile, free space, model availability, and gives an explicit model-download estimate. | Planned |
| 12 | Support | Redacted diagnostic bundle | Owner can create a bundle with host/model/checksum/benchmark data while excluding prompts, tokens, and private files by default. | Planned |
| 13 | Tasks | Task templates | Built-in templates prepopulate model, capability set, output shape, and confirmation needs. | Planned |
| 14 | Privacy | Offline-only control | A persistent UI control shows local/remote route state and blocks remote-provider execution when enabled. | Planned |
| 15 | Storage | Safe storage dashboard | HUD shows USB/cache/index/model/database use plus a non-destructive cleanup recommendation. | Planned |
| 16 | HUD | Performance ribbon | HUD charts CPU, RAM, storage queue, active route, first-token latency, and task latency over time. | Planned |
| 17 | HUD | Model topology map | HUD shows model availability, loaded state, route purpose, fallback chain, and local/remote boundary. | Planned |
| 18 | HUD | Task checkpoint timeline | Each task records and displays queue, routing, retrieval, tool, response, approval, and final/error checkpoints. | Planned |
| 19 | HUD | Capability and permission panel | HUD shows skills, capability approvals, allowed folders, token/session state, and elevated-helper state. | Planned |
| 20 | HUD | Storage/cache heatmap | HUD shows USB/cache usage, parser queue, cache hits, model read state, and cleanup opportunities. | Planned |
| 21 | Automation | Launch integrity and readiness check | Startup validates manifest, free-space reserve, model runtime, registry, and configured data locations before use. | Planned |
| 22 | Automation | Idle-time indexing queue | Queue persists a cursor, respects allowlists, pauses under resource pressure, and never interrupts interactive work. | Planned |
| 23 | Automation | Adaptive benchmark scheduler | Scheduler runs only when idle or owner-triggered, records host change reason, and never overlaps interactive tasks. | Planned |
| 24 | Automation | Versioned update staging and rollback | Update is staged, verified, smoke-tested, activated atomically, and can roll back to prior release metadata. | Planned |
| 25 | Automation | Follow-up reminders | Local notices surface low space, missing model, failed benchmark, expired session, and pending approval without auto-action. | Planned |
| 26 | Performance | Bounded hot-memory working set | Active context, model state, parser buffers, and queues remain inside the selected host profile budget. | Planned |
| 27 | Performance | Optional bounded zram tier | Linux launcher can configure zram only when supported and within a conservative profile percentage; it records activation. | Planned |
| 28 | Performance | Persistent encrypted cache tier | USB/owner-approved local cache paths are explicit, encrypted where configured, and never misrepresented as RAM. | Planned |
| 29 | Performance | No USB-backed inference swap | Runtime warns and refuses to enable USB-backed swap for interactive inference. | Planned |
| 30 | Performance | Single-model load and idle unload policy | Router exposes loaded/active model state and prevents concurrent heavyweight model loading beyond profile capacity. | Planned |
| 31 | Performance | Incremental, hash-based file parsing | Unchanged files/chunks are skipped; queue streams large files and excludes generated paths by default. | Planned |
| 32 | Performance | Profile-aware bounded concurrency | Interactive work preempts background work; worker limits adapt to profile, memory pressure, and task latency. | Planned |
| 33 | Performance | Deferred startup and virtualised HUD | Gateway and chat become interactive before nonessential scans; large event/task views are incrementally rendered. | Planned |
| 34 | Performance | Measurement-gated tuning | Benchmarks persist cold/warm start, first-token, completion, throughput, RAM, disk, cache, parser, and UI metrics; tune recommendations require evidence. | Planned |

## Release Rules

A release must pass source lint/static checks, unit and integration tests, dashboard smoke tests, launch readiness validation, model-script dry-run validation, manifest verification, capability-denial tests, and package integrity checks. Android release output additionally requires a reproducible debug build, Android manifest inspection, a user-controlled signing decision for release distribution, and device/emulator validation in an approved environment.

The Android companion remains a thin paired command center. Any host or device action must stay a local proposal with a reason, target, and explicit owner approval. This rule applies even after the application has been granted Android permissions.
