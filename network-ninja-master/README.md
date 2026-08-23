# Network Ninja Master

**Network Ninja Master** is an offline-first Linux x86_64 diagnostic workstation for **authorized** security assessment, device inventory, and evidence-backed reporting. It is designed to run as a portable binary or from a Ventoy-deployed live ISO. The first release deliberately concentrates on real, low-impact capabilities that can be implemented and tested without inventing device support or claiming unsafe operations are complete.

> **Authorization is mandatory.** Active network operations require a time-bounded scope file with explicit target, port, and operation allowlists. The runtime rejects requests outside that scope before opening a network connection.

## Implemented first-release capabilities

| Capability | Operational behavior | Evidence behavior |
|---|---|---|
| Host inventory | Reads hostname, OS, kernel, CPU count, memory total, and local network interfaces from the running system. | Returns real values only; unavailable values are omitted. |
| USB and serial inventory | Reads local USB descriptors from `/sys` and enumerates present serial device nodes. | Records only values exposed by the operating system. |
| Scope validation | Requires target list/CIDR, exclusions, allowed ports, allowed operations, start/expiry, and rate limit. | Produces explicit allow/deny rationale. |
| TCP diagnostic | Attempts one bounded TCP connection to a scoped target and port. | Stores the result and a SHA-256-addressed JSON evidence snapshot. |
| TLS diagnostic | Performs a metadata-only TLS handshake and records protocol, cipher, certificate metadata, and host trust result. | Does not send credentials or treat an untrusted certificate as trusted. |
| HTTP diagnostic | Performs a scoped `HEAD` request with no redirect following and records status plus headers, not document bodies. | Stores an evidence snapshot and append-only audit event. |
| Local dashboard | Serves host, device, job, and guarded-diagnostic data on loopback only. | API and CLI use the same authorization and execution path. |
| Local persistence | Uses SQLite WAL for state and JSONL plus JSON evidence files for auditability. | Keeps user data outside the immutable ISO image. |
| Human-approved agent console | Adds contextual local chat, visible activity, command proposal cards, task history, takeover, endpoint snapshots, local memory, and chat-added skill drafts. | Commands remain inert until the operator approves a typed catalog action; chat-added skills are data records, not executable code. |

## Deliberately unavailable in version 0.2.0

The following are **not implemented** and must not be inferred from this repository or user interface: ADB/Fastboot support, firmware selection, firmware flashing, recovery writes, credential capture, authentication bypasses, payload delivery, exploitation, remote shells, lateral movement, covert persistence, packet capture, arbitrary tool execution, Windows/macOS release validation, and physical Ventoy USB validation.

These features require vendor documentation, physical authorized hardware, dedicated permission/rollback controls, licensing review, and separate evidence. They are recorded as `BLOCKED` rather than simulated.

## Build

The project uses Go 1.22 or later. On Debian/Ubuntu, install Go and the ISO tooling listed in `docs/SAFETY_AND_FEASIBILITY.md`, then run:

```bash
cd network-ninja-master
make fmt test build
```

The Linux x86_64 binary appears at `dist/linux-amd64/network-ninja`. Build a candidate standard GRUB live ISO for Ventoy selection with:

```bash
make iso
```

The ISO build needs privileged live-image filesystem steps and network access to Debian package sources. It produces an ISO only when the live-image build actually completes.

## Authorization scope

Create a scope file before any active diagnostic. The sample below is intentionally limited to a loopback lab service.

```json
{
  "engagement_id": "lab-2026-001",
  "operator": "authorized-operator",
  "authorized_targets": ["127.0.0.1"],
  "excluded_targets": [],
  "allowed_ports": [8443],
  "allowed_operations": ["tcp_connect", "tls_metadata", "http_metadata"],
  "forbidden_operations": [],
  "start_time": "2026-08-23T00:00:00Z",
  "expiry_time": "2026-08-24T00:00:00Z",
  "rate_limit_per_min": 10,
  "network_boundary": "isolated-local-lab",
  "evidence_location": "./evidence"
}
```

Validate the scope and execute a TCP diagnostic against an authorized service:

```bash
./dist/linux-amd64/network-ninja scope validate --file scope.json
./dist/linux-amd64/network-ninja scan tcp --scope scope.json --target 127.0.0.1 --port 8443
```

For a TLS or HTTP metadata operation, use `scan tls` or `scan http`. The HTTP command receives a full URL and derives the checked host and port from it before scope validation.

## Local dashboard

Start the local dashboard:

```bash
./dist/linux-amd64/network-ninja serve --addr 127.0.0.1:8000
```

Open `http://127.0.0.1:8000` on the **same host**. The executable rejects a non-loopback listener address. The dashboard contains only backed controls: its diagnostic form invokes the same scope engine and operation pipeline as the CLI.

## Human-approved agent console

The local dashboard now opens the original **Network Ninja Console**. It keeps chat, activity, command proposals, approvals, task results, takeover status, durable local notes, and skill drafts in the configured local data root. The console has no remote administration mode.

Type a normal request to receive a contextual local response. Requests such as “inspect this computer,” “USB inventory,” or “capture a snapshot” create a visible typed action card. The operator must press **Approve and run** before the local collector executes. **Take control** pauses pending actions; ending takeover requires a new proposal. Use `skill:` or `upgrade:` in chat to add an inert local knowledge draft. Review `docs/AGENT_CONSOLE_ARCHITECTURE.md` for exact states and boundaries.

## Ventoy deployment

Ventoy persistence uses a configuration in `/ventoy/ventoy.json` that maps an ISO file to a separately stored backend image.[1] A supported deployment is:

| Ventoy first-partition path | Contents |
|---|---|
| `/iso/network-ninja-master-0.2.0-linux-amd64.iso` | The ISO produced by `make iso`. |
| `/ventoy/ventoy.json` | Copy `build/ventoy/ventoy.json` after confirming its ISO path matches the filename used. |
| `/persistence/network-ninja-master-persistence.dat` | Ext4 persistence image created by `scripts/create-persistence.sh`. |
| `/data/` | Optional external working directory for logs, SQLite state, and evidence. |

Create a backend image **before** copying it to the Ventoy first partition:

```bash
./scripts/create-persistence.sh \
  --output ./network-ninja-master-persistence.dat \
  --size-mb 4096
```

The script only writes its explicitly supplied regular-file output. It refuses `/dev/*` paths. It creates an ext4 image labeled `persistence` and writes the Debian-live `persistence.conf` union rule. The physical USB preparation and actual Ventoy boot are operator actions and are not claimed as verified until tested on the target machine.

## Release packaging and verification

Run the following after a successful build:

```bash
make package
cd dist/release/network-ninja-master-0.2.0
sha256sum -c SHA256SUMS
```

The package contains the Linux binary, checksums, Ventoy template, persistence-image script, generated SBOM, manifests, documentation, and retained build evidence. It includes an ISO only when `make iso` created one.

## Security properties

The runtime does not interpolate target values into a shell command, follows no HTTP redirects, does not collect HTTP bodies, accepts no arbitrary executable upload, does not expose an administration endpoint to the network, and writes scope/audit decisions to disk. The agent console adds only a typed, local command catalog; it does not convert chat text or skill drafts into arbitrary executable commands. Review `docs/SAFETY_AND_FEASIBILITY.md`, `docs/ARCHITECTURE.md`, and `docs/TRACEABILITY_MATRIX.md` before deployment.

## References

[1]: https://www.ventoy.net/en/plugin_persistence.html "Ventoy Persistence Plugin"
