# Human-Approved Agent Console Architecture

## Objective

The agent console extends Network Ninja Master with an original local chat and activity interface. It presents a cooperative, concise working style, but it is not a clone of any third-party product interface. The console provides contextual chat, visible task status, activity history, terminal proposal cards, a takeover switch, durable local memory, and skill drafts submitted through the chat interface.

## Capability mapping

| Requested capability | Implementation | Runtime boundary |
|---|---|---|
| Computer activity | A local activity timeline records observed host/device snapshots, proposed actions, approvals, executions, cancellation, and takeover events. | The first release does not synthesize mouse or keyboard input, capture screens, or remotely control another device. |
| Terminal access | A typed command catalog exposes local, read-only host and device inventory commands. A command proposal requires explicit approval before it may run. | No arbitrary shell text, privilege escalation, network shell, or hidden execution path is available. |
| Command knowledge | Each command has an identifier, explanation, risk class, argument schema, expected result, and approval requirement. | Commands are immutable program-defined handlers in this release. |
| Memory knowledge base | SQLite stores user messages, assistant messages, durable facts, activity records, pending approvals, skill drafts, and task state. | Memory stays in the configured local data root and is queryable in the console. |
| Orchestrator | Chat input becomes a typed task proposal, task state transition, optional approval, execution, verification, and activity event. | A task cannot execute while in `PENDING_APPROVAL`, `CANCELLED`, or `TAKEOVER`. |
| Agent backend | Loopback-only JSON APIs expose chat, task, approvals, activity, memories, skills, and monitoring. | The backend never binds to a non-loopback address. |

## Context and acceleration layers

The chat service uses a deterministic layered context builder: current task state, recent activity, durable facts, recent conversation, and command/skill metadata. It returns an explainable coworker-style response immediately and surfaces a compact context summary. A configured model endpoint can later be added as a separate assistant layer, but no model receives permission to execute commands directly.

The console recognises `skill:` and `upgrade:` chat prefixes. These create a **draft** skill record containing the operator-provided title and content. Draft skills are informative only; they are never executed, installed, or converted to code automatically. The operator can review, approve, archive, or delete them through the same visible history.

## Approval and takeover state machine

| State | Meaning | Permitted transition |
|---|---|---|
| `PENDING_APPROVAL` | A command or monitored action has been proposed. | `APPROVED`, `CANCELLED`, `TAKEOVER` |
| `APPROVED` | The operator approved the specific action. | `RUNNING`, `CANCELLED`, `TAKEOVER` |
| `RUNNING` | A bounded local handler is executing. | `COMPLETED`, `FAILED`, `TAKEOVER` |
| `COMPLETED` | The action returned verified structured output. | Terminal state |
| `FAILED` | The action failed and an activity record explains why. | Terminal state |
| `CANCELLED` | The operator withdrew approval before execution. | Terminal state |
| `TAKEOVER` | Automation is paused; the operator is explicitly in control. | `PENDING_APPROVAL` only after a new proposal |

## Monitoring model

A monitor snapshot captures host inventory, local listening-interface metadata exposed by the existing inventory layer, and connected local devices. It creates a time-stamped activity event and can be invoked manually from the console. It is not a background collector in this release, avoiding hidden long-running behavior and making each collection visible to the operator.

## Acceptance checks

The smoke tests must prove that chat context is persisted, `skill:` chat creates a draft, a terminal action stays pending until approval, approval runs the bounded handler, cancellation prevents execution, takeover stops pending work, monitor snapshots create activity, and non-loopback binding remains rejected.
