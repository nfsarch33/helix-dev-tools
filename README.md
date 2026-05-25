# helix-dev-tools

Single Go binary for Cursor IDE hooks, git hooks, health checks, and memory system management.

- Module: `github.com/nfsarch33/helix-dev-tools`
- Binary: `helix-dev-tools` (backward-compat symlink `cursor-tools` available via `make install-compat`)
- Go: 1.25.0+

## Quick Start

```bash
make build install
helix-dev-tools version
```

## Commands

| Command | Description |
|---------|-------------|
| `helix-dev-tools hook guard-shell` | beforeShellExecution: block dangerous commands |
| `helix-dev-tools hook sanitize-read` | beforeReadFile: block secret file reads |
| `helix-dev-tools hook guard-mcp` | beforeMCPExecution: gate destructive MCP tools |
| `helix-dev-tools hook post-edit` | afterFileEdit: format, sync counts, promote |
| `helix-dev-tools hook housekeeping` | stop: log rotation, git sync, promote |
| `helix-dev-tools hook guard-no-shell-leak-sync` | beforeReadAgent: SHA-verify and resync no-shell-leak rule across 14 mirror repos (v299 D6) |
| `helix-dev-tools githook commit-msg` | Reject AI attribution, enforce conventional commits |
| `helix-dev-tools githook pre-push` | Block direct pushes to main/master |
| `helix-dev-tools sync-counts [--apply]` | Verify and fix skill/hook counts in index files |
| `helix-dev-tools promote [--workspace] [--dry-run]` | Promote learnings through memory hierarchy |
| `helix-dev-tools health-check` | 33-suite integration health check |
| `helix-dev-tools docsync check` | Audit README/VERSION/CHANGELOG/OpenAPI/release-checklist/ADR drift |
| `helix-dev-tools docsync fix` | Repair deterministic docs drift such as ADR indexes and version fields |
| `helix-dev-tools docs-check` | Backward-compatible wrapper for docs drift checks |
| `helix-dev-tools selftest` | Hook unit tests (94 assertions) |
| `helix-dev-tools memory-routine` | Export memory KPI and parity evidence, then optionally sync durable docs |
| `helix-dev-tools bootstrap [--dry-run]` | Create all symlinks on a fresh machine |
| `helix-dev-tools safe` | Launch Cursor with --disable-gpu |
| `helix-dev-tools version` | Print version, commit, build date |
| `helix-dev-tools sprint-dispatch` | Build headless Claude/Codex dispatch from a kickoff handoff (logs `~/logs/runx/agent-dispatch.ndjson`) |
| `helix-dev-tools sprintboard-monitor` | Append Sprintboard status snapshot to `~/logs/runx/sprintboard-monitor.ndjson` |
| `helix-dev-tools sprint-scaffold` | Emit 7-story sprint Markdown (5 themed + hygiene + capsule) per `sprint-scaffold-7-stories` rule |

### Overnight agent dispatch (v7100+)

`~/.cursor/hooks.json` wires **IDE lifecycle** hooks (`hook guard-shell`, `hook post-edit`, etc.).
It does **not** launch external agents. For copy-paste-free overnight runs, use
`sprint-dispatch` after writing a kickoff under your KB session-handoffs:

```bash
helix-dev-tools sprint-dispatch --agent codex \
  --kickoff <your-kb>/session-handoffs/<kickoff-file>.md \
  --sprint v7100

helix-dev-tools sprint-dispatch --agent claude-code \
  --kickoff <your-kb>/session-handoffs/<kickoff-file>.md \
  --sprint v7100
```

See the agent-dispatch-automation SOP in your knowledge base (Sprintboard MCP:
`agent_register` → `task_claim` → `task_complete`).

## Development

```bash
make test        # Run all Ginkgo/Gomega tests with -race
make test-cover  # Run with coverage report
make lint        # go vet + staticcheck
make build       # Build binary to bin/
make install     # Build + copy to ~/bin/
make release     # Cross-compile darwin-arm64 + linux-amd64
make docker      # Build Docker image
make clean       # Remove build artefacts
```

## Architecture

Single binary, zero runtime dependencies. All internal packages are in `/internal/`
(Go compiler enforced: cannot be imported by external modules).

Source lives in the `nfsarch33/helix-dev-tools` repository. A backward-compat symlink `cursor-tools -> helix-dev-tools` can be created via `make install-compat`.

- `internal/hookio` -- Cursor hook JSON stdin/stdout protocol
- `internal/patterns` -- Pre-compiled regex deny/warn/allow pattern engine
- `internal/lockfile` -- Cross-platform mkdir and flock locking
- `internal/logger` -- Structured JSONL logging with rotation
- `internal/config` -- Platform-aware path configuration
- `internal/learnings` -- Self-improvement pipeline (parse, merge, digest)
- `internal/health` -- multi-suite health check runner
- `internal/cli` -- Cobra command definitions

## Private Module Access

Some follow-up work imports private modules such as
`github.com/nfsarch33/offload-telemetry`. Use a process-local Git rewrite when
testing private module fetches from this MacBook; do not write permanent global
Git config for this.

```bash
# Replace <your-github-alias> with your local SSH host alias from ~/.ssh/config
GIT_CONFIG_COUNT=1 \
GIT_CONFIG_KEY_0=url.git@<your-github-alias>:.insteadOf \
GIT_CONFIG_VALUE_0=https://github.com/ \
GOPRIVATE=github.com/nfsarch33/* \
GONOSUMDB=github.com/nfsarch33/* \
GOPROXY=direct \
go test ./...
```

The current `tier-a` CLI emits the same redacted `offload.telemetry.v1` shape
locally. Import the shared module only after the private fetch path is green in
CI and local developer shells.

## Rollback

Both bash/Go versions coexist. To rollback any hook:

1. Edit `~/.cursor/hooks.json` to point back to `.sh` scripts
2. Old scripts remain in `cursor-config/hooks/` and `cursor-config/bin/`
3. Legacy reference: `cursor-config/legacy/README.md`
