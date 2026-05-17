# Sprint v6025 — Dashboard QA — Hygiene KPI

## Date: 2026-05-17
## Sprint: v6025 (K3s Fleet Production — Pair 3 QA)

## QA Acceptance
- RBAC validator correctly rejects wildcard resources+verbs
- RBAC validator accepts read-only ClusterRole and namespaced Role
- NodePort validator enforces expected port 30043
- Port accessibility probe correctly times out on unreachable hosts
- ServiceAccount validator enforces namespace requirement

## Test Results
| Category | Count | Status |
|----------|-------|--------|
| QA unit tests | 5 | PASS |
| QA integration tests (gated) | 2 | SKIP |
| Full package total | 63 PASS + 10 SKIP |

## Edge Cases Covered
- [x] Read-only ClusterRole is valid
- [x] Wildcard admin ClusterRole is rejected
- [x] Role kind (namespaced) is accepted
- [x] Port probe timeout on RFC5737 test address
- [x] Dashboard namespace/service existence checks

## Quality Gates
- QA acceptance criteria met ✓
- Security posture validated (no cluster-admin for dashboard) ✓
- No regressions introduced ✓
- Build clean ✓
