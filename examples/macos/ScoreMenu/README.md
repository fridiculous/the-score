# ScoreMenu

ScoreMenu is a native macOS menu-bar example for The Score. It is read-only and
uses only the local `scored` JSON-RPC API over the Score socket. It does not scan
processes, read terminals, or inspect source integrations directly.

Run the daemon first:

```bash
score start
```

Then run the example:

```bash
cd examples/macos/ScoreMenu
swift run ScoreMenu
```

The popover shows daemon reachability, version/API compatibility, storage path,
active sessions grouped by source/status/workspace, confidence, and source
diagnostics. If the daemon is unreachable it shows the exact start command.
