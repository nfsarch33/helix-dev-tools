package evoloop

import (
	"context"
	"fmt"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/mem0outbox"
)

// GateResult is the structured outcome of one TDD gate invocation. The
// gate is a callable contract (e.g. shelling out to "go test -race
// ./...") rather than an inline function so the production CLI can
// drop the heavy os/exec dependency only at the leaves while the
// runner stays trivially testable.
type GateResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// GateRunner gates promotion of a single rollup capsule. Returning
// (result, nil) records the gate's structured outcome; returning a
// non-nil error means the gate could not be evaluated at all (process
// could not be spawned, transient infra error). Errors stop the run
// for that capsule and surface a "gate_failed" decision with the
// returned error message in Reason.
type GateRunner func(ctx context.Context, source Capsule) (GateResult, error)

// CapsuleWriter persists one outbox capsule. The runner calls this
// once per promotion and once per rollback. Production wires this to
// mem0outbox.Writer.Append; tests inject an in-memory recorder.
type CapsuleWriter func(c mem0outbox.Capsule) error

// PromoteRunner is the orchestrator that turns a slice of rollup
// candidates plus a TDD gate into outbox-bound promotion / rollback
// capsules. All non-determinism (clock, gate command, file writes) is
// injected so unit tests run in <1ms with zero IO.
type PromoteRunner struct {
	Now    func() time.Time
	Gate   GateRunner
	Writer CapsuleWriter
	UserID string
	// DryRun, when true, skips Writer entirely so an operator can
	// preview promotion decisions before mutating the outbox.
	DryRun bool
}

// PromoteOptions describes a single Run invocation.
type PromoteOptions struct {
	// Candidates are the rollup capsules pulled from Mem0 (or an
	// equivalent source) that we consider for promotion this run.
	Candidates []Capsule
	// History is a fleet-wide snapshot of prior promotion / rollback
	// capsules used for dedup and rollback correlation. Newer-first
	// or older-first is fine; AlreadyPromoted is order-insensitive.
	History []Capsule
	// Rollups is the rolling-window source for regression analysis.
	// Typically the same slice as Candidates, but tests can supply
	// a deeper history without polluting promotion decisions.
	Rollups []Capsule
	// Criteria tunes promotion eligibility. Zero value uses
	// DefaultPromotionCriteria.
	Criteria PromotionCriteria
	// Window is the duration looked back for KPI regression
	// analysis. Zero defaults to 24h.
	Window time.Duration
	// Sigma is the regression threshold; the latest KPI must fall
	// at least Sigma standard deviations below the rolling mean to
	// trigger a rollback. Zero defaults to 1.0.
	Sigma float64
}

// PromoteSummary aggregates the per-decision outcomes into a single
// structure the CLI can render or serialise as JSON.
type PromoteSummary struct {
	Decisions []PromotionDecision
	Promoted  int
	Skipped   int
	Failed    int
	Rollbacks int
}

// Run evaluates each candidate, gates promotion through r.Gate, and
// writes promotion/rollback capsules through r.Writer (unless DryRun
// is set). The returned summary is deterministic for a fixed input,
// gate, and clock.
func (r *PromoteRunner) Run(ctx context.Context, opts PromoteOptions) (PromoteSummary, error) {
	if r == nil {
		return PromoteSummary{}, fmt.Errorf("evoloop: nil PromoteRunner")
	}
	if r.Now == nil {
		r.Now = time.Now
	}
	criteria := opts.Criteria
	if criteria.MinImproved == 0 && criteria.MinMeanDelta == 0 && len(criteria.OnlyMachines) == 0 {
		criteria = DefaultPromotionCriteria()
	}
	window := opts.Window
	if window <= 0 {
		window = 24 * time.Hour
	}
	sigma := opts.Sigma
	if sigma <= 0 {
		sigma = 1.0
	}

	now := r.Now().UTC()
	from := now.Add(-window)

	summary := PromoteSummary{}
	for _, c := range opts.Candidates {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		decision := PromotionDecision{
			CapsuleID:  c.ID,
			Machine:    c.Machine,
			Day:        c.Day,
			Kind:       c.Kind,
			LastKPI:    c.LastKPI,
			MeanDelta:  c.MeanDelta,
			Improved:   c.Improved,
			RolledBack: c.RolledBack,
		}

		if !IsPromotionCandidate(c, criteria) {
			decision.State = PromotionStateSkipped
			decision.Reason = "not a promotion candidate"
			summary.Decisions = append(summary.Decisions, decision)
			summary.Skipped++
			continue
		}
		if AlreadyPromoted(opts.History, c.ID) {
			decision.State = PromotionStateSkipped
			decision.Reason = "already promoted"
			summary.Decisions = append(summary.Decisions, decision)
			summary.Skipped++
			continue
		}

		var gateResult GateResult
		if r.Gate != nil {
			res, err := r.Gate(ctx, c)
			if err != nil {
				decision.State = PromotionStateGateFailed
				decision.Reason = "gate runner error: " + err.Error()
				decision.GateExitCode = -1
				summary.Decisions = append(summary.Decisions, decision)
				summary.Failed++
				continue
			}
			gateResult = res
		}
		decision.GateExitCode = gateResult.ExitCode
		decision.GateStdout = truncate(gateResult.Stdout, 4096)
		decision.GateStderr = truncate(gateResult.Stderr, 4096)

		if gateResult.ExitCode != 0 {
			decision.State = PromotionStateGateFailed
			decision.Reason = fmt.Sprintf("gate command exited %d", gateResult.ExitCode)
			summary.Decisions = append(summary.Decisions, decision)
			summary.Failed++
			continue
		}

		decision.State = PromotionStatePromoted
		decision.Reason = "TDD gate green"
		summary.Decisions = append(summary.Decisions, decision)
		summary.Promoted++

		if r.DryRun {
			continue
		}
		if r.Writer == nil {
			return summary, fmt.Errorf("evoloop: PromoteRunner.Writer must be set when DryRun is false")
		}
		cap := BuildPromotionCapsule(decision, c, r.UserID, r.Now)
		if err := r.Writer(cap); err != nil {
			return summary, fmt.Errorf("evoloop: write promotion capsule for %s: %w", c.ID, err)
		}
	}

	// Rollback pass. Walks the rolling window per machine derived
	// from rollup candidates (deduped by machine) and emits one
	// rollback capsule when the latest KPI falls below the mean by
	// at least sigma standard deviations and there is a prior
	// promotion to invalidate.
	for _, machine := range distinctMachines(opts.Rollups) {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		w := RollingKPIWindow(opts.Rollups, machine, from, now)
		if !IsRegression(w, sigma) {
			continue
		}
		latestSourceID := latestRollupID(opts.Rollups, machine, from, now)
		promo := FindPromotionForRollup(opts.History, latestSourceID)
		if promo == nil {
			continue
		}
		rollback := BuildRollbackCapsule(promo.ID, w, sigma, r.UserID, r.Now)
		summary.Rollbacks++
		summary.Decisions = append(summary.Decisions, PromotionDecision{
			CapsuleID: latestSourceID,
			Machine:   machine,
			Kind:      KindRollup,
			State:     PromotionStateRolledBack,
			Reason:    fmt.Sprintf("KPI regression: latest=%.3f mean=%.3f stdev=%.3f sigma=%.2f", w.Latest, w.Mean, w.Stdev, sigma),
			LastKPI:   w.Latest,
		})
		if r.DryRun {
			continue
		}
		if r.Writer == nil {
			return summary, fmt.Errorf("evoloop: PromoteRunner.Writer must be set when DryRun is false")
		}
		if err := r.Writer(rollback); err != nil {
			return summary, fmt.Errorf("evoloop: write rollback capsule for %s: %w", promo.ID, err)
		}
	}
	return summary, nil
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func distinctMachines(caps []Capsule) []string {
	seen := make(map[string]struct{}, len(caps))
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		if c.Kind != KindRollup {
			continue
		}
		if c.Machine == "" {
			continue
		}
		if _, ok := seen[c.Machine]; ok {
			continue
		}
		seen[c.Machine] = struct{}{}
		out = append(out, c.Machine)
	}
	return out
}

func latestRollupID(caps []Capsule, machine string, from, to time.Time) string {
	var best Capsule
	for _, c := range caps {
		if c.Kind != KindRollup || c.Machine != machine {
			continue
		}
		if !c.CreatedAt.IsZero() && (c.CreatedAt.Before(from) || c.CreatedAt.After(to)) {
			continue
		}
		if c.CreatedAt.After(best.CreatedAt) {
			best = c
		}
	}
	return best.ID
}
