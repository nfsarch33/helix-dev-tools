# launchd plists

Operator-applied artefacts for macOS background services backed by `cursor-tools` subcommands.

## com.user.mem0-watchdog.plist

Runs `cursor-tools mem0-watchdog` as a launchd background job. Probes
the configured Mem0 health endpoint every 60s; after 3 consecutive failures
it kills stale ssh forwards matching the default port pattern and resets the
counter. Logs to `~/logs/runx/mem0-watchdog.ndjson`.

### Install

```bash
mkdir -p ~/Library/LaunchAgents ~/logs/runx
cp deploy/launchd/com.user.mem0-watchdog.plist ~/Library/LaunchAgents/
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.user.mem0-watchdog.plist
launchctl enable gui/$(id -u)/com.user.mem0-watchdog
launchctl kickstart -p gui/$(id -u)/com.user.mem0-watchdog
```

### Verify

```bash
launchctl print gui/$(id -u)/com.user.mem0-watchdog | grep -E 'state|pid|last exit'
tail -f ~/logs/runx/mem0-watchdog.ndjson
```

### Uninstall

```bash
launchctl bootout gui/$(id -u)/com.user.mem0-watchdog
rm ~/Library/LaunchAgents/com.user.mem0-watchdog.plist
```

### Why a plist instead of a separate `~/runs/mem0-watchdog` binary

The watchdog logic is a `cursor-tools` subcommand
(`internal/cli/mem0_watchdog.go` + `internal/mem0watchdog/`), so reusing
the existing binary avoids duplicating build, packaging, and identity
discipline. The plist is the only deployment surface required.
