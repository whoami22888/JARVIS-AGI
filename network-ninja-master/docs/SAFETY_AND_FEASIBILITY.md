# Safety and Feasibility Assessment

**Status:** `PARTIAL`
**Date:** 2026-08-23 (GMT+10)

## Authorized-operation model

Network Ninja Master is designed only for systems included in an explicit, time-bounded engagement definition. The product must not infer authorization from a user prompt, network proximity, or a claim of ownership. Each active network operation is checked against an operator identity, expiry time, target set, allowed ports, allowed operation categories, excluded targets, and rate limit before any connection is attempted.

| Operation class | First-release handling | Authorization requirement |
|---|---|---|
| Local host and USB inventory | Implemented | Local operator acknowledgement and audit event. |
| TCP connect assessment | Implemented | Target must exactly match an approved hostname/IP or belong to an approved CIDR; port must be allowed. |
| TLS metadata inspection | Implemented | Same scope requirement as TCP connect assessment. |
| HTTP response-header inspection | Implemented | Approved HTTPS/HTTP target and allowed port. Credentials and bodies are not collected. |
| Packet capture | Not implemented | Requires interface, storage, privacy, and privileged-operation design review. |
| UDP probing | Not implemented | Requires careful rate, response, and false-positive treatment. |
| Vulnerability exploitation or payload delivery | Not implemented | Requires dedicated lab/engagement verification and a narrow, non-destructive validator design. |
| Remote shell, credential assessment, lateral movement | Not implemented | Not part of the portable first release. |
| Firmware flashing or recovery write operations | Not implemented | Requires exact device profile, manufacturer documentation, physical device, and separate explicit confirmation. |

## Enforcement invariants

The command layer is constructed around the following invariants:

1. A network operation cannot execute without a valid `scope.json` object.
2. A scope object has a finite expiry; expired scopes are rejected.
3. A target outside the declared host or CIDR list is rejected prior to network access.
4. Ports and operation categories are independently allowlisted.
5. The scanner never shells out to user-provided strings. The initial operations use Go standard-library network APIs with timeouts.
6. Results have an explicit status: `OBSERVED`, `TESTED`, `VERIFIED`, `FAILED`, `BLOCKED`, or `UNKNOWN`. The system never upgrades inference to verification.
7. Audit events capture the request, scope identifier, decision, raw evidence digest, timestamps, and outcome. They do not intentionally capture secrets.
8. The loopback dashboard binds only to `127.0.0.1` unless a future authenticated network-exposure feature is separately implemented.

## Build feasibility

| Capability | Build environment finding | Result |
|---|---|---|
| Go compiler | Go 1.22.2 installed during the build | `AVAILABLE` |
| Debian live-image tooling | `live-build`, `debootstrap`, `xorriso`, GRUB, SquashFS tooling installed | `AVAILABLE` |
| Virtual boot test | QEMU x86_64 installed | `AVAILABLE` |
| Physical Ventoy USB validation | No removable Ventoy media is attached to this environment | `BLOCKED` |
| BIOS/UEFI hardware boot test | No physical host firmware access is exposed | `BLOCKED`; virtual ISO boot is attempted instead. |
| Android/ADB/Fastboot physical testing | No authorized device is attached | `BLOCKED` |
| Firmware signing/vendor workflow | No manufacturer authorization, images, or device profile supplied | `BLOCKED` |
| Windows/macOS native testing | No native Windows/macOS environment is available | `BLOCKED` |
| Local model inference | No model backend or model weights are provided | `OPTIONAL / UNAVAILABLE` unless configured by the operator. |

## Ventoy persistence position

Ventoy supports a persistence configuration in `/ventoy/ventoy.json` that maps an ISO path to one or more backend data-image paths. The backend image must be stored on Ventoy’s first partition and uses a distribution-specific label; Ventoy documents this as the supported persistence mechanism.[1] The release therefore supplies a template mapping and a script that validates the requested image path and persistence image before USB use. It does not claim that persistence is verified until the release ISO has booted from an actual Ventoy USB.

## Engineering decision

A complete production system across USB boot, Windows/macOS portability, real device protocols, firmware validation, local AI, and unrestricted security testing cannot be truthfully verified from this single Linux build environment. The correct release strategy is a functioning and testable **Linux x86_64 Ventoy candidate** with a strict scope engine and evidence-first low-impact diagnostics. Features dependent on real hardware, external credentials, vendor contracts, signed firmware, native OS environments, or a physical Ventoy drive remain `BLOCKED`, not simulated.

## References

[1]: https://www.ventoy.net/en/plugin_persistence.html "Ventoy Persistence Plugin"
