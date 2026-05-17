# Sprint v6024 — K8s Dashboard + RBAC MVP — Hygiene KPI

## Date: 2026-05-17
## Sprint: v6024 (K3s Fleet Production — Pair 3)

## Research Findings
- kubernetes-dashboard namespace Active, deployment 0/1 (metrics-scraper 1/1)
- Dashboard currently ClusterIP (needs NodePort for external access)
- RBAC validation needed: prevent wildcard cluster-admin grants to dashboard

## Test Results
| Category | Count | Status |
|----------|-------|--------|
| RBAC validator tests | 5 | PASS |
| ServiceAccount validator tests | 4 | PASS |
| NodePort service validator tests | 5 | PASS |
| Port accessibility test | 1 | PASS |
| Total package tests | 63 PASS + 10 SKIP |

## Deliverables
- [x] `internal/k3svalidator/dashboard_manifest.go` — ValidateDashboardRBAC, ValidateServiceAccount, ValidateNodePortService, CheckPortAccessibility
- [x] `internal/k3svalidator/dashboard_manifest_test.go` — 15 unit tests (TDD RED→GREEN)
- [x] RBAC overly-permissive wildcard detection
- [x] NodePort expected-port validation
- [x] TCP port accessibility probe with timeout

## Quality Gates
- TDD discipline maintained ✓
- Security: wildcard RBAC rule detection ✓
- Build clean ✓
