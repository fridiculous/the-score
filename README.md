# The Score

The Score is a headless local observability daemon for coding-work sessions,
processes/runtimes, workspaces, lineage, sources, and recent metadata events.

The daemon is `scored`. The CLI is `score`, and it is a thin client over the
local API.

## Quick Start

Run the daemon:

```bash
go run ./cmd/score start
```

For signal/shutdown testing, build and run the daemon binary directly:

```bash
go build -o scored ./cmd/scored
./scored
```

`Ctrl-C` should stop `scored` cleanly. When using `go run`, the Go wrapper may
still report an interrupt exit code even though the daemon shuts down promptly.

In another terminal:

```bash
go run ./cmd/score sources
go run ./cmd/score sessions
go run ./cmd/score processes
```

Score passively detects known running agent processes, currently including
`codex`, `claude`, `opencode`, `hermes`, `openclaw`, and `nanoclaw`. You do not
need to launch those tools through Score for basic active-process visibility:

```bash
go run ./cmd/score sessions --source codex
```

Passive process detection is low-confidence because it can only infer that a
process is alive, not whether the underlying session is blocked, idle, or
reviewable. On Unix, Score also tries to resolve the process cwd so the
workspace column can populate.

For commands Score launches itself, `score run` records stronger metadata,
workspace identity, and parent/root session environment:

```bash
go run ./cmd/score run -- sleep 30
go run ./cmd/score run -- codex
```

While that command is running, another terminal will show it:

```bash
go run ./cmd/score sessions
```

Stop the daemon:

```bash
go run ./cmd/score stop
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
- passive low-confidence process-to-session inference for common agent CLIs
- native observation ingest API
- source diagnostics for the planned core bundle

The first implementation does not yet include deep Claude, Codex, OpenCode,
tmux, git-worktree, MCP, or container-specific source implementations. The
agent CLIs above have passive process probes; deeper telemetry is represented as
source diagnostics and is intended to be added behind the same API.

## Contributing And Releases

- Commit convention: [CONTRIBUTING.md](CONTRIBUTING.md)
- Release model: [docs/release.md](docs/release.md)
- Repository guidance for future agents/contributors: [AGENTS.md](AGENTS.md)
