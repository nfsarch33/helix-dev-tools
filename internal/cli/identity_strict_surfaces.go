package cli

import (
	"fmt"
	"sort"
	"strings"
)

// IdentityStrictSurface declares a single enforcement point for the
// strict identity gate. The plan requires four canonical surfaces; this
// type is the source of truth so the registry stays in sync with docs,
// tests, and the operator runbook.
type IdentityStrictSurface struct {
	ID          string // stable identifier (lowercase, hyphenated)
	Description string // human-readable summary
	Remediation string // suggested commands to validate / repair
}

var identityStrictSurfaces = []IdentityStrictSurface{
	{
		ID:          "wrapper",
		Description: "cursor-tools binary wrapper at ~/bin/cursor-tools (preflight before any subcommand on personal repos)",
		Remediation: "cursor-tools doctor identity --strict   # verify clean state",
	},
	{
		ID:          "daily-startup",
		Description: "cursor-tools daily-refresh verification step (runs at session start / cron)",
		Remediation: "cursor-tools daily-refresh --dry-run    # exercises the strict identity gate end-to-end\ncursor-tools doctor identity --strict   # standalone check",
	},
	{
		ID:          "pre-push",
		Description: "git pre-push hook (cursor-tools githook pre-push) — blocks personal-repo pushes when identity is poisoned",
		Remediation: "cursor-tools doctor identity --strict   # validate before every push",
	},
	{
		ID:          "ide-hook",
		Description: "Cursor IDE hooks.json beforeShellExecution chain — fast-fail when identity is leaked into a Cursor session",
		Remediation: "cursor-tools doctor identity --strict   # add to ~/.cursor/hooks.json beforeShellExecution chain",
	},
}

// IdentityStrictSurfaces returns the canonical 4-surface registry as a
// stable copy. The order is sorted-by-ID for deterministic diffability;
// callers expecting a specific surface should look it up by ID.
func IdentityStrictSurfaces() []IdentityStrictSurface {
	out := make([]IdentityStrictSurface, len(identityStrictSurfaces))
	copy(out, identityStrictSurfaces)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// runIdentityStrictGateWithState evaluates the strict gate against an
// injected state. Daily-refresh and the wrapper preflight call this so
// the same canonical evaluator runs everywhere.
func runIdentityStrictGateWithState(state identityGateState) error {
	failures := evaluateIdentityGateStrict(state)
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("identity strict gate failed:\n  - %s", strings.Join(failures, "\n  - "))
}
