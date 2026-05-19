# API Contract

The Score daemon, `scored`, exposes newline-delimited JSON-RPC 2.0 over a local
transport.

- macOS/Linux: Unix domain socket.
- Windows: named pipe.
- Override address: `SCORE_SOCKET`.

`v0.0.1` is pre-1.0, but the method names and response shapes below are
intentional API surface.

## Version Metadata

`daemon/info` returns:

```json
{
  "name": "The Score",
  "daemon": "scored",
  "daemonVersion": "0.0.1",
  "apiVersion": "score-jsonrpc/v1",
  "sourcePackVersion": "source-pack/v0",
  "buildCommit": "unknown",
  "pid": 1234,
  "startedAt": "2026-05-16T12:00:00Z",
  "storagePath": "/Users/example/Library/Application Support/the-score/score.db"
}
```

The legacy `api` field may also be present and mirrors `apiVersion`.

## Methods

Core:

```text
daemon/info
health/check
refresh/status
sessions/list
sessions/get
processes/list
processes/get
workspaces/list
workspaces/get
lineage/get
events/list
events/subscribe
sources/list
sources/doctor
sources/testFixtures
```

Native observation ingest:

```text
observations/upsertSession
observations/removeSession
observations/upsertWorkspace
observations/upsertEdge
```

## Status Vocabulary

Sessions use these statuses:

```text
idle
working
blocked
reviewable
completed
failed
stopped
disconnected
unknown
```

Attention values:

```text
none
input
review
error
```

Confidence values:

```text
high
medium
low
unknown
```

Process-only agent detection is `low` confidence. It means Score saw a process
shape that matches a source pack. It does not mean the session is blocked,
idle, reviewable, or safe to control.

## Lifecycle Fields

Sessions separate liveness from activity:

- `lastSeenAt`: latest time any source saw evidence that the session/runtime
  still exists. Process probes may update this field.
- `lastActivityAt`: latest time a structured source or native observation
  reported activity. Process probes must not advance this after initial
  detection.
- `statusUpdatedAt`: time the current status was last changed.
- `statusSource`: source that supplied the current status.

Process-backed session candidates become `disconnected` when the process probe
has not seen them past the daemon grace period. A missing process does not imply
`completed`; only a native observation, `score run` exit, or structured source
state should report terminal statuses.

Lifecycle event types:

```text
session.detected
session.started
session.heartbeat
session.activity
session.waiting_for_input
session.reviewable
session.idle
session.completed
session.failed
session.stopped
session.disconnected
session.status_changed
session.updated
session.removed
```

## Storage

The daemon uses SQLite by default. The data path is:

- `SCORE_DATA_DIR/score.db` when `SCORE_DATA_DIR` is set.
- macOS: `~/Library/Application Support/the-score/score.db`.
- Linux: `$XDG_DATA_HOME/the-score/score.db` or
  `~/.local/share/the-score/score.db`.
- Windows: `%LOCALAPPDATA%\the-score\score.db`.

`processes/list` remains live process-table data. Sessions, workspaces, lineage
edges, sources, and events are persisted as daemon state/history.

## Source Fixture Validation

Run all bundled fixture checks through the daemon:

```bash
score sources test-fixtures
```

Run one source family:

```bash
score sources test-fixtures codex
```

The JSON-RPC method is `sources/testFixtures` with optional params:

```json
{ "id": "codex" }
```
