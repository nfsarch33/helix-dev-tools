// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop tests verify machine and source filters using the literal canonical labels

package evoloop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/mem0outbox"
)

type recorder struct {
	caps []mem0outbox.Capsule
	err  error
}

func (r *recorder) write(c mem0outbox.Capsule) error {
	if r.err != nil {
		return r.err
	}
	r.caps = append(r.caps, c)
	return nil
}

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestPromoteRunnerHappyPath(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := Capsule{
		ID:        "rollup-wsl1-2026-06-11",
		Kind:      KindRollup,
		Machine:   "wsl1",
		Day:       "2026-06-11",
		Improved:  3,
		MeanDelta: 0.075,
		LastKPI:   0.62,
		CreatedAt: now,
	}
	gateCalls := 0
	rec := &recorder{}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, c Capsule) (GateResult, error) {
			gateCalls++
			if c.ID != candidate.ID {
				t.Fatalf("gate received unexpected capsule %q", c.ID)
			}
			return GateResult{ExitCode: 0, Stdout: "ok"}, nil
		},
		Writer: rec.write,
		UserID: "jason-lian-macbook",
	}

	summary, err := runner.Run(context.Background(), PromoteOptions{
		Candidates: []Capsule{candidate},
		Rollups:    []Capsule{candidate},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Promoted != 1 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("summary: %+v", summary)
	}
	if gateCalls != 1 {
		t.Fatalf("gate calls: %d", gateCalls)
	}
	if len(rec.caps) != 1 {
		t.Fatalf("expected one outbox capsule, got %d", len(rec.caps))
	}
	got := rec.caps[0]
	if got.Metadata["kind"] != "evoloop_promotion" {
		t.Fatalf("metadata.kind: %q", got.Metadata["kind"])
	}
	if got.UserID != "jason-lian-macbook" {
		t.Fatalf("user id: %q", got.UserID)
	}
}

func TestPromoteRunnerSkipsNonCandidates(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	rec := &recorder{}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			t.Fatal("gate must not be called for skipped capsule")
			return GateResult{}, nil
		},
		Writer: rec.write,
		UserID: "jason-lian-macbook",
	}
	summary, err := runner.Run(context.Background(), PromoteOptions{
		Candidates: []Capsule{
			{ID: "no-improvement", Kind: KindRollup, Machine: "wsl1", Improved: 0, MeanDelta: 0.1, CreatedAt: now},
			{ID: "negative-mean", Kind: KindRollup, Machine: "wsl1", Improved: 4, MeanDelta: -0.05, CreatedAt: now},
			{ID: "cycle-not-rollup", Kind: KindCycle, Machine: "wsl1", Improved: 9, MeanDelta: 0.2, CreatedAt: now},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Skipped != 3 {
		t.Fatalf("expected 3 skipped, got %d (%+v)", summary.Skipped, summary)
	}
	if len(rec.caps) != 0 {
		t.Fatalf("no outbox writes expected, got %d", len(rec.caps))
	}
}

func TestPromoteRunnerSkipsAlreadyPromoted(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, CreatedAt: now,
	}
	prior := []Capsule{
		{ID: "earlier-promo", Metadata: map[string]string{
			"kind":           "evoloop_promotion",
			"source_capsule": candidate.ID,
		}},
	}
	rec := &recorder{}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			t.Fatal("gate must not run on already-promoted")
			return GateResult{}, nil
		},
		Writer: rec.write,
		UserID: "u",
	}
	summary, err := runner.Run(context.Background(), PromoteOptions{
		Candidates: []Capsule{candidate},
		History:    prior,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Promoted != 0 || summary.Skipped != 1 {
		t.Fatalf("summary: %+v", summary)
	}
}

func TestPromoteRunnerWithholdsOnGateFailure(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, CreatedAt: now,
	}
	rec := &recorder{}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			return GateResult{ExitCode: 7, Stderr: "tests failed"}, nil
		},
		Writer: rec.write,
		UserID: "u",
	}
	summary, err := runner.Run(context.Background(), PromoteOptions{Candidates: []Capsule{candidate}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Promoted != 0 || summary.Failed != 1 {
		t.Fatalf("summary: %+v", summary)
	}
	if got := summary.Decisions[0].State; got != PromotionStateGateFailed {
		t.Fatalf("state: %q", got)
	}
	if summary.Decisions[0].GateExitCode != 7 {
		t.Fatalf("gate_exit_code: %d", summary.Decisions[0].GateExitCode)
	}
	if len(rec.caps) != 0 {
		t.Fatalf("no writes on gate failure, got %d", len(rec.caps))
	}
}

func TestPromoteRunnerGateRunnerErrorIsFailure(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, CreatedAt: now,
	}
	rec := &recorder{}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			return GateResult{}, errors.New("could not exec gate")
		},
		Writer: rec.write,
		UserID: "u",
	}
	summary, err := runner.Run(context.Background(), PromoteOptions{Candidates: []Capsule{candidate}})
	if err != nil {
		t.Fatalf("gate runner error must surface as a failed decision, not a Run error: %v", err)
	}
	if summary.Failed != 1 {
		t.Fatalf("summary: %+v", summary)
	}
	if got := summary.Decisions[0].State; got != PromotionStateGateFailed {
		t.Fatalf("state: %q", got)
	}
	if len(rec.caps) != 0 {
		t.Fatalf("no writes when gate could not run, got %d", len(rec.caps))
	}
}

func TestPromoteRunnerRollbackOnRegression(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	rollups := []Capsule{
		{ID: "r1", Kind: KindRollup, Machine: "wsl1", LastKPI: 0.80, CreatedAt: now.Add(-20 * time.Hour)},
		{ID: "r2", Kind: KindRollup, Machine: "wsl1", LastKPI: 0.82, CreatedAt: now.Add(-15 * time.Hour)},
		{ID: "r3", Kind: KindRollup, Machine: "wsl1", LastKPI: 0.80, CreatedAt: now.Add(-10 * time.Hour)},
		{ID: "r4", Kind: KindRollup, Machine: "wsl1", LastKPI: 0.55, CreatedAt: now.Add(-1 * time.Hour)}, // big drop
	}
	prior := []Capsule{
		{
			ID:        "1234-evoloop-promotion-r4",
			CreatedAt: now.Add(-2 * time.Hour),
			Metadata: map[string]string{
				"kind":           "evoloop_promotion",
				"source_capsule": "r4",
				"machine":        "wsl1",
			},
		},
	}
	rec := &recorder{}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			return GateResult{ExitCode: 0}, nil
		},
		Writer: rec.write,
		UserID: "u",
	}
	summary, err := runner.Run(context.Background(), PromoteOptions{
		Candidates: nil,
		History:    prior,
		Rollups:    rollups,
		Window:     24 * time.Hour,
		Sigma:      1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Rollbacks != 1 {
		t.Fatalf("expected one rollback, got %+v", summary)
	}
	if len(rec.caps) != 1 {
		t.Fatalf("expected one outbox capsule, got %d", len(rec.caps))
	}
	got := rec.caps[0]
	if got.Metadata["kind"] != "evoloop_rollback" {
		t.Fatalf("metadata.kind: %q", got.Metadata["kind"])
	}
	if got.Metadata["source_promotion"] != "1234-evoloop-promotion-r4" {
		t.Fatalf("source_promotion: %q", got.Metadata["source_promotion"])
	}
}

func TestPromoteRunnerNoRollbackWithoutPriorPromotion(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	rollups := []Capsule{
		{ID: "r1", Kind: KindRollup, Machine: "wsl1", LastKPI: 0.80, CreatedAt: now.Add(-10 * time.Hour)},
		{ID: "r2", Kind: KindRollup, Machine: "wsl1", LastKPI: 0.82, CreatedAt: now.Add(-5 * time.Hour)},
		{ID: "r3", Kind: KindRollup, Machine: "wsl1", LastKPI: 0.30, CreatedAt: now.Add(-1 * time.Hour)},
	}
	rec := &recorder{}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			return GateResult{ExitCode: 0}, nil
		},
		Writer: rec.write,
		UserID: "u",
	}
	summary, err := runner.Run(context.Background(), PromoteOptions{
		Rollups: rollups,
		History: nil,
		Window:  24 * time.Hour,
		Sigma:   1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Rollbacks != 0 {
		t.Fatalf("rollback emitted with no prior promotion: %+v", summary)
	}
	if len(rec.caps) != 0 {
		t.Fatalf("expected zero outbox writes")
	}
}

func TestPromoteRunnerDryRunSkipsWriter(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, CreatedAt: now,
	}
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			return GateResult{ExitCode: 0}, nil
		},
		Writer: nil, // intentionally nil so the test would panic without DryRun
		DryRun: true,
		UserID: "u",
	}
	summary, err := runner.Run(context.Background(), PromoteOptions{Candidates: []Capsule{candidate}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Promoted != 1 {
		t.Fatalf("dry-run should still report decisions: %+v", summary)
	}
}

func TestPromoteRunnerRequiresWriterWhenNotDryRun(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, CreatedAt: now,
	}
	runner := &PromoteRunner{
		Now:    fixedNow(now),
		Gate:   func(_ context.Context, _ Capsule) (GateResult, error) { return GateResult{ExitCode: 0}, nil },
		Writer: nil,
		UserID: "u",
	}
	_, err := runner.Run(context.Background(), PromoteOptions{Candidates: []Capsule{candidate}})
	if err == nil {
		t.Fatal("expected an error when Writer is nil and DryRun is false")
	}
}

func TestPromoteRunnerHonoursContextCancellation(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := Capsule{
		ID: "rollup-wsl1", Kind: KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, CreatedAt: now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &PromoteRunner{
		Now: fixedNow(now),
		Gate: func(_ context.Context, _ Capsule) (GateResult, error) {
			t.Fatal("gate must not run when context is cancelled")
			return GateResult{}, nil
		},
		Writer: func(c mem0outbox.Capsule) error { t.Fatal("writer must not run when context is cancelled"); return nil },
		UserID: "u",
	}
	_, err := runner.Run(ctx, PromoteOptions{Candidates: []Capsule{candidate}})
	if err == nil {
		t.Fatal("expected context error to surface")
	}
}
