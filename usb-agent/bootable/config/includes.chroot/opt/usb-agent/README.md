# USB Agent

USB Agent is a **fresh, portable, local-first agent runtime**. It is separate from the uploaded APK and combines adaptive host profiling, local model routing, optional official cloud providers, a browser HUD, SQLite task memory, benchmark tools, a Windows launcher, and a bootable-live build configuration.

## Quick Start on Linux

Copy `config/.env.example` to `.env`, adjust the paths for the USB mount if needed, then start the gateway.

```bash
cp config/.env.example .env
chmod +x scripts/*.sh
./scripts/launch.sh
```

Open `http://127.0.0.1:8787`. Before model use, run the explicit model installer and choose a profile.

```bash
./scripts/pull_models.sh starter
./scripts/pull_models.sh balanced
```

The installer checks the configured storage reserve and asks for confirmation. It is designed to pull official runtime tags only; it does not extract credentials or reuse an application session.

## Adaptive Runtime

The host profiler classifies available hardware and stores a local profile. Run it directly when preparing a host:

```bash
python3 scripts/profile_host.py
./scripts/benchmark.sh
```

The gateway is usable without a model installed: requests such as `system status` execute deterministically and provide live host metrics. Local model requests become available once their selected local runtime tag is installed. Optional DeepSeek API access requires an owner-provided `DEEPSEEK_API_KEY` in the local untracked `.env` file.

## Windows Portable Mode

The USB package expects an embedded Python runtime under `runtime/python/python.exe`. Once supplied by the USB preparation build, launch it with:

```powershell
powershell -ExecutionPolicy Bypass -File .\windows\Start-AIAgent.ps1
```

The normal launcher does not request administrator rights. The separate `Invoke-ElevatedAction.ps1` helper invokes the Windows UAC dialog and currently provides an auditable diagnostics action only.

## Bootable Mode

The `bootable/` directory contains the reproducible live-image configuration and first-boot scripts. Booting the image starts the local agent runtime only after the user reaches the desktop/session. It does not attempt automatic removable-media execution.

## Testing

```bash
python3 tests/smoke_test.py
```

The smoke test covers host profiling, route selection, SQLite task persistence, deterministic task execution, and approval state changes.

## Project Layout

| Path | Purpose |
|---|---|
| `backend/` | FastAPI gateway, adaptive router, provider adapters, persistent memory. |
| `frontend/` | Static command-center HUD served by the gateway. |
| `models/` | Registry and local runtime model store. |
| `scripts/` | Launch, profile, benchmark, model acquisition, and package validation utilities. |
| `windows/` | Portable launcher and optional elevation helper. |
| `android/` | Android companion source and build assets. |
| `bootable/` | Live system build configuration and boot-time setup. |
| `data/` | Untracked portable runtime state. |

Read [`docs/FRESH_BOOTABLE_SPEC.md`](docs/FRESH_BOOTABLE_SPEC.md) for the complete fresh-build architecture and model bundle rationale.
