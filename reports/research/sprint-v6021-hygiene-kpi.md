# Sprint v6021 — K3s Server QA — Hygiene KPI

## Date: 2026-05-17
## Sprint: v6021 (K3s Fleet Production — Pair 1 QA)

## QA Acceptance
- K3s status checker correctly parses real cluster output (2 nodes Ready, v1.35.4+k3s1)
- Version compatibility check validates same-version, 1-minor-skew, and rejects 2+ skew
- Kubeconfig validator catches missing server, empty config, and non-YAML

## Regression Test Results
| Category | Count | Status |
|----------|-------|--------|
| Regression tests | 9 | PASS |
| Dashboard probe tests | 2 | PASS |
| Total unit tests | 29 | PASS |
| Integration tests | 5 | SKIP (gated) |

## Edge Cases Covered
- [x] Multiple nodes with one NotReady
- [x] All nodes NotReady
- [x] 3-node version mismatch (2+ minor skew)
- [x] Version string without +k3s suffix
- [x] Single node always compatible
- [x] Multi-cluster kubeconfig
- [x] One cluster missing server in multi-cluster
- [x] Extra whitespace in node output
- [x] Older K3s version format

## Dashboard Health Probe
- [x] URL construction with custom port
- [x] Default port 30043 fallback
- [x] TLS-insecure check (self-signed cert support)
- [x] HTTP 5xx error detection

## Quality Gates
- TDD discipline maintained ✓
- No regressions found in existing tests ✓
- Build clean ✓
