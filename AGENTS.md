# Repository Guidance

This repo builds The Score: a headless local observability daemon for coding-work
sessions, processes/runtimes, workspaces, lineage, sources, and metadata events.

## Product Boundary

- Keep the core API-first. Do not add a built-in UI/TUI unless the project
  direction explicitly changes.
- Treat `score` as the CLI client and `scored` as the daemon.
- Keep the CLI thin. It should call the daemon API rather than scanning files,
  processes, or source integrations directly.
- Prefer user-facing nouns: sessions, processes, workspaces, lineage, events,
  sources, and history.
- Treat agent names such as Claude, Codex, OpenCode, and Hermes as source or
  session metadata, not as the core product abstraction.
- Store metadata by default. Do not persist prompts, full transcripts, or
  scraped terminal output unless a future change adds explicit opt-in support.

## Engineering Defaults

- Keep the Go code boring and stdlib-heavy.
- Preserve cross-platform support: macOS/Linux use Unix sockets; Windows uses a
  named pipe.
- Put daemon behavior behind internal packages. Keep public package surface
  minimal until an SDK is intentionally designed.
- Every source/integration should report provenance and confidence.
- Unsupported source versions should degrade with diagnostics, not crash the
  daemon.

## Commit Rules

Use Conventional Commits for every commit:

```text
<type>(<scope>): <summary>
```

Examples:

```text
feat(api): add lineage resource
fix(ipc): close stale unix socket before listen
docs(release): document stable and rc tracks
test(store): cover child session history
```

Use scopes that match stable repo areas: `api`, `cli`, `daemon`, `ipc`,
`model`, `runtime`, `sources`, `store`, `docs`, `release`, `deps`, or `ci`.

Before committing, run:

```bash
gofmt -w cmd internal
go test ./...
```
