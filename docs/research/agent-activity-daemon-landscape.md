# Agent Activity Daemon Landscape Research

Date: 2026-05-14

## Summary

The opportunity is an open-source, local-first daemon that exposes a low-level API for observing coding agents across tools, sessions, directories, and spawned child work. The daemon should answer a few basic questions reliably:

- Which agents and sessions exist right now?
- Which directory or workspace is each one operating in?
- Is each session idle, working, blocked on input, reviewable, completed, failed, stopped, or disconnected?
- Did a session spawn subagents or child sessions, and what is each child doing?
- What changed recently, so UIs can update without polling everything?

The recommended shape is a protocol-first daemon with adapters. ACP should be supported, but ACP alone is not the right product boundary for a global local activity registry. PTY parsing should exist as a fallback for tools that do not expose structured state, but it should not be the core contract.

## Landscape Notes

### Agent Client Protocol (ACP)

ACP is the closest emerging standard for editor-to-agent communication. It uses JSON-RPC and currently standardizes lifecycle operations like `initialize`, `session/new`, `session/load`, `session/prompt`, `session/cancel`, `session/list`, and `session/update`.

Useful pieces for this daemon:

- `session/list` can discover known sessions, including `sessionId`, `cwd`, optional `title`, `updatedAt`, and `_meta`.
- `session/update` already carries real-time progress such as message chunks, tool calls, plans, mode changes, and session metadata updates.
- ACP requires absolute paths, which is good for unambiguous directory grouping.
- ACP has an extensibility story through `_meta`, custom methods, and custom capabilities.
- Draft RFDs are moving toward additional workspace roots, usage/context status, telemetry export, and proxy chains.

Limitations:

- ACP is primarily scoped to a client talking to one agent process, not a per-user cross-tool activity registry.
- It does not fully standardize live status vocabulary like idle, working, blocked, completed, or failed across all agents.
- It does not guarantee that subagents appear as separate first-class sessions.
- ACP transport is currently stdio-first, while this daemon needs a stable Unix domain socket service for many UIs.

Conclusion: ACP should be one adapter and one influence on the protocol design, not the entire system.

Sources:

- https://agentclientprotocol.com/protocol/overview
- https://agentclientprotocol.com/protocol/session-list
- https://agentclientprotocol.com/protocol/transports
- https://agentclientprotocol.com/rfds/session-usage
- https://agentclientprotocol.com/rfds/agent-telemetry-export
- https://agentclientprotocol.com/rfds/proxy-chains
- https://agentclientprotocol.com/rfds/additional-directories

### Claude Code Agent View

Claude Code Agent View is the clearest product analogue for the desired experience. It shows background Claude Code sessions in one place, grouped by state and directory, with quick status, row summaries, attach/peek/reply, and background session lifecycle management.

Useful pieces for this daemon:

- It distinguishes working, needs input, idle, completed, failed, stopped, and sleeping loop sessions.
- It shows that global session visibility across projects is valuable.
- It uses a supervisor process and on-disk per-session state.
- It tracks session state under the Claude config directory, including a roster and per-job state.
- Background sessions have durable IDs and can be attached, stopped, respawned, or removed.

Limitations:

- It is Claude-specific and not an open cross-agent protocol.
- Interactive sessions do not show up until backgrounded.
- Subagents spawned by a session are not listed as separate rows.
- State/control semantics are product-specific.

Conclusion: Agent View validates the user experience. The open-source daemon should generalize the observable subset across agents and expose child work separately when available.

Source:

- https://code.claude.com/docs/en/agent-view

### Codex

Codex app-server exposes a structured JSON-RPC application server with thread and turn lifecycle events. It is a strong fit for an adapter because it has explicit event streams and lifecycle notifications.

Useful pieces for this daemon:

- Thread lifecycle events include thread start, archive, unarchive, close, and status changes.
- Turn lifecycle events and item lifecycle events expose active work and tool activity.
- Codex app-server supports skills, apps, dynamic tool calls, and request/response flows that can be normalized into activity events.
- Codex subagents exist as a first-class user-facing concept in the current Codex product surface.

Limitations:

- The public docs describe app-server and product concepts, but this daemon still needs adapter-specific mapping from Codex thread/turn/item events into a generic status vocabulary.
- Some child-agent relationships may be product-internal unless surfaced through app-server events or metadata.

Conclusion: Codex should get a structured app-server adapter first, with a lower-quality CLI/session fallback only if needed.

Sources:

- https://developers.openai.com/codex/app-server
- https://developers.openai.com/codex/subagents
- https://developers.openai.com/codex/cli

### OpenCode

OpenCode has a useful client-server architecture. Its TUI starts a server, and `opencode serve` exposes an HTTP server with an OpenAPI 3.1 spec, global SSE events, session APIs, session status, and child-session endpoints.

Useful pieces for this daemon:

- `GET /session` lists sessions.
- `GET /session/status` returns status for all sessions.
- `GET /session/:id/children` exposes child sessions.
- `GET /global/event` exposes server-sent events.
- `opencode acp` supports ACP-compatible editor integrations.
- OpenCode distinguishes primary agents and subagents, and child sessions are a documented navigation concept.

Limitations:

- The native API is HTTP/OpenAPI rather than the desired Unix socket JSON-RPC.
- Status normalization still needs care because OpenCode status fields may not align exactly with other tools.

Conclusion: OpenCode is one of the best V1 adapters because it already exposes session status and child sessions.

Sources:

- https://open-code.ai/en/docs/server
- https://open-code.ai/en/docs/agents
- https://open-code.ai/en/docs/acp
- https://open-code.ai/en/docs/cli

### Chorus

Chorus is a local daemon and cockpit for multi-LLM code review. It orchestrates existing AI CLIs and exposes an MCP interface with chat lifecycle operations.

Useful pieces for this daemon:

- It already has a local daemon, web UI, SQLite database, and MCP server.
- It tracks runs/chats with statuses such as reviewing and terminal results.
- MCP tools include create, wait, status, cancel, resume, list templates/personas, invoke persona, and list blocked.
- It runs multiple AI reviewers as subprocesses, which maps naturally to parent run plus child reviewer sessions.

Limitations:

- Chorus is review-run oriented, not a universal agent-session registry.
- Reviewer subprocess status may require reading Chorus internals unless public API coverage is sufficient.
- It has its own daemon and UI surface; this project should interoperate rather than replace it.

Conclusion: Chorus is both a competitor pattern and a useful adapter target. It validates the daemon-plus-UI shape.

Source:

- https://github.com/chorus-codes/chorus

### Symphony

Symphony turns project work into isolated autonomous implementation runs. It watches work items, spawns agents, collects proof of work, and manages PR lifecycle.

Useful pieces for this daemon:

- It treats work, not chat, as the primary object.
- It creates isolated implementation runs and workspaces.
- It manages lifecycle states around implementation, CI, PR review, proof of work, and landing.
- It is a strong model for exposing run status, workspace path, PR link, and current stage.

Limitations:

- It is an orchestrator, not a low-level cross-agent observation protocol.
- Its state vocabulary is workflow-centric and likely needs mapping to generic session status.

Conclusion: Symphony should be represented as a workflow-run adapter where each run becomes a root session and any spawned implementation agents become child sessions when visible.

Source:

- https://github.com/openai/symphony

### AgentDeck

AgentDeck is close to the desired category: a daemon hub plus multiple dashboards and control surfaces for AI coding agents.

Useful pieces for this daemon:

- It supports Claude Code, Codex CLI, OpenCode, and OpenClaw-style gateway integrations.
- It centralizes state in a daemon and broadcasts updates to UIs.
- It has a documented state machine: disconnected, idle, processing, awaiting permission, awaiting option, and awaiting diff.
- It has `sessions_list`, `state_update`, `usage_update`, and WebSocket protocol events.
- It uses a mix of PTY parsing, hooks, SSE, WebSocket, and daemon state relay.

Limitations:

- It is more product/control-surface oriented than a minimal open status protocol.
- Its protocol is custom WebSocket rather than JSON-RPC over Unix domain socket.
- Some adapter paths are platform-specific.

Conclusion: AgentDeck is the strongest existing proof that state aggregation across agents is valuable. The new daemon should be narrower and lower-level: status registry first, UI/control surfaces second.

Sources:

- https://github.com/puritysb/AgentDeck
- https://github.com/puritysb/AgentDeck/blob/master/docs/daemon.md
- https://github.com/puritysb/AgentDeck/blob/master/docs/protocol.md
- https://github.com/puritysb/AgentDeck/blob/master/docs/gateway-protocol.md

## Recommended Product Shape

Build a per-user daemon named generically, for example `agentd`, `agent-statusd`, or `agent-activityd`.

V1 should be observe-only:

- Report status.
- Report directory/workspace.
- Report parent/child relationships.
- Report last activity and current activity summary.
- Stream events.
- Avoid control actions like stop, attach, send prompt, or approve permissions until the observation model is stable.

This keeps the daemon easy to adopt. Existing tools can integrate without delegating control of their agents.

## Protocol Recommendation

Use newline-delimited JSON-RPC 2.0 over a Unix domain socket.

Default socket path:

```text
$XDG_RUNTIME_DIR/agent-activity.sock
```

Fallbacks:

```text
/tmp/agent-activity-$UID.sock
~/.agent-activity/agent.sock
```

Optional bridges:

- HTTP over localhost for debugging and generated clients.
- WebSocket or SSE bridge for browser UIs.
- mDNS only for explicit LAN dashboard support, not V1 default.

### Core Methods

```text
daemon/info
adapters/list
sessions/list
sessions/get
events/subscribe
events/unsubscribe
health/check
```

### Optional Later Methods

```text
sessions/attach
sessions/stop
sessions/send_prompt
sessions/respond
approvals/respond
```

These should not be part of V1 unless a native adapter can implement them safely and consistently.

## Canonical Data Model

### Session

```json
{
  "id": "claude:7c5dcf5d",
  "adapter": "claude",
  "agentType": "claude-code",
  "title": "Investigate flaky checkout test",
  "summary": "Running tests and inspecting failure logs",
  "cwd": "/Users/me/project",
  "workspaceRoots": ["/Users/me/project"],
  "status": "working",
  "processState": "alive",
  "lastActivityAt": "2026-05-14T18:22:00Z",
  "startedAt": "2026-05-14T18:10:00Z",
  "parentSessionId": null,
  "rootSessionId": "claude:7c5dcf5d",
  "spawnKind": "root",
  "capabilities": {
    "children": false,
    "usage": true,
    "currentTool": true,
    "control": false
  },
  "links": {
    "pr": "https://github.com/org/repo/pull/123"
  },
  "meta": {}
}
```

### Status Vocabulary

Use a compact, UI-friendly vocabulary:

- `idle`: session is ready for input.
- `working`: model, tool, test, command, or workflow step is active.
- `blocked`: user input, approval, permission, option, or conflict resolution is required.
- `reviewable`: the session produced an artifact that needs review, such as a PR or diff.
- `completed`: task finished successfully.
- `failed`: task ended with an error.
- `stopped`: user or tool stopped the session.
- `disconnected`: session exists but the process/backend is not reachable.
- `unknown`: adapter cannot determine status.

Adapters can also expose raw status under `meta.rawStatus`.

### Events

```text
session.upserted
session.removed
status.changed
activity.changed
child.spawned
approval.requested
adapter.connected
adapter.disconnected
adapter.heartbeat
```

Events should include monotonically increasing sequence numbers per daemon process and enough session data for UIs to update without immediately refetching everything.

## Adapter Strategy

### Native Adapter Contract

New agents should meet the daemon protocol directly by either:

- registering with the daemon over the Unix socket, or
- exposing their own local socket that the daemon can discover and subscribe to.

Native agents should emit child sessions explicitly. A subagent should not be hidden inside a text log if it is doing independent work.

### ACP Adapter

Map ACP concepts into daemon sessions:

- `session/list` -> session discovery.
- `SessionInfo.cwd` -> `cwd`.
- `SessionInfo.updatedAt` -> `lastActivityAt`.
- `session/update` tool calls/plans/message chunks -> `activity.changed`.
- `session_info_update` -> title and metadata update.
- Future `additionalDirectories` -> `workspaceRoots`.
- Future `usage_update` -> usage metadata.

Status inference:

- Active `session/prompt` or in-flight tool call -> `working`.
- Permission request or elicitation -> `blocked`.
- Prompt response with normal stop reason -> `idle` or `completed`, depending on agent metadata.
- Transport closed unexpectedly -> `disconnected`.

### Claude Adapter

Read documented Claude background-agent state:

- `~/.claude/daemon/roster.json`
- `~/.claude/jobs/<id>/state.json`
- `~/.claude/daemon.log` only as diagnostic fallback.

Map Claude Agent View states into canonical statuses. Do not attempt control actions in V1. Subagents are exposed only if Claude state exposes them separately; otherwise report `capabilities.children=false`.

### Codex Adapter

Prefer Codex app-server over CLI scraping.

Map:

- Thread lifecycle -> session lifecycle.
- `thread/status/changed` -> `status.changed`.
- `turn/*` and `item/*` -> activity, current tool, and progress.
- Dynamic tool calls and server requests -> blocked or working depending on request type.
- Codex subagent/source metadata -> child session linkage where available.

### OpenCode Adapter

Use OpenCode server APIs:

- `GET /session`
- `GET /session/status`
- `GET /session/:id/children`
- `GET /global/event`

OpenCode is the best early proof point for child-session support because child sessions are documented.

### Chorus Adapter

Use public daemon/MCP APIs first:

- chat/run status -> root session.
- blocked chat -> `blocked`.
- terminal verdict -> `completed` or `failed`.
- reviewer/persona subprocesses -> child sessions only if exposed through public API or stable local DB schema.

Avoid depending on private database shape unless the project accepts an adapter contract.

### AgentDeck Adapter

Consume AgentDeck daemon state:

- `sessions_list` -> session discovery.
- `state_update` -> status and activity.
- `usage_update` -> usage metadata.

AgentDeck is a strong reference implementation for state-machine inference from PTY and hooks.

### Symphony Adapter

Treat each Symphony implementation run as a root session:

- work item ID -> session ID.
- workspace path -> `cwd` or `workspaceRoots`.
- run phase -> `status`.
- PR/proof-of-work -> `links` and `meta`.
- spawned agent process -> child session when visible.

## Tradeoffs: Protocol-First vs ACP Extension vs PTY-First

### Protocol-First With Adapters

Best default.

Pros:

- Clean, purpose-built status model.
- Easy for UIs to build against.
- Lets compliant agents integrate directly.
- Still supports existing tools through adapters.
- Avoids bending ACP into a role it was not designed for.

Cons:

- Requires a new protocol and adoption story.
- Early adapters may be uneven.

### ACP Extension

Good for ecosystem alignment, but incomplete as the main boundary.

Pros:

- Builds on existing editor-agent protocol momentum.
- Reuses JSON-RPC and session concepts.
- Could influence standard status extensions upstream.

Cons:

- ACP is client-to-agent, not global daemon-to-many-UIs.
- It does not currently solve cross-directory discovery across unrelated tools.
- Non-ACP tools still need adapters.

### PTY-First

Useful fallback, poor foundation.

Pros:

- Works with many existing CLIs immediately.
- Can infer status before official integration exists.

Cons:

- Fragile across UI updates, themes, language changes, terminal modes, and spinners.
- Hard to expose child sessions reliably.
- Easy to overfit to a single tool.

## V1 Acceptance Criteria

- A daemon starts and creates a Unix domain socket.
- A UI or CLI can call `sessions/list` and receive normalized session records.
- A UI or CLI can subscribe to events and update without polling.
- At least two structured adapters work, preferably OpenCode and Codex app-server or ACP.
- At least one file-backed adapter works, preferably Claude Agent View state.
- Child sessions are represented for OpenCode.
- Unknown or unsupported fields degrade explicitly via `capabilities` and `status: "unknown"` instead of guessing.
- The daemon never sends prompts, approves actions, or terminates agents in V1.

## Open Questions

- Should native agents register with the daemon, or should the daemon discover agent sockets/manifests?
- Should directory grouping use only `cwd`, or a stable workspace identity derived from VCS root?
- Should the protocol include privacy levels to hide prompts, summaries, file paths, or PR links?
- Should the daemon define an OpenTelemetry bridge, or leave telemetry out of scope?
- Should UI bridges expose network access, or should V1 remain local socket only?

## Recommended Next Step

Write a short protocol draft with JSON Schema and a tiny reference CLI:

```text
agent-activityd
agent-activity sessions
agent-activity watch
```

Then implement OpenCode and Claude adapters first:

- OpenCode proves structured server and child-session support.
- Claude proves compatibility with the most visible product analogue.

Add Codex app-server and ACP next.
