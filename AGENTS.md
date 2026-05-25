# helix-dev-tools -- Agent Guidelines

- Repo: `https://github.com/nfsarch33/helix-dev-tools`
- **Purpose**: Single Go binary for Cursor IDE hooks, git hooks, health checks,
  sprint tooling, agent coordination, and memory system management.
- **Module**: `github.com/nfsarch33/helix-dev-tools`
- **Go**: 1.25.0+

## Build & Test

```bash
make build        # Build binary to bin/
make test         # Run all Ginkgo/Gomega tests with -race
make test-cover   # Run with coverage report
make lint         # go vet + staticcheck
make install      # Build + copy to ~/bin/
make release      # Cross-compile darwin-arm64 + linux-amd64
make docker       # Build Docker image
make clean        # Remove build artefacts
```

## Architecture

Single binary, zero runtime dependencies. All internal packages live under
`internal/` (Go compiler enforced: cannot be imported by external modules).

### Commands

| Command | Description |
|---------|-------------|
| `helix-dev-tools hook guard-shell` | beforeShellExecution: block dangerous commands |
| `helix-dev-tools hook sanitize-read` | beforeReadFile: block secret file reads |
| `helix-dev-tools hook guard-mcp` | beforeMCPExecution: gate destructive MCP tools |
| `helix-dev-tools hook post-edit` | afterFileEdit: format, sync counts, promote |
| `helix-dev-tools hook housekeeping` | stop: log rotation, git sync, promote |
| `helix-dev-tools githook commit-msg` | Reject AI attribution, enforce conventional commits |
| `helix-dev-tools githook pre-push` | Block direct pushes to main/master |
| `helix-dev-tools sync-counts` | Verify and fix skill/hook counts in index files |
| `helix-dev-tools promote` | Promote learnings through memory hierarchy |
| `helix-dev-tools health-check` | 33-suite integration health check |
| `helix-dev-tools docsync check` | Audit README/VERSION/CHANGELOG drift |
| `helix-dev-tools docsync fix` | Repair deterministic docs drift |
| `helix-dev-tools selftest` | Hook unit tests (94 assertions) |
| `helix-dev-tools memory-routine` | Export memory KPI and sync durable docs |
| `helix-dev-tools bootstrap` | Create all symlinks on a fresh machine |
| `helix-dev-tools version` | Print version, commit, build date |
| `helix-dev-tools sprint-dispatch` | Headless agent dispatch from kickoff handoff |
| `helix-dev-tools sprint-scaffold` | Emit 7-story sprint Markdown |
| `helix-dev-tools sprintboard-monitor` | Append status snapshot to NDJSON log |

### Key Internal Packages

| Package | Role |
|---------|------|
| `internal/cli` | Cobra command definitions |
| `internal/hookio` | Cursor hook JSON stdin/stdout protocol |
| `internal/patterns` | Pre-compiled regex deny/warn/allow engine |
| `internal/lockfile` | Cross-platform mkdir and flock locking |
| `internal/logger` | Structured JSONL logging with rotation |
| `internal/config` | Platform-aware path configuration |
| `internal/learnings` | Self-improvement pipeline (parse, merge, digest) |
| `internal/health` | Multi-suite health check runner |
| `internal/sprintboard` | Sprint lifecycle, ticket management |
| `internal/sprintgen` | Sprint scaffold generation |
| `internal/agentrace` | Agent trace NDJSON telemetry |
| `internal/workspace` | Workspace doctor and hygiene checks |
| `internal/eval` | Evaluation harness and runners |
| `internal/evoloop` | Self-improvement cycle integration |
| `internal/platform` | Platform detection and abstraction |
| `internal/k3svalidator` | Kubernetes manifest validation |
| `internal/metrics` | Prometheus metrics bridge |
| `internal/coordination` | Multi-agent coordination primitives |

### Additional Binaries

| Binary | Location | Purpose |
|--------|----------|---------|
| `helix-dashboard` | `cmd/helix-dashboard/` | Web dashboard server |
| `fleet-agent` | `cmd/fleet-agent/` | Fleet node agent |
| `mcp-proxy` | `cmd/mcp-proxy/` | MCP proxy server |

## Coding Conventions

- Go, strict typing. `go vet` + `go test -race` before commit.
- No secrets in committed files.
- Conventional commits: `type(scope): message`.
- Ginkgo/Gomega test framework.

## License

MIT
