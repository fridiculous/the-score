# The Score

The Score is a headless local observability daemon for coding-work sessions,
processes/runtimes, workspaces, lineage, sources, and recent metadata events.

The daemon is `scored`. The CLI is `score`, and it is a thin client over the
local API.

## Quick Start

Run the daemon:

```bash
go run ./cmd/scored
```

In another terminal:

```bash
go run ./cmd/score sources
go run ./cmd/score sessions
go run ./cmd/score processes
```

Report a session through the native observation API:

```bash
go run ./cmd/score observe-session \
  --id codex:demo \
  --status working \
  --source codex \
  --workspace "$PWD" \
  --activity "running tests"
```

Then inspect the resource graph:

```bash
go run ./cmd/score sessions
go run ./cmd/score lineage codex:demo
go run ./cmd/score history
```

## API Shape

The primary API is newline-delimited JSON-RPC over a local transport:

- macOS/Linux: Unix domain socket.
- Windows: named pipe.

Set `SCORE_SOCKET` to override the default address.

Core methods:

```text
daemon/info
health/check
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
```

Native observation ingest methods:

```text
observations/upsertSession
observations/removeSession
observations/upsertWorkspace
observations/upsertEdge
```

## CLI Resources

```text
score sessions      # session snapshot
score processes     # runtime/process snapshot
score workspaces    # known workspaces
score lineage <id>  # parent/child and linked resource graph
score events        # metadata events
score history       # recent metadata timeline
score sources       # integrations/sources and diagnostics
score inspect <id>  # full resource record
```

All read commands support `--json`.

## Current Scope

This is the first headless MVP. It includes:

- a Go daemon and CLI
- local JSON-RPC transport
- session, workspace, lineage, source, and event models
- process listing source
- native observation ingest API
- source diagnostics for the planned core bundle

The first implementation does not yet include Claude, Codex, OpenCode, tmux,
git-worktree, MCP, or container-specific source implementations. They are
represented as source diagnostics and are intended to be added behind the same
API.

## Contributing And Releases

- Commit convention: [CONTRIBUTING.md](CONTRIBUTING.md)
- Release model: [docs/release.md](docs/release.md)
- Repository guidance for future agents/contributors: [AGENTS.md](AGENTS.md)
