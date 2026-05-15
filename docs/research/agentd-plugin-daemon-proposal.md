# agentd Plugin Daemon Proposal

Date: 2026-05-14

## Positioning

`agentd` is a neutral, local-first daemon for observing coding agents across
vendors, runtimes, workspaces, terminals, and orchestrators.

Naming note: `agentd` is a strong working name because the CLI shape is obvious,
but the name is already used by unrelated agent infrastructure projects and
packages. Before release, treat `agentd` as a placeholder and run a naming/legal
check. Safer package names may be `agent-activityd`, `agent-statusd`,
`agent-observed`, or `agent-registryd`, while still allowing a short local alias
such as `agent ps`.

The closest mental model is Docker:

```text
agentd ps
agentd inspect <session>
agentd logs <session>
agentd events
agentd tree
agentd plugins
```

The daemon is the stable product boundary. Agent-specific support lives in
plugins. A Claude plugin, Codex plugin, OpenCode plugin, tmux plugin, MCP proxy
plugin, worktree plugin, or microVM plugin can be added without changing the
core daemon or client API.

The core job is not to control every agent. The core job is to answer:

- What agent sessions exist?
- Where are they running?
- Are they actively working, idle, blocked, reviewable, completed, failed,
  stopped, or disconnected?
- What spawned what?
- What workspace, terminal, process, container, VM, branch, PR, or task does
  each session belong to?
- What changed since the client last checked?

## Product Principles

1. **Neutral core, vendor plugins**

   The core daemon should not know how Claude Code, Codex, OpenCode, Conductor,
   tmux, or a custom API agent works. It should know only the canonical
   activity model and the plugin protocol.

2. **Observe first, control later**

   V1 should be read-only: list, inspect, stream events, and show topology.
   Control actions such as stop, attach, approve, resume, or send prompt should
   be optional capabilities added only when a plugin can implement them safely.

3. **Explicit confidence**

   Native runtime events are high confidence. State files are medium-high.
   Hooks are medium. PTY/process/git inference is low to medium. The API should
   expose `confidence` and `source` so UIs do not pretend guesses are facts.

4. **Topology is first-class**

   A flat process list is not enough. The daemon should expose a graph of
   sessions, runtimes, workspaces, tool calls, artifacts, and parent-child
   relationships.

5. **Small API, rich metadata**

   Keep the core schema small and stable. Let plugins attach raw fields under
   `meta`, namespaced by plugin.

## CLI Shape

### `agentd ps`

Show active and recent sessions, similar to `docker ps`.

```text
$ agentd ps
ID            AGENT      STATUS      CWD                         ACTIVITY                         AGE   UPDATED
claude:7c5d   Claude     working     ~/repo/.claude/worktrees/a   editing src/auth/session.ts      18m   12s
codex:91af    Codex      blocked     ~/repo                       approval: run migration test      9m   2m
open:42cd     OpenCode   idle        ~/api                        ready for input                  44m   5m
tmux:a3.1     unknown    working?    ~/legacy                     shell output active              21m   4s
```

Useful flags:

```text
agentd ps --all
agentd ps --status working,blocked
agentd ps --agent claude
agentd ps --cwd ~/repo
agentd ps --json
agentd ps --watch
```

### `agentd tree`

Show topology.

```text
$ agentd tree
root claude:7c5d "Fix checkout flake" working
├─ subagent claude:7c5d/reviewer blocked "needs permission"
├─ tool mcp:filesystem/write working "patching checkout_test.ts"
└─ workspace git:repo:a /Users/me/repo/.claude/worktrees/a

root conductor:run-128 "Implement notifications" reviewable
├─ worker codex:91af completed
├─ worker claude:22ab working
└─ artifact pr:github.com/acme/app/pull/481 checks_pending
```

Useful flags:

```text
agentd tree <session>
agentd tree --cwd ~/repo
agentd tree --format dot
agentd tree --json
```

### `agentd inspect`

Return the full normalized record plus plugin metadata.

```text
agentd inspect claude:7c5d
agentd inspect claude:7c5d --json
```

### `agentd events`

Stream daemon events.

```text
agentd events
agentd events --since 1842
agentd events --json
```

### `agentd plugins`

Manage plugins.

```text
agentd plugins
agentd plugins install agentd-plugin-opencode
agentd plugins enable claude
agentd plugins disable tmux
agentd plugins doctor
```

## Daemon API

Use newline-delimited JSON-RPC 2.0 over a Unix domain socket.

Default:

```text
$XDG_RUNTIME_DIR/agentd.sock
```

Fallbacks:

```text
/tmp/agentd-$UID.sock
~/.agentd/agentd.sock
```

Core read API:

```text
daemon/info
plugins/list
plugins/inspect
sessions/list
sessions/get
graph/get
events/subscribe
events/unsubscribe
health/check
```

Optional control API:

```text
sessions/attach
sessions/stop
sessions/send_prompt
approvals/respond
plugins/configure
```

V1 should not require any control method.

## Canonical Model

### Session

```json
{
  "id": "claude:7c5dcf5d",
  "kind": "session",
  "agent": {
    "name": "Claude Code",
    "vendor": "anthropic",
    "adapter": "claude"
  },
  "title": "Fix checkout flake",
  "summary": "Editing checkout tests after reproducing the failure",
  "status": "working",
  "statusDetail": "editing src/checkout_test.ts",
  "confidence": "high",
  "source": "claude:state-file",
  "cwd": "/Users/me/repo/.claude/worktrees/fix-checkout",
  "workspaceRoots": ["/Users/me/repo"],
  "startedAt": "2026-05-14T16:12:00Z",
  "lastActivityAt": "2026-05-14T16:30:11Z",
  "parentSessionId": null,
  "rootSessionId": "claude:7c5dcf5d",
  "runtimeIds": ["process:44218"],
  "workspaceIds": ["git-worktree:repo:fix-checkout"],
  "artifactIds": ["pr:github.com/acme/app/pull/481"],
  "capabilities": {
    "logs": true,
    "children": false,
    "attach": false,
    "stop": false,
    "sendPrompt": false,
    "approvals": false
  },
  "meta": {
    "claude": {
      "rawStatus": "working",
      "backgroundJobId": "7c5dcf5d"
    }
  }
}
```

### Status Vocabulary

```text
idle          reachable and ready for new input
working       model, tool, command, workflow, CI, or background step is active
blocked       waiting for user input, permission, approval, auth, option, or conflict resolution
reviewable    produced a diff, PR, report, or artifact that needs review
completed     finished successfully
failed        ended with an error
stopped       stopped by user, tool, or supervisor
disconnected  known session exists but backend/runtime is unreachable
unknown       plugin cannot determine status
```

Plugins may expose raw status under namespaced `meta`.

### Graph Node Types

```text
session       agent conversation, run, or work item execution
turn          one prompt/response/tool cycle
tool_call     shell/API/MCP/browser/file operation
runtime       process, tmux pane, app-server, container, microVM, or cloud worker
workspace     cwd, repo root, git worktree, checkout, container mount, or VM filesystem
artifact      diff, PR, issue, report, log bundle, branch, or generated file set
approval      permission request, option selection, or human question
```

### Graph Edge Types

```text
spawned_by       child session was created by parent session
delegated_by     parent delegated a task to child
runs_in          session runs inside runtime
uses_workspace   session uses workspace
produced         session produced artifact
blocked_by       session is blocked by approval, conflict, auth, review, or CI
mirrors          two records represent the same underlying session from different plugins
sibling_of       sessions share a parent task, branch family, or orchestrator run
```

## Plugin System

### Plugin Packaging

A plugin is an executable plus a manifest.

```json
{
  "schemaVersion": "agentd.plugin/v1",
  "name": "agentd-plugin-opencode",
  "displayName": "OpenCode",
  "version": "0.1.0",
  "entrypoint": ["agentd-plugin-opencode"],
  "protocol": "agentd-plugin-jsonrpc/v1",
  "provides": ["sessions", "events", "children", "logs"],
  "permissions": [
    "network:localhost",
    "read:~/.config/opencode",
    "process:list"
  ],
  "agentTypes": ["opencode"],
  "configSchema": {}
}
```

The daemon starts plugins as child processes and talks to them over stdio
JSON-RPC. This keeps plugins portable, crash-isolated, and easy to write in any
language.

Later, high-performance or sandboxed plugins could use:

- Unix socket plugin processes.
- WASI plugins for strict sandboxing.
- Built-in trusted plugins for core OS/runtime discovery.

### Plugin RPC

Daemon to plugin:

```text
plugin/initialize
plugin/capabilities
plugin/configure
plugin/scan
plugin/subscribe
plugin/unsubscribe
plugin/health
plugin/shutdown
```

Plugin to daemon:

```text
session/upsert
session/remove
graph/upsert_node
graph/upsert_edge
event/emit
log/append
diagnostic/report
```

Plugins can be pull-based, push-based, or both.

Pull examples:

- Scan Claude Agent View state files.
- Query OpenCode server APIs.
- Poll tmux panes and process trees.
- Inspect git worktrees.
- Query container or VM runtimes.

Push examples:

- Receive Claude/Codex hooks.
- Receive MCP proxy events.
- Receive SDK instrumentation events.
- Receive native agent registration heartbeats.

## Plugin Classes

### Native Agent Plugin

Best quality. A compatible agent or orchestrator reports directly to `agentd`
or to its plugin.

Signals:

- session started/stopped
- turn started/finished
- tool call started/finished
- approval requested/resolved
- child session spawned
- artifact produced

Status confidence: high.

### Runtime Adapter Plugin

Reads structured local APIs or state from existing tools.

Examples:

- Claude Agent View state files.
- Codex app-server.
- OpenCode HTTP/SSE server.
- RoboRev daemon API and JSONL event stream.
- OpenClaw gateway/session state and lifecycle hooks.
- Hermes Agent CLI/gateway state and plugin hooks.
- NanoClaw SQLite/session/container state.
- Conductor local state, if exposed.
- AgentPulse or AgentDeck local state, if users opt in.

Status confidence: medium-high to high.

### Hook Plugin

Installs or receives lifecycle hooks from agent CLIs.

Examples:

- Claude Code hooks.
- Codex hooks.
- shell wrapper hooks.
- post-tool or pre-tool events.

Status confidence: medium to high, depending on hook coverage.

### MCP Proxy Plugin

Runs as an MCP proxy between agents and MCP servers.

It can observe:

- tool call start/end
- progress notifications
- elicitation/user-input requests
- roots/workspace information
- errors and cancellations

It cannot fully observe model generation unless combined with SDK/provider
instrumentation.

Status confidence: medium for tool activity, low for total session state unless
paired with another plugin.

### API Instrumentation Plugin

Wraps Anthropic/OpenAI SDK calls or receives OpenTelemetry-style spans from
custom agents.

It can observe:

- model stream start/end
- tool call planning
- function call output
- handoffs
- token usage
- errors

Status confidence: high for custom agents, unavailable for opaque CLIs.

### tmux/PTTY Plugin

Fallback for arbitrary terminal agents.

Signals:

- pane exists
- process tree
- recent output
- prompt detection
- known approval/input patterns
- command still running
- terminal title/badge markers

Status confidence: low to medium.

This plugin should report `working?` style uncertainty through `confidence`.

### Git Worktree Plugin

Tracks workspace topology, not agent activity.

Signals:

- repository root
- worktree path
- branch
- HEAD
- dirty files
- associated PR
- parent checkout

Status confidence: high for workspace facts, low for active/idle state.

### Container/MicroVM Plugin

Tracks runtime isolation.

Signals:

- container/VM alive
- mounted workspace
- process list
- CPU/network/disk activity
- guest-agent heartbeat
- associated agent session ID from env vars or labels

Status confidence: high for runtime liveness, low to medium for semantic
agent state unless guest-side instrumentation is installed.

## Candidate Community Plugins

### RoboRev

RoboRev should be a strong source integration for review-oriented sessions. It
already runs a local daemon, stores durable review jobs, exposes a REST/OpenAPI
surface and JSONL event stream, supports multiple coding agents, and uses git
hooks to queue work as commits are produced.

Best integration path:

- discover the configured RoboRev daemon address
- consume public job/review/repo/branch endpoints
- subscribe to `roborev stream` or the daemon event stream
- map review, analyze, fix, and refine jobs to Score sessions
- map repo path and branch to workspace resources
- map commit SHA, dirty diff marker, verdict, findings, and job ID to artifacts
  and `meta.roborev`
- represent refine iterations or fix jobs as child sessions when visible

RoboRev is also a reference implementation for how a local daemon can expose
both human-friendly CLI commands and machine-friendly API/event surfaces without
requiring a hosted service.

### OpenClaw

OpenClaw should be a strong plugin target. It has a gateway/control-plane shape,
session concepts, and documented lifecycle/plugin hooks such as session start,
session end, before/after tool call, message received/sent, and gateway
start/stop. A `score-plugin-openclaw` plugin could provide high-confidence
session lifecycle, tool activity, blocked/error state, and gateway/channel
metadata.

Best integration path:

- consume OpenClaw lifecycle/plugin hooks
- read local session state if stable
- map Gateway runs to `session`
- map delegated agents or tool-driven child work to graph edges when visible
- expose channel metadata under `meta.openclaw`

### NanoClaw

NanoClaw should be supportable, but probably through a composed adapter at
first. It is built on the Claude Agent SDK, uses SQLite for messages/sessions
and state, and runs agents in isolated containers. That means Score can combine
three signal sources:

- Claude/SDK or hook events for semantic activity
- NanoClaw SQLite/session state for session identity and memory
- Docker/Apple Container runtime data for liveness and isolation topology

Best integration path:

- start with Claude + container + SQLite observation
- add a dedicated NanoClaw plugin once its local schema/plugin surface is stable
- represent agent swarms as child sessions when NanoClaw exposes swarm metadata

### Hermes Agent

Hermes Agent should be a good plugin target. It exposes CLI and messaging
gateway surfaces, has plugins and hooks, supports MCP, and documents subagent
hooks such as child stop events. It also has bundled skills for delegating to
Claude Code, Codex, OpenCode, and Hermes itself.

Best integration path:

- use Hermes hooks/plugins for session, tool, and subagent lifecycle
- use CLI/gateway state for session discovery
- use MCP proxy signals where Hermes calls MCP tools
- when Hermes delegates to Claude/Codex/OpenCode, preserve parent-child
  topology by propagating `SCORE_*` or `AGENTD_*` environment variables into the
  delegated process

Hermes is especially useful for validating topology because it can be both a
root agent and an orchestrator of other coding agents.

## Identity And Topology

The daemon should derive stable identities from multiple signals.

Preferred identity sources:

1. Native `AGENTD_SESSION_ID`.
2. Agent runtime session ID.
3. Orchestrator run ID.
4. Claude/Codex/OpenCode session ID.
5. tmux session/window/pane ID plus process start time.
6. Git worktree path plus branch plus runtime PID.

Recommended environment variables for native integration:

```text
AGENTD_SOCKET=/tmp/agentd-501.sock
AGENTD_SESSION_ID=...
AGENTD_PARENT_SESSION_ID=...
AGENTD_ROOT_SESSION_ID=...
AGENTD_WORKSPACE_ID=...
AGENTD_RUNTIME_ID=...
AGENTD_AGENT_NAME=...
```

When a parent agent spawns a child process, subagent, tmux pane, container, or
microVM, the parent or wrapper should propagate these variables. That makes
topology reliable without scraping logs.

## State Resolution

Multiple plugins may report the same session. The daemon should merge records
using source priority and identity hints.

Example priority:

1. Native registration.
2. Structured app-server or runtime API.
3. Official state files.
4. Hooks.
5. MCP/API proxy instrumentation.
6. tmux/process inference.
7. git/worktree inference.

Each field can carry provenance:

```json
{
  "status": {
    "value": "working",
    "confidence": "high",
    "source": "opencode:sse",
    "observedAt": "2026-05-14T16:30:11Z"
  }
}
```

The public API can flatten this for ergonomics while allowing `--verbose` or
`inspect` to show field-level provenance.

## MVP

V1 should include:

- `agentd` daemon with Unix socket JSON-RPC.
- `agentd ps`, `inspect`, `events`, `tree`, and `plugins`.
- Plugin host over stdio JSON-RPC.
- SQLite event/state store.
- OpenCode plugin using server APIs and SSE.
- Claude plugin using Agent View state files and hooks where available.
- RoboRev plugin using daemon API and event stream for review-job sessions.
- tmux/process plugin for fallback discovery.
- git worktree plugin for workspace topology.
- JSON output suitable for external dashboards.

V1 should explicitly exclude:

- sending prompts
- approval handling
- stopping sessions
- attaching to terminals
- LAN dashboard exposure by default
- cloud sync

## Why This Is Different From Existing Tools

Existing early tools tend to be UI-first:

- dashboards
- control surfaces
- terminal managers
- worktree launchers
- approval queues

`agentd` should be infrastructure-first:

- small daemon
- stable local API
- plugin protocol
- normalized state model
- topology graph
- CLI clients first, richer UIs later

The winning wedge is not another dashboard. The wedge is that every dashboard,
editor extension, status bar, terminal plugin, notification service, and
orchestrator can call the same local API:

```text
agentd ps --json
agentd events --json
agentd tree --json
```

That is the missing layer.
