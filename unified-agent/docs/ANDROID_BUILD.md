# Android Build & Connector Framework

This document describes the production Android pipeline implemented in
`templates/android/` (copied to `android/` by `bootstrap_android.sh`).

## Flavors

| Flavor | On-device hotword | Intended use |
|---|---|---|
| `minimal` | No (stub engine) | Small APK, no Picovoice access key required |
| `full` | Yes (Porcupine "Jarvis") | Full feature set, requires a Picovoice Console access key |

```bash
cd android
./gradlew :app:assembleMinimalDebug
./gradlew :app:assembleFullDebug
./gradlew :app:testMinimalDebugUnitTest
```

## Signing

Release builds require four Gradle properties (or a `keystore.properties` loaded by
your CI); the build fails instead of silently falling back to the debug key.
See `keystore.properties.example`:

```properties
RELEASE_STORE_FILE=/secure/path/release.keystore
RELEASE_STORE_PASSWORD=...
RELEASE_KEY_ALIAS=...
RELEASE_KEY_PASSWORD=...
```

## Connector framework (`connectors/`)

- `ServiceConnector` — one interface for every service: OAuth2, API key, session
  cookie, device code, biometric, and no-auth (local) connectors.
- `ConnectorCatalog` — the predefined registry of connectors (AI providers,
  Google Drive/Calendar/Gmail, Microsoft Graph, GitHub, Notion, Dropbox,
  Telegram, WhatsApp Business, Discord, Slack, Spotify, Home Assistant,
  Local Ollama).
- `ConnectorTokenVault` + `SecretBox` — AES/GCM keys generated per connector in
  the Android Keystore; tokens and OAuth client secrets are stored encrypted.
- Room database (`db/`) — `ConnectorEntity` (per-connector config and client
  credentials), `TokenEntity` (encrypted token mirror), `AccountEntity`
  (account email/display name).
- `ConnectorViewModel` + Accounts tab in `MainActivity` — manage API keys,
  configure OAuth2 clients, authorize in Custom Tabs, reveal stored secrets
  behind a biometric prompt, revoke, enable/disable for agent tasks.

### OAuth2 flow

1. Save the OAuth2 client ID (and optional secret/redirect URI) in the Accounts tab.
2. Blank redirect URI uses a local loopback server (`http://127.0.0.1:48731/oauth`).
3. Authorization uses PKCE (S256). The redirect returns through
   `MainActivity.onNewIntent` (custom scheme `wrn.neo://oauth/<connector>`) or
   the loopback server, then the code is exchanged at the provider token endpoint.

## Voice & vision

- **STT**: Vosk (`com.alphacephei:vosk-android`). The small English model
  (~40 MB) downloads on first use from alphacephei.com and unpacks into
  `filesDir/models/`. Streaming partial/final transcripts.
- **TTS**: Android `TextToSpeech` (offline, free) with a speak-answer button.
- **Vision**: CameraX + ML Kit on-device object detection with classification
  labels over a live preview.
- **Hotword**: `full` flavor runs Porcupine in a foreground service
  (`HotwordService`, microphone type). Configure the access key through the
  managed restriction `porcupine_access_key`.

## Browser automation

- `WhiteRabbitAccessibilityService` reads the active window tree (URL bar,
  visible text, nodes by id/description) and performs gestures only after the
  local approval gate (`ApprovalGate`) is granted in-app. Reads never leave the
  device; mutating actions require approval.

## Device admin & storage

- `DeviceAdminReceiver` + `DeviceAdminManager` — lock screen, camera disable,
  password policy, and explicit wipe (all gated behind owner confirmation).
- `StorageAccessManager` — all-files access request, SAF directory picker,
  battery optimization exemption for background services.

## CI

`.github/workflows/android.yml` bootstraps the project, runs JVM unit tests, and
builds both flavors. Release signing secrets are provided via repository
secrets (`RELEASE_STORE_FILE` etc.).