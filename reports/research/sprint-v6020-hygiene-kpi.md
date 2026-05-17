# Sprint v6020 — K3s Server Install MVP — Hygiene KPI

## Date: 2026-05-17
## Sprint: v6020 (K3s Fleet Production — Pair 1)

## Research Findings
- K3s v1.35.4+k3s1 already running on gpu-host-1 (control-plane) + gpu-host-2 (agent)
- Both nodes Ready, service active, k3d no longer required
- Existing `fleetk8s` package handles generic kubectl JSON parsing
- Kubeconfig at `/etc/rancher/k3s/k3s.yaml` on control-plane node
- kubectl default context pointed at stale k3d certs (fixed by using k3s kubeconfig)

## Test Results
| Category | Count | Status |
|----------|-------|--------|
| Unit tests | 18 | PASS |
| Integration tests | 5 | SKIP (gated) |
| Build | 1 | PASS |

## Deliverables
- [x] `internal/k3svalidator/validator.go` — ParseNodeStatus, ValidateAllReady, ParseK3sVersion, CheckVersionCompatibility, ValidateKubeconfig
- [x] `internal/k3svalidator/validator_test.go` — 18 unit tests (TDD RED→GREEN)
- [x] `internal/k3svalidator/integration_test.go` — 5 integration tests (gated by CURSOR_TOOLS_INTEGRATION=1)
- [x] `internal/cli/k3s_status.go` — CLI command: `cursor-tools k3s status`
- [x] `internal/cli/root.go` — k3sCmd registered

## Quality Gates
- TDD discipline: RED written first, GREEN implemented second ✓
- No secrets in code ✓
- No `git add -A` ✓
- Go-first tooling ✓
- Build clean ✓
