# Contributing

## Commit Messages

The Score uses Conventional Commits so the project history stays readable and
can later drive changelog and release automation.

Format:

```text
<type>(<optional-scope>): <description>
```

Allowed types:

- `feat`: user-visible capability
- `fix`: bug fix
- `docs`: documentation-only change
- `test`: tests only
- `refactor`: behavior-preserving code change
- `perf`: performance improvement
- `build`: build, packaging, or dependency behavior
- `ci`: CI or release automation
- `chore`: maintenance with no product behavior change

Recommended scopes:

```text
api
cli
daemon
ipc
model
runtime
sources
store
docs
release
deps
ci
```

Breaking changes must use `!` and include a `BREAKING CHANGE:` footer:

```text
feat(api)!: rename sessions/list response fields

BREAKING CHANGE: sessions/list now returns resources under `items`.
```

## Checks

Before committing:

```bash
gofmt -w cmd internal
go test ./...
```

Before cutting a release, also cross-compile the CLI and daemon:

```bash
GOOS=linux GOARCH=amd64 go build ./cmd/score ./cmd/scored
GOOS=windows GOARCH=amd64 go build ./cmd/score ./cmd/scored
go build ./cmd/score ./cmd/scored
```

## Design Principles

- The daemon owns state; clients consume API snapshots and event streams.
- CLI commands are reference clients, not independent scanners.
- Sources/integrations are evidence providers. The store merges their facts into
  sessions, processes, workspaces, lineage, and events.
- Prefer explicit source diagnostics over guessing.
