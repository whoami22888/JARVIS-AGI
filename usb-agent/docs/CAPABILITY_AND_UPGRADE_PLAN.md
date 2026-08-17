# USB Agent Capability and Upgrade Plan

## Current Capability Confirmation

The fresh USB design is a **portable, local-first AI environment**. It is independent of the uploaded APK. The system is designed to boot on compatible x64 PCs into a persistent live environment and also to operate from Windows without booting. It includes a local FastAPI gateway, static command-center dashboard, adaptive host profiler, local model registry, SQLite task memory, model-download workflow, benchmarking workflow, Windows launcher, user-approved elevated diagnostics helper, and source assets for an Android command-center companion.

The currently implemented gateway can measure host capacity, choose a conservative hardware profile, retain task and event history in SQLite, select a local-first route, expose model status, start asynchronous tasks, publish task events to the dashboard, record metrics, and present approval proposals separately from normal work. A deterministic `system status` path works without a model installed. Local model tasks activate only after their model tag is explicitly installed. Optional cloud routes remain disabled until the owner configures credentials locally.

| Area | Present in the fresh build | Scope and boundary |
|---|---|---|
| Local agent gateway | Yes | FastAPI service, task queue, model router, health/status APIs, WebSocket events, task memory, and metrics. |
| Command-center HUD | Yes | Browser dashboard for host profile, model availability, live task events, task history, and advanced-action proposals. |
| Local model routing | Yes | Registry covers WhiteRabbit Neo, Qwen Coder, DeepSeek R1 local, and Llama; only installed tags are selected. |
| Model acquisition | Yes | Explicit, space-aware profile script for starter, balanced, and expanded bundles. |
| Portable data | Yes | USB-rooted configuration, SQLite state, logs, model registry, checksums, and benchmarks. |
| Windows operation | Yes | Portable PowerShell launcher and separate UAC-approved diagnostics helper; a full embedded Windows runtime remains a packaging task. |
| Android companion | Source scaffold | Compose companion supports HTTPS pairing, encrypted token storage, status retrieval, and task dispatch once built and paired. |
| Bootable image | Reproducible build configuration | Debian live-build configuration includes the runtime and first-session launcher; final ISO verification remains a release gate. |
| Automatic execution | No | The package deliberately uses a visible user-approved launch flow rather than silent USB auto-execution. |

> **Important design decision:** the USB is the portable source of truth, but it should not force all temporary work onto flash storage. The runtime uses local host storage only as an optional, removable cache tier and keeps the canonical state on the USB.

## Five Recommended New Features, Skills, Tools, or Integrations

| Priority | Feature | User value | Implementation approach |
|---:|---|---|---|
| 1 | **Offline knowledge vault** | Makes manuals, code, notes, PDFs, logs, and prior task outputs searchable without cloud dependency. | Add a document-ingestion worker with file-type extraction, content hashes, chunk metadata, SQLite FTS5, and an optional local embedding index. Index only owner-selected folders. |
| 2 | **Skill manifest and health registry** | Makes tools discoverable, versioned, permission-scoped, and easier to troubleshoot. | Define `skills/*.json` manifests containing inputs, outputs, required permissions, latency budget, version, integrity hash, and a self-test command. Display readiness in the HUD. |
| 3 | **Local network workspace connector** | Lets the agent work with a user-approved local NAS, SMB share, or development folder without copying everything to the USB. | Add a mount wizard with read-only as default, a folder allowlist, path normalization, file-operation audit log, and a clear disconnect control. |
| 4 | **Code workspace assistant** | Provides fast local code search, file change review, test execution, and patch previews. | Add an indexed workspace tool, language-aware chunking, a diff-preview panel, and explicit “apply change” confirmation. Run tests in a constrained project process. |
| 5 | **Model evaluation and routing lab** | Shows which model is fastest and most useful on each host instead of relying on assumptions. | Store benchmark prompt suites, first-token latency, completion latency, memory use, failure rate, and quality ratings. Use the results to tune per-host route rules. |

## Five Security Optimisations

| Priority | Optimisation | Why it matters | Implementation approach |
|---:|---|---|---|
| 1 | **Encrypted persistent-data partition** | Protects task memory, configurations, model metadata, and exports if the USB is lost. | Use a separately encrypted data volume with a user-supplied passphrase at boot; keep only the bootable runtime unencrypted if needed. |
| 2 | **Per-session local gateway token** | Prevents a nearby device on the same network from using the dashboard or mobile API without pairing. | Generate a random token at launch, bind the gateway to loopback by default, require an explicit pairing code for LAN/mobile use, and expire tokens on shutdown. |
| 3 | **Signed release manifest and verification screen** | Detects modified runtime files, model manifests, and skill bundles before use. | Generate SHA-256 hashes during packaging; verify them at launch; show a red HUD warning and disable affected skill launches on mismatch. |
| 4 | **Permission-scoped tool runner** | Keeps normal AI requests from receiving direct file, network, elevated, or device capabilities automatically. | Define capability tokens such as `read_files`, `write_files`, `network_read`, and `elevated_diagnostics`; request user approval per scope and record the decision. |
| 5 | **Separate elevated helper with audit trail** | Keeps administrator access out of the primary agent process. | Retain a narrowly scoped UAC helper, accept only allowlisted action manifests, create a timestamped JSON result, and terminate the helper immediately. |

## Five Usability Upgrades

| Priority | Upgrade | Implementation approach |
|---:|---|---|
| 1 | **First-run setup wizard** | Detect USB storage capacity, host hardware, free space, supported acceleration, and available models; propose Starter, Balanced, or Expanded profile with a clear download estimate. |
| 2 | **One-click recovery and diagnostics bundle** | Add “Create support bundle” to export redacted logs, host profile, model status, benchmark summaries, and checksums without exporting prompts, tokens, or private files by default. |
| 3 | **Task templates** | Provide structured templates for code review, document Q&A, local file search, performance benchmark, system status, and model comparison. Each template declares model, tools, and expected output. |
| 4 | **Offline state indicator** | Show whether the active task is entirely local, uses an optional remote provider, or is waiting on network access. Include an “offline-only” hard switch. |
| 5 | **Safe storage dashboard** | Display USB free space, host-cache free space, model footprint, index footprint, SQLite size, and a cleanup recommendation before space becomes critical. |

## Five HUD and Visual Dashboard Upgrades

| Priority | Upgrade | Implementation approach |
|---:|---|---|
| 1 | **Real-time performance ribbon** | Add CPU, RAM, disk queue, USB throughput, active model, first-token latency, and task latency. Use small sparklines and retain a 60-second window. |
| 2 | **Model topology map** | Visualise installed models, loaded/unloaded state, RAM allocation, route purpose, local/remote status, and current fallback chain. |
| 3 | **Task timeline with checkpoints** | Show queued, model-selected, retrieval, tool-call, first-token, completion, and approval stages. This makes bottlenecks visible rather than hidden. |
| 4 | **Capability and permission panel** | Display active skills, requested permissions, current token scope, enabled local folders, network status, and any elevated helper state. |
| 5 | **Storage and cache heatmap** | Show USB versus host cache use, model read rate, index size, file-parser queue, cache hit rate, and cleanup opportunities. |

## Five Automation Upgrades

| Priority | Automation | Implementation approach |
|---:|---|---|
| 1 | **Model availability and integrity check at launch** | Verify the model registry, free-space reserve, runtime availability, and hashes before models are selected. Do not automatically download missing multi-gigabyte models. |
| 2 | **Idle-time indexing queue** | Index only allowlisted files while the computer is idle, pause under memory or thermal pressure, and resume from a persisted cursor. |
| 3 | **Adaptive benchmark scheduler** | Run a brief host benchmark after first setup, after model updates, and after significant hardware changes; never run while interactive tasks are active. |
| 4 | **Versioned update staging** | Download updates into a staging folder, verify hashes, run smoke tests, then switch the active release with a rollback record. |
| 5 | **Task follow-up reminders** | Create local notifications for incomplete downloads, failed benchmarks, low storage, expired pairing tokens, and pending user approvals. Notifications should never execute an action without confirmation. |

## Low-Latency and Fast File-Parsing Plan

### 1. Use a Three-Tier Working-Set Design

“Persistent RAM” should be treated carefully: RAM itself is not persistent across a shutdown. The fast design is a **three-tier working set** rather than pretending flash is RAM.

| Tier | Medium | What belongs there | Rule |
|---|---|---|---|
| Hot | System RAM | Active model KV cache, active task context, UI state, small parser buffers. | Keep this bounded by the active host profile. |
| Warm | Compressed RAM (`zram`) | Reclaimable inactive pages and short-lived overflow. | Enable only after measuring RAM pressure; use a conservative cap such as 20–35% of physical RAM. |
| Cold, persistent | Encrypted USB data store or an owner-approved host SSD cache | Model artifacts, document index, SQLite data, parser cache, benchmarks, logs, and checkpoints. | Treat it as durable cache/storage, never as latency-critical virtual memory. |

A persistent swap file on the USB is **not** recommended for interactive model inference. It can cause stalls and unnecessary flash wear. If a host has an internal SSD and the owner approves it, a temporary encrypted swap or cache there can be used as a last-resort cold tier; it should be deleted on exit or encrypted under a per-session key.

### 2. Keep Models Off the Hot Path

Load one primary model at a time. Keep the router and small status model available, but avoid loading WhiteRabbit, DeepSeek R1, Qwen Coder, and Llama simultaneously on a low-memory host. Use memory-mapped model weights where supported, persist model metadata separately, warm only the model selected for the next likely task, and unload inactive models after a measured idle timeout.

### 3. Make File Parsing Incremental

The file parser should use a queue with file-size limits, MIME-type routing, and content hashing. A file that has not changed should never be fully re-parsed. Large text should be streamed and chunked; PDFs should extract page-by-page; source trees should skip generated folders such as `node_modules`, virtual environments, build output, and binary assets unless explicitly requested. Store a tuple of path, size, modification time, and content hash in SQLite, then process only changed chunks.

### 4. Use Host-Profile-Aware Concurrency

Interactive chat has priority. On low-memory hosts, run one parser worker and one model request. On stronger hosts, allow independent read-only parsing and indexing workers, but keep the model runtime single-threaded at the request scheduler level unless the selected backend benefits from additional concurrency. Pause background jobs when first-token latency, memory pressure, or USB queue depth crosses a configured threshold.

### 5. Reduce Startup and UI Cost

Start the gateway first and return a ready dashboard immediately. Defer model inventory refresh, document indexing, and benchmark scans until after the HUD is interactive. Use WebSocket events rather than repeated full-status polling. Keep dashboard history virtualised, truncate very large event payloads in the initial view, and load detailed task traces only on demand.

### 6. Measure Before Tuning

Add these measurements to every benchmark record: cold start, warm start, first-token latency, completion latency, tokens per second, peak RAM, disk-read throughput, USB write latency, model load time, parser throughput, cache hit ratio, and UI frame delay. Apply a change only when the before/after measurement improves the intended metric without causing unacceptable memory or stability regressions.

## Recommended Implementation Order

| Phase | Outcome |
|---|---|
| 1 | Finish release verification: boot-image validation, gateway smoke test, model script validation, and Android source build once the Android SDK license is owner-approved. |
| 2 | Add encrypted persistent storage, session tokens, signed manifest verification, and the permission-scoped tool runner. |
| 3 | Add the performance ribbon, task timeline, storage heatmap, and benchmark-result persistence. |
| 4 | Add offline knowledge vault, incremental file parser, code workspace assistant, and skill manifest registry. |
| 5 | Add setup wizard, staged updates, idle-time indexing, and Android pairing UX refinements. |

The highest-impact initial combination is: **encrypted persistent storage + session pairing + model routing lab + performance HUD + incremental file parsing**. It directly improves practical reliability, speed visibility, local use, and recovery without overloading the USB or requiring all models to remain in memory.
