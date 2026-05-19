# Source Packs

Source Pack v0 is an in-repo Go contract for declaring how Score recognizes a
source. It is not an external plugin runtime. WASI or out-of-process plugins are
deferred until the core daemon API is stable enough to justify them.

## What A Source Pack Declares

A Source Pack v0 entry declares:

- source ID and display name
- source kind: `api`, `runtime`, `workspace`, `session`, or `protocol`
- supported command/process shapes
- capabilities it can report
- lifecycle evidence it can prove
- provenance of its evidence
- confidence rules
- version profiles and downgrade behavior
- diagnostics for unsupported or partial support
- fixture cases that prove expected detection behavior

Source packs should report metadata only by default. Do not add prompt capture,
full transcript storage, or scraped terminal output.

## Confidence Semantics

Process probes are baseline evidence, not truth.

- `low`: process shape matched, but no structured session state was observed.
- `medium`: multiple independent metadata signals agree.
- `high`: native observation API or structured source API reported state.
- `unknown`: source did not provide enough evidence.

When in doubt, return a lower confidence and include diagnostics.

## Lifecycle Semantics

Source packs declare lifecycle support separately from general capabilities:

```json
{
  "lifecycle": {
    "canDetectLiveness": true,
    "canDetectStart": false,
    "canDetectActivity": false,
    "canDetectWaiting": false,
    "canDetectTerminal": false
  }
}
```

Process-table source packs should normally declare liveness only. They can prove
that a matching process shape is present, but not whether the underlying agent is
working, idle, blocked, reviewable, or complete. Structured source APIs and
native observations can raise lifecycle support as their evidence improves.

## Fixture Rules

Every source-pack behavior change should add or update fixture cases. The
bundled process fixture corpus lives at:

```text
internal/runtime/testdata/source-pack-processes.json
```

The fixture file includes examples for Codex, Claude, Hermes, OpenCode, app
helper false positives, shell mention false positives, and launcher-backed CLIs.

Validate fixtures through the daemon:

```bash
score sources test-fixtures
score sources test-fixtures codex
```

The CLI calls `sources/testFixtures`; it does not inspect process shapes itself.

## Adding Support Safely

1. Add the source declaration in `internal/sources`.
2. Add process/source fixture cases before changing detection behavior.
3. Keep unsupported versions non-fatal and report diagnostics.
4. Preserve existing false-positive fixtures.
5. Run `gofmt -w cmd internal` and `go test ./...`.

Do not add a dependency on a source's private state files unless the source owner
documents them as stable or the diagnostics clearly mark the support as best
effort.
