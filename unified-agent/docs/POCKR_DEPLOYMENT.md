# Jarvis Gateway inside Pockr (Android ARM64)

This optional deployment runs the existing `unified-agent/backend` FastAPI
gateway in a **Docker container inside Pockr's Alpine VM**, without changing
the native Android Jarvis app. It requires the Pockr port-forward integration
from [Pockr PR #1](https://github.com/whoami22888/Pockr/pull/1).

## Prepare

1. Install a Pockr APK whose complete Alpine base disk and QEMU native
   libraries have passed packaging checks. Start its VM and Docker service.
2. Obtain the approved source for `JARVIS-AGI/unified-agent/backend` inside
   Pockr's VM, for example from the GitHub source archive. Avoid unverified
   third-party images; use the revision that passed CI.
3. In the backend directory, copy `.env.example` to `.env` and generate
   two independent, random secrets of at least 24 characters:
   `PAIRING_CODE` and `TOKEN_SECRET`. Never commit `.env`.
4. Ensure `DATA_DIR=/srv/data`; set `LOCAL_OLLAMA_ENABLED` to `false`
   until a real free local model endpoint is reachable from the VM.
   Keep `CLOUD_FALLBACK_ENABLED=false` for offline-only operation.
5. From the backend directory execute `bash run-in-pockr.sh`. The script
   builds a local ARM64 image and launches a non-root, read-only container
   with a persistent Docker volume, bounded memory and logs.

The script never replaces an existing container or deletes its volume.
Existing configuration updates require a deliberate stop/recreate process.

## Pair

Pockr needs to forward its guest port `8080` to Android
`127.0.0.1:8080` (and `7080` for Pockr's own management API).
Open the Jarvis Android app, enter `http://127.0.0.1:8080`, and the
`PAIRING_CODE` from your private server environment. Tap **Save personal
gateway**, then **Trust & connect**. Remote unencrypted HTTP is not
supported.

## Use a free model

The gateway's existing local-first OpenAI-compatible provider supports a
self-hosted local model, such as llama.cpp or a compatible Android inference
server. A potential Android service is
`Uncensored-Local-AI-Multiplatform`, documented at
`http://127.0.0.1:4891/v1` on the **Android host** with model `local`.

Do **not** paste that localhost address into the guest's backend config:
inside Pockr's Docker container, `127.0.0.1` means the guest/container,
not Android. QEMU's SLIRP guest-to-host address `10.0.2.2` is a possible
route, but the host model service may listen only on loopback. Verify
host reachability, network exposure, model API compatibility, device
performance and security *before* enabling it.

No external provider account is required for the local-only gateway, but
model weights must be downloaded or imported lawfully, and model RAM needs
must fit the Android device.

## Verification

The CI ARM64 container test runs outside Pockr and proves only gateway
health, SQLite persistence, device pairing and task API lifecycle.
Physical phone acceptance still requires a complete Pockr VM boot, port
binding inspection, local-model interaction, restart persistence, RAM,
battery and thermal measurements.
