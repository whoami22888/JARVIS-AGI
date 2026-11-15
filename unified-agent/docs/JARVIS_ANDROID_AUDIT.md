# Jarvis Android: build audit and integration blockers

Source reviewed: JARVIS-AGI main at `8893089eb7c49134caf929db7369c5869c1fa358`, plus the sibling repos `JARVIS`, `deepseek4free`, `Uncensored-Local-AI-Multiplatform` and `Pockr` on 2026-11-16.

## Verified and corrected in this change

1. Both bootstrap scripts previously removed the entire target directory with `rm -rf`, including any local configuration, runtime data and build output. They now perform a non-destructive synchronization with a complete conflict preflight. The new regression tests cover first generation, preservation and refusal on divergent files.
2. The maintained Android templates were behind generated `MainActivity.kt` and `GatewayClient.kt`. They now use the exact generated blob versions. CI checks every maintained template against its corresponding generated file.
3. A GitHub Actions workflow now builds the debug Android APK and executes backend and Android unit tests. Passing CI is required before claiming that an APK is buildable.

## Remaining blockers, ordered by dependency

1. **Personal-phone enrollment:** `ManagedGatewayConfig.isComplete` accepts only device-owner managed HTTPS configuration. A normal sideload on an unmanaged phone has no way to enter the gateway URL and pairing code. Add a user-approved personal enrollment flow while preserving managed enrollment.
2. **Pockr gateway connectivity:** Pockr's `VmManager.buildQemuCommand` forwards only TCP 7080 to its guest API. A Jarvis gateway on 8080 is unreachable from an Android app until an explicitly scoped forwarding mechanism is implemented and tested. The Jarvis Android client also currently rejects cleartext HTTP; use valid HTTPS or a reviewed, loopback-only allowance.
3. **Local model endpoint routing:** `LOCAL_OLLAMA_BASE_URL=http://127.0.0.1:11434/v1` refers to the gateway container itself when Docker is used. Point it to the correct accessible local OpenAI-compatible inference endpoint. Do not conflate an enabled config flag with an actual model-readiness check.
4. **Tool execution:** Existing tool, skill and MCP registry definitions are not equivalent to a complete autonomous agent tool loop. Implement explicit tool-call dispatch with consent, logging and bounded retries; test end-to-end.
5. **Provider reliability:** The `deepseek4free` repository uses browser cookies, Cloudflare workarounds and an external service. Do not make the free offline product depend on that path or claim unlimited provider capacity.
6. **APK/model sizing:** Keep GGUF/voice-model weights outside the APK and download or import them into integrity-checked app-private storage when requested. Audit Pockr's committed duplicate binaries and base QCOW2 archives before deleting or deduplicating anything. Enable release R8 shrinking only with signed release-build and functional regression evidence.
7. **Physical device validation:** Install the produced APK on the target ARM64 phone; test pairing, background execution, cold-start, memory pressure, offline inference, screen-off recovery and network boundaries. No such device checks were performed as part of this commit.

## Free-first routing design

Android client -> authenticated Jarvis gateway -> owner-selected OpenAI-compatible on-device model endpoint. Cloud fallback remains optional and disabled by default. A standalone native model service (e.g. llama.cpp/Uncensored Local AI's HTTP API) avoids running a large model inside Pockr's QEMU guest. Persist conversations and tasks using the existing SQLite storage. Persist downloaded models in a separate app-private cache that can be integrity checked and reclaimed independently of the APK.

## Regression verification

Locally executed against the new isolated bootstrap scripts: `bash -n` and `python -m unittest discover`, three tests passed. Full repository integration, Android SDK build and physical device acceptance await the workflow and device evidence.
