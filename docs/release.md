# Release Model

The Score follows a CLI/daemon split inspired by tools like Tailscale:

- `score` is the user-facing CLI.
- `scored` is the local daemon.
- Both binaries are released from the same repo and should normally share one
  version.

## Versioning

Use semantic versioning for public releases:

```text
vMAJOR.MINOR.PATCH
```

Release impact:

- `feat`: eligible for a minor release.
- `fix`: eligible for a patch release.
- `BREAKING CHANGE` or `!`: major release after v1.0.0.
- `docs`, `test`, `refactor`, `chore`, and `ci`: no release by themselves
  unless they support a release task.

Before v1.0.0, breaking API changes may ship in minor versions, but release
notes must call them out explicitly.

## v0.0.1 Contract

`v0.0.1` is the first API contract release. It should prove:

- `score version`, `score version --daemon`, and `scored --version` report
  compatible versions.
- `daemon/info` includes daemon version, API version, source-pack version, build
  commit, process ID, start time, and SQLite storage path.
- SQLite persists sessions, workspaces, lineage edges, sources, and events.
- Source Pack v0 declarations and process fixtures ship in-repo.
- Release notes clearly state that the API is pre-1.0 but documented.

Homebrew packaging can wait until after `v0.0.1` unless it is already trivial.

## Tracks

Use three tracks once distribution exists:

- Stable: tagged releases intended for normal use.
- Release candidate: `vX.Y.Z-rc.N` tags used to test packaging, upgrades, and
  daemon/CLI compatibility.
- Development: commits on `main` with no stability promise.

Do not copy Tailscale's even/odd minor numbering unless the project adopts a
high-frequency release train. Keep Score's versioning conventional until there
is enough release volume to justify a special scheme.

## CLI And Daemon Compatibility

The CLI and daemon should report version information independently once version
embedding exists:

```bash
score version
score version --daemon
scored --version
```

Compatibility rules:

- A newer CLI should show a clear error when talking to an older daemon that
  lacks a required method.
- A newer daemon should continue serving older CLI read methods where practical.
- JSON-RPC method names are API surface. Renames require a compatibility shim or
  an explicit breaking release note.
- Add new fields without removing old fields when possible.

## Release Checklist

1. Confirm `main` is green:

   ```bash
   gofmt -w cmd internal
   go test ./...
   go build ./cmd/score ./cmd/scored
   GOOS=linux GOARCH=amd64 go build ./cmd/score ./cmd/scored
   GOOS=windows GOARCH=amd64 go build ./cmd/score ./cmd/scored
   ```

   Release builds should embed the commit:

   ```bash
   go build -ldflags "-X github.com/fridiculous/the-score/internal/version.BuildCommit=$(git rev-parse --short HEAD)" ./cmd/score ./cmd/scored
   ```

2. Review conventional commits since the previous tag and write release notes.
3. Tag the release:

   ```bash
   git tag -s vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

4. Publish binaries for at least:

   ```text
   darwin/arm64
   darwin/amd64
   linux/amd64
   linux/arm64
   windows/amd64
   ```

5. Confirm `score` can talk to `scored` for the same release version.
