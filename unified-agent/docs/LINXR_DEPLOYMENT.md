# Jarvis Gateway inside LinxR (Android ARM64, experimental)

LinxR replaces the retired Pockr integration. This plan is based on the
user-supplied `linxr-release.apk` (SHA-256
`e4f61f725950870293fc0b2b0f72ce36a63296210b92d5975e1f70fb00834957`).
The package `com.ai2th.linxr` contains an ARM64 QEMU engine, Alpine Linux,
a Python guest management API and a Podman/crun FastTrack mode.
Its editable source and phone-side integration tests are unavailable.

## Security blocker: fixed SSH password

The included `assets/bootstrap/init_bootstrap.sh` sets a root SSH password
of `alpine`, permits root login and enables password authentication.
Do not expose the guest SSH port to a local network or the Internet.
Replace this fixed credential and configure key-based access before allowing
external SSH. Review the broad Android storage permission requested.

## Deploy with Podman

1. Boot LinxR's Alpine guest and verify `uname -m`, `podman info`,
   `df -h /`, and `df -i /`. Podman must be available and the guest
   must have enough space to build the Python gateway.
2. Obtain the owner-approved `JARVIS-AGI/unified-agent/backend` source.
   Copy `.env.example` to `.env` and set independently generated
   `PAIRING_CODE` and `TOKEN_SECRET` (at least 24 characters each).
   Set `DATA_DIR=/srv/data`, `LOCAL_OLLAMA_ENABLED=false`,
   `CLOUD_FALLBACK_ENABLED=false` until a real local model is reachable.
3. Run `bash run-in-linxr.sh`. It requires native ARM64, Podman, valid
   secrets, and a persistent volume. It does not overwrite an existing
   Jarvis container or fall back to the retired Docker deployment.
4. Inside the guest check `podman ps -a` and
   `http://127.0.0.1:8080/health`; verify pairing and SQLite persistence
   across a VM restart.

## Android port-forwarding: blocker

The Jarvis Android client connects to `http://127.0.0.1:8080` on the
**phone**; guest `127.0.0.1:8080` is different. The uploaded APK has
not been proven to forward guest TCP 8080 into Android. Do not claim
connectivity or AI function until verified on the actual handset.

The Podman runner tests use mocks only. They do not establish runtime,
battery, thermal, port-forwarding or container compatibility on a phone.
The uploaded APK is not committed into the Jarvis source repository.
The separate historical Pockr GitHub repository is not deleted here.
