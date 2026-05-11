# cursor-tools

Single Go binary for Cursor IDE hooks, git hooks, health checks, and memory system management.

## Quick Start

```bash
cd ~/cursor-tools
make build install
cursor-tools version
```

## Commands

| Command | Description |
|---------|-------------|
| `cursor-tools hook guard-shell` | beforeShellExecution: block dangerous commands |
| `cursor-tools hook sanitize-read` | beforeReadFile: block secret file reads |
| `cursor-tools hook guard-mcp` | beforeMCPExecution: gate destructive MCP tools |
| `cursor-tools hook post-edit` | afterFileEdit: format, sync counts, promote |
| `cursor-tools hook housekeeping` | stop: log rotation, git sync, promote |
| `cursor-tools hook guard-no-shell-leak-sync` | beforeReadAgent: SHA-verify and resync no-shell-leak rule across 14 mirror repos (v299 D6) |
| `cursor-tools githook commit-msg` | Reject AI attribution, enforce conventional commits |
| `cursor-tools githook pre-push` | Block direct pushes to main/master |
| `cursor-tools sync-counts [--apply]` | Verify and fix skill/hook counts in index files |
| `cursor-tools promote [--workspace] [--dry-run]` | Promote learnings through memory hierarchy |
| `cursor-tools health-check` | 33-suite integration health check |
| `cursor-tools docsync check` | Audit README/VERSION/CHANGELOG/OpenAPI/ADR drift |
| `cursor-tools docsync fix` | Repair deterministic docs drift such as ADR indexes and version fields |
| `cursor-tools docs-check` | Backward-compatible wrapper for docs drift checks |
| `cursor-tools selftest` | Hook unit tests (94 assertions) |
| `cursor-tools memory-routine` | Export memory KPI and parity evidence, then optionally sync durable docs |
| `cursor-tools bootstrap [--dry-run]` | Create all symlinks on a fresh machine |
| `cursor-tools safe` | Launch Cursor with --disable-gpu |
| `cursor-tools version` | Print version, commit, build date |

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

Source lives in the private `nfsarch33/cursor-tools` repository. `global-kb` keeps bootstrap pointers and installs a pinned release into `~/bin/cursor-tools`.

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
