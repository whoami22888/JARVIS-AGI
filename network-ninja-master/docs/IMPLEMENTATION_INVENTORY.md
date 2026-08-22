# Network Ninja Master — Implementation Inventory

**Status:** `PARTIAL`
**Inspection date:** 2026-08-23 (GMT+10)
**Repository baseline:** `837c98e` — `Add secure local gateway discovery`

## Scope of this inspection

The selected repository contains two substantially different codebases. The original top-level **JARVIS-AGI** program is a Python voice-assistant prototype. Its entry point is an unbounded interactive loop that performs speech-driven AI routing and host-setting actions; it does not have a service architecture, authorization system, reproducible build, or automated test suite suitable for this product. It is therefore **not reused** for Network Ninja Master.

The `unified-agent/` subtree is a separate Android and FastAPI gateway project. It supplies useful patterns for device pairing, local SQLite persistence, provider routing, and declarative registry validation. It is not a security-agent runtime: its current task endpoint performs prompt completion only, while automation remains a proposal object. It can inform this build’s design, but the new portable engine is implemented independently in Go so it can operate consistently from a Linux live environment.

| Existing component | Observed state | Reuse decision | Reason |
|---|---|---|---|
| Top-level Python voice assistant | Prototype; broad imports, third-party AI routes, host-setting paths, no structured policy engine | **REPLACE** | Does not meet the requirement for scoped, evidence-backed, portable security operations. |
| Top-level `requirements.txt` | Small Python dependency list for audio, speech, web, and vision features | **DO NOT BUNDLE** | Not required for a network/device recovery platform and lacks production pinning and security provenance. |
| `unified-agent/backend` | FastAPI prompt gateway with pairing, HMAC device tokens, SQLite task/event records, provider fallback | **REFERENCE ONLY** | Useful security concepts, but no host-agent, tool runner, scope model, device engine, or ISO support. |
| `unified-agent/templates/android` | Kotlin command-center client with encrypted token storage | **NOT IN FIRST RELEASE** | A separate Android client is out of scope for a bootable x86_64 USB first release. |
| Existing documentation/tests | Good local documentation and small backend test set | **REFERENCE ONLY** | Tests cover gateway routing and registry validation, not the requested USB, device, network, or recovery requirements. |

## Security findings in the reusable reference code

| Finding | Evidence | Disposition in Network Ninja Master |
|---|---|---|
| Development secrets have fallback defaults. | `unified-agent/backend/app/config.py` supplies default pairing and token values. | New implementation requires a scope document and does not embed reusable production secrets. |
| Authentication establishes a device identity but no operation-specific permission system. | `authenticated_device()` validates a token; the task route does not evaluate a target scope. | Replace with explicit scope parsing, operation allowlists, CIDR/host target checks, expiry, and rate controls. |
| Provider routing is text-only. | `provider.py` sends prompts to a model endpoint and returns text. | Replace with deterministic local tools; AI integration remains an optional planner interface only. |
| Registry validation accepts declarative definitions without integrity or lifecycle checks. | `policy.py` validates basic URL/transport constraints only. | New registry records binary path, hash, expected version, allowed arguments, risk, and health-check result. |
| SQLite schema lacks migration, WAL configuration, retention boundaries, and immutable evidence handling. | `db.py` creates a single database with four tables. | New storage separates state and append-only audit records, enables WAL, and stores command evidence with SHA-256 digests. |

## Requirements normalization

The supplied requirements have been normalized into an implementable first-release boundary:

| Requirement family | First-release implementation position |
|---|---|
| Ventoy deployment | Build a genuine bootable x86_64 Linux ISO, checksum, manifest, persistence configuration, and deployment guide. |
| USB persistence | Use a separate Ventoy persistence image reference and a configurable external data root; no application data is written into the immutable ISO. |
| Security assessment | Implement passive local interface inspection and **explicitly scoped** host reachability, TCP connect checks, TLS metadata collection, and HTTP header collection. |
| AI agent | Implement an evidence-preserving planner interface and deterministic operation registry. An actual model is optional and reported unavailable when absent. |
| Device tools | Implement read-only local USB and serial inventory. ADB/Fastboot, firmware, recovery, and flashing are declared unavailable in this initial artifact rather than simulated. |
| Dashboard | Provide a local loopback-only HTTP dashboard and JSON API. Every visible operation calls the same guarded backend implementation. |
| Cross-platform builds | Produce and test Linux x86_64 first. Other OS/architectures receive source build targets but are not release-verified without their native test environments. |
| Firmware/recovery | Out of first release; `BLOCKED` pending documented vendor integrations, physical devices, and explicit user authorization. |

## Source and build baseline

The implementation is created under `network-ninja-master/` on the `redforge-network-ninja` branch. No protected application code, signing key, credential, binary, or reference APK is copied from the prior project.

> A feature is treated as verified only after a real build, real execution, expected result, independent validation, and retained evidence. Source existence alone is not treated as proof of capability.

## Next implementation boundary

The initial release deliberately does **not** contain credential collection, bypass functionality, arbitrary remote command execution, exploitation payloads, persistence mechanisms, stealth controls, unbounded scans, firmware flashing, or automatic installation. Such behavior is neither necessary for the initial defensive toolkit nor compatible with an evidence-backed authorized-scope model.

The functional baseline is a portable local security workstation that can inspect its own host, create and enforce engagements, run low-impact authorized assessments against explicitly enumerated assets, record verifiable evidence, and package itself for Ventoy boot.
