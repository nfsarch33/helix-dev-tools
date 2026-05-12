# EC v8 Pair 7 Docsync Automation Research

> Date: 2026-05-12  
> Branch: `feat/v8-p07-docsync-automation`  
> Scope: cursor-tools/runx documentation drift automation for README, VERSION, CHANGELOG, OpenAPI, release checklist, package metadata, and ADR index alignment.

## Sources Reviewed

- `cursor-tools:internal/docsync/audit.go`
- `cursor-tools:internal/docsync/audit_test.go`
- `cursor-tools:internal/cli/docsync.go`
- `cursor-tools:README.md`
- `runx docs --help`
- Backend release checklist: `agentic-ecommerce:docs/release-checklist.md`
- Frontend release checklist: `agentic-ecommerce-web:docs/release-checklist.md`
- v8 roadmap: `global-kb:backlog/ec-v8-10-pair-roadmap.md`

## Current Code Facts

- `cursor-tools docsync check|fix|report` already exists.
- `cursor-tools docs-check` is the compatibility wrapper used by runx and repo gates.
- `runx docs check|sync|report` already wraps cursor-tools.
- Existing docsync checks cover:
  - required README;
  - README mentions current version;
  - CHANGELOG mentions current version;
  - `api/openapi.yaml` `info.version` matches `VERSION`;
  - `package.json` version matches `VERSION`;
  - ADR index references ADR files;
  - public-repo required files when requested.
- Existing deterministic fixes cover README current-release lines, OpenAPI
  `info.version`, and ADR index regeneration.
- Release checklist drift is not currently checked or fixed.

## Decision

Implement the MVP in `cursor-tools` first because `runx docs` already delegates
to cursor-tools. This keeps the automation reusable across backend, frontend,
and future repos without adding a second implementation in runx.

The MVP adds:

1. `RELEASE_CHECKLIST_VERSION` audit finding when `docs/release-checklist.md`
   exists but does not mention the current repo version.
2. Deterministic `docsync fix` support for release checklist headings and common
   release/version references.
3. Tests that prove stale backend/frontend-style release checklists are caught
   and fixed.
4. README command description update to include release-checklist drift.

## RED Targets

1. `TestAuditRepoCatchesReleaseChecklistVersionDrift`
   - Fails until `AuditRepo` checks `docs/release-checklist.md`.
2. `TestFixRepoUpdatesReleaseChecklistVersion`
   - Fails until `FixRepo` can rewrite common release-checklist version text.
3. `TestDocsCheckAliasReportsReleaseChecklistDrift`
   - Fails until the existing CLI path surfaces the new finding via docs-check.

## Non-Goals

- Do not implement a hook installer in this MVP; Pair 7 QA can decide whether
  the hook is low-noise enough.
- Do not modify release metadata in backend/frontend repos in this branch.
- Do not add live network calls.

## Expected Validation

- Focused docsync tests pass.
- `go test ./internal/docsync ./internal/cli -count=1` passes or any unrelated
  suite failures are explicitly separated from focused evidence.
- `make test` if the repo is stable enough.
- `runx docs check --repo cursor-tools` or direct cursor-tools docs-check.
- `runx shell-leak-scan --repo cursor-tools`.
- Branch-local Sentrux gate.
