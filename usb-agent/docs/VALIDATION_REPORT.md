# USB Agent Upgrade Validation Report

## Automated Validation Status

The upgraded portable gateway passed its saved source, feature, performance, JavaScript, live API, Android packaging, alignment, signature, and repository hygiene checks on the build host.

| Validation | Result | Evidence |
|---|---|---|
| Core deterministic gateway flow | Pass | `tests/smoke_test.py` completed. |
| Session, capability, skill, integrity, and knowledge-vault primitives | Pass | `tests/test_features.py` completed. |
| Working-set policy, incremental queue, storage telemetry, and benchmark store | Pass | `tests/test_performance.py` completed. |
| Python static compilation | Pass | `python3 -m py_compile backend/*.py`. |
| Dashboard syntax | Pass | `node --check frontend/app.js`. |
| Live authenticated API | Pass | Health, pairing, expiring bearer session, authenticated status, and deterministic task submission completed against a temporary gateway instance. |
| Release manifest | Pass | A 21-file SHA-256 release manifest was generated and surfaced by the gateway. |
| Source hygiene | Pass | `git diff --check` and a targeted credential-pattern scan completed without tracked secrets. |
| Android debug build | Pass | Gradle `:app:assembleDebug` completed with Android SDK 36 and Java 17. |
| Android lint | Pass | `:app:lintDebug` completed. |
| Android package metadata | Pass | `ai.usbagent.commander`, min SDK 31, target SDK 36, label `USB Agent Commander`. |
| Android alignment/signature | Pass | `zipalign -c -p 4` and `apksigner verify --verbose` completed. |

## Android Signing Note

The emitted package is a **debug APK** signed by the automatically generated Android Debug certificate. It is suitable for owner-controlled testing but is not production-signed. A future production release must use an owner-controlled release keystore; a replacement signed with a different key cannot update an application signed by another key.

## Remaining Owner-Environment Validation

The following checks cannot be truthfully completed in the build sandbox because they require the owner’s physical devices or chosen secrets: boot the rebuilt ISO on a compatible x64 PC; create and unlock the encrypted persistent data store; install and exercise the debug APK on the owner’s Android 16 device; pair it with a trusted HTTPS gateway; configure and benchmark local model weights; and review any host-capability grant before using file, network, test-runner, or elevated diagnostics features.

> All host integrations remain capability-scoped and user-approved. A granted capability is limited to its declared target and expiry; it is not general unattended control of the host.
