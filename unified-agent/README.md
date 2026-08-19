# WhiteRabbit Neo Unified Agent for Android

This project builds a single Android command-center application with a **WhiteRabbit Neo** identity and an **authorized DeepSeek R1-compatible** reasoning backend. The Android surface combines the dashboard/HUD, capability selection, persistent local memory, tool builder, skill builder, MCP registry, and approval-centric automation proposal flow in one interface.

> The supplied `DeepSeek-mod.apk` was preserved as a static installation-reference artifact only. This project does not reuse its protected code, credentials, signing key, package identity, cloud-access modifications, or access-control bypasses.

## Included components

| Component | Purpose |
|---|---|
| `android/` | Generated Kotlin/Jetpack Compose Android client. It offers the unified HUD, HTTPS pairing, encrypted gateway token storage, capability toggles, registry forms, and local memory. |
| `backend/` | Generated FastAPI gateway. It pairs devices, stores tasks/events and registry items in SQLite, routes authorized prompts to a DeepSeek-compatible endpoint, and rejects unsafe registry definitions. |
| `templates/` | Maintained source templates for both generated projects. Edit these files, then run the bootstrap scripts. |
| `bootstrap_android.sh` | Recreates `android/` from `templates/android/`. |
| `bootstrap_backend.sh` | Recreates `backend/` from `templates/backend/`. |
| `docs/` | Architecture and security documentation. |

## Agent capabilities

| Capability | Status in this build | Control |
|---|---|---|
| DeepSeek R1 reasoning | Implemented through a configurable OpenAI-compatible gateway adapter | Requires an owner-provided authorized provider key at the gateway. |
| WhiteRabbit Neo HUD/persona | Implemented | Runs in the Android client. |
| Model/tool router | Implemented baseline | Gateway records selected model and explicit enabled-capability set. |
| Persistent memory | Implemented | Memory notes remain on the Android device until deleted. |
| Web tools | Declarative registry support | Public HTTPS targets only; private and cleartext targets are rejected. |
| API tools | Declarative registry support | Owner-controlled gateway configuration and user approval are required before use. |
| Automation | Proposal flow implemented | Every action must be proposed and locally approved; no silent device control. |
| Tool and skill builder | Implemented registry forms | Tools and skills are defined as metadata, not arbitrary executable uploads. |
| MCP registry | Implemented metadata registry | HTTPS MCP endpoint metadata can be registered; credentials remain server-side. |

## Generate the project

Run the bootstrap scripts from this directory whenever the maintained templates change.

```bash
./bootstrap_backend.sh
./bootstrap_android.sh
```

## Configure the authorized backend

Copy the example environment file, generate long random values for pairing and token secrets, and supply your own authorized DeepSeek-compatible credentials. Do not put the provider key in the Android client.

```bash
cd backend
cp .env.example .env
# Edit .env: PAIRING_CODE, TOKEN_SECRET, DEEPSEEK_API_KEY, DEEPSEEK_BASE_URL, DEEPSEEK_MODEL
```

The mobile client intentionally rejects `http://` gateway addresses. Place the FastAPI service behind HTTPS before pairing a physical Android device. A `Caddyfile.example` is included for the TLS reverse-proxy boundary.

For local gateway development only, run:

```bash
uvicorn app.main:app --host 127.0.0.1 --port 8080
```

For container deployment:

```bash
docker compose up --build
```

## Build the Android client

The generated project uses Java 17 bytecode targets, Android API 35, and Gradle 8.9. Point `local.properties` at an Android SDK with `platforms;android-35` and `build-tools;35.0.0`, then run:

```bash
cd android
./gradlew --no-daemon :app:assembleDebug
```

The debug APK is produced at:

```text
android/app/build/outputs/apk/debug/app-debug.apk
```

A debug APK is suitable only for test installation. Release distribution requires an owner-controlled keystore and a release signing configuration. A rebuilt application has a different signing certificate than the reference package and therefore cannot update it in place.

## Android pairing flow

1. Deploy the gateway behind a publicly reachable HTTPS hostname you control.
2. Start the Android app and enter the HTTPS gateway URL.
3. Enter the one-time pairing code configured at the gateway.
4. The client stores the expiring device token using the Android keystore-backed vault when available.
5. Submit a prompt through the Command destination. The gateway will route only through the configured authorized endpoint.

## Validation commands

```bash
bash -n bootstrap_backend.sh && bash -n bootstrap_android.sh
python3 -m py_compile backend/app/*.py
PYTHONPATH=backend python3 backend/tests/provider_response_validation_test.py
PYTHONPATH=backend python3 backend/tests/smoke_test.py
cd android && ./gradlew --no-daemon :app:assembleDebug
```

## Security expectations

The agent has no embedded DeepSeek key, no unrestricted local-network browsing, no silent installs, no credential capture, and no hidden automation execution. The gateway will return a configuration error until its owner provides an authorized provider endpoint and key. Review `docs/ANDROID_SECURITY.md` and `docs/BACKEND_SECURITY.md` before deployment.
