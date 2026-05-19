# What Score Sees

Score models coding-work metadata as sessions, processes, workspaces, lineage,
events, sources, and history. Agent names are source metadata, not the core
abstraction.

## Codex

When a `codex` process is running, Score can infer a low-confidence session
candidate:

```json
{
  "id": "codex:process:100",
  "status": "unknown",
  "confidence": "low",
  "source": "codex",
  "statusDetail": "process detected: codex",
  "statusSource": "process",
  "lastSeenAt": "2026-05-16T12:00:00Z",
  "lastActivityAt": "2026-05-16T12:00:00Z"
}
```

This proves process presence only. It does not prove whether the session is
blocked, idle, or waiting for review.

Later process scans update `lastSeenAt`, not `lastActivityAt`. If the process
disappears and remains unseen past the daemon grace period, Score marks the
candidate `disconnected` rather than `completed`.

## Hermes

Launcher-backed CLIs can be recognized when the command shape identifies the
source:

```json
{
  "command": "python3",
  "args": "python3 hermes --workspace /work/app",
  "source": "process"
}
```

The resulting session remains low confidence until Hermes provides structured
session metadata through a source API or native observation calls.

## OpenCode

Node-backed launchers can also map to a source pack:

```json
{
  "command": "node",
  "args": "node opencode run",
  "source": "process"
}
```

Fixture tests cover this shape so future changes do not break it silently.

## Generic Processes And False Positives

Score should not infer a session just because an agent name appears in an
argument:

```json
{
  "command": "rg",
  "args": "codex"
}
```

Shell snippets, app helper processes, and provider flags are also treated as
false-positive cases unless they match a source pack's supported process shape.
