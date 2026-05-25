// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop tests verify machine and source filters using the literal canonical labels

package evoloop

import (
	"strings"
	"testing"
	"time"
)

func TestIsPromotionCandidate(t *testing.T) {
	rollup := func(machine string, improved, rolled int, mean float64) Capsule {
		return Capsule{
			ID:         "c-" + machine,
			Kind:       KindRollup,
			Machine:    machine,
			Day:        "2026-06-11",
			Improved:   improved,
			RolledBack: rolled,
			MeanDelta:  mean,
		}
	}
	cycle := Capsule{Kind: KindCycle, Machine: "test-host-1"}

	cases := []struct {
		name     string
		capsule  Capsule
		crit     PromotionCriteria
		expected bool
	}{
		{"rollup with improvement passes", rollup("test-host-1", 3, 1, 0.05), DefaultPromotionCriteria(), true},
		{"rollup with no improvement is rejected", rollup("test-host-1", 0, 0, 0.05), DefaultPromotionCriteria(), false},
		{"rollup with negative mean delta is rejected", rollup("test-host-1", 3, 0, -0.01), DefaultPromotionCriteria(), false},
		{"cycle capsule never promoted", cycle, DefaultPromotionCriteria(), false},
		{"machine filter accepts match", rollup("test-host-1", 1, 0, 0.1), PromotionCriteria{MinImproved: 1, OnlyMachines: []string{"test-host-1"}}, true},
		{"machine filter rejects non-match", rollup("macbook", 1, 0, 0.1), PromotionCriteria{MinImproved: 1, OnlyMachines: []string{"test-host-1"}}, false},
		{"raised mean threshold filters out small wins", rollup("test-host-1", 1, 0, 0.005), PromotionCriteria{MinImproved: 1, MinMeanDelta: 0.01}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsPromotionCandidate(tc.capsule, tc.crit)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestRollingKPIWindow(t *testing.T) {
	t0 := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	rollups := []Capsule{
		{Kind: KindRollup, Machine: "test-host-1", LastKPI: 0.50, CreatedAt: t0.Add(-2 * time.Hour)},
		{Kind: KindRollup, Machine: "test-host-1", LastKPI: 0.55, CreatedAt: t0.Add(-1 * time.Hour)},
		{Kind: KindRollup, Machine: "test-host-1", LastKPI: 0.60, CreatedAt: t0},
		{Kind: KindRollup, Machine: "test-host-1", LastKPI: 0.20, CreatedAt: t0.Add(-48 * time.Hour)}, // outside window
		{Kind: KindRollup, Machine: "macbook", LastKPI: 0.99, CreatedAt: t0},                   // wrong machine
		{Kind: KindCycle, Machine: "test-host-1", LastKPI: 0.99, CreatedAt: t0},                       // not a rollup
	}

	from := t0.Add(-24 * time.Hour)
	to := t0
	w := RollingKPIWindow(rollups, "test-host-1", from, to)

	if w.Samples != 3 {
		t.Fatalf("samples: expected 3, got %d", w.Samples)
	}
	if got, want := w.Mean, 0.55; absDiff(got, want) > 1e-9 {
		t.Fatalf("mean: expected %v, got %v", want, got)
	}
	if w.Stdev <= 0 {
		t.Fatalf("stdev: expected positive, got %v", w.Stdev)
	}
	if got, want := w.Latest, 0.60; absDiff(got, want) > 1e-9 {
		t.Fatalf("latest: expected %v, got %v", want, got)
	}
}

func TestRollingKPIWindowSparseReturnsZeros(t *testing.T) {
	t0 := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	w := RollingKPIWindow([]Capsule{
		{Kind: KindRollup, Machine: "test-host-1", LastKPI: 0.5, CreatedAt: t0},
	}, "test-host-1", t0.Add(-time.Hour), t0.Add(time.Hour))
	if w.Samples != 0 || w.Mean != 0 || w.Stdev != 0 {
		t.Fatalf("sparse window must return zero stats, got %+v", w)
	}
}

func TestIsRegression(t *testing.T) {
	t.Run("no regression when latest is at the mean", func(t *testing.T) {
		w := RegressionWindow{Samples: 3, Mean: 0.50, Stdev: 0.05, Latest: 0.50}
		if IsRegression(w, 1.0) {
			t.Fatal("equal latest should not trigger regression")
		}
	})
	t.Run("regression when latest is mean - 1.5 sigma", func(t *testing.T) {
		w := RegressionWindow{Samples: 4, Mean: 0.60, Stdev: 0.04, Latest: 0.54}
		if !IsRegression(w, 1.0) {
			t.Fatal("1.5 sigma drop should trigger regression at sigma=1")
		}
	})
	t.Run("flat history flags strict drop", func(t *testing.T) {
		w := RegressionWindow{Samples: 3, Mean: 0.70, Stdev: 0, Latest: 0.69}
		if !IsRegression(w, 1.0) {
			t.Fatal("flat history should regress on any strict drop")
		}
	})
	t.Run("flat history does not flag equality", func(t *testing.T) {
		w := RegressionWindow{Samples: 3, Mean: 0.70, Stdev: 0, Latest: 0.70}
		if IsRegression(w, 1.0) {
			t.Fatal("flat history must not regress on equality")
		}
	})
	t.Run("under-sampled window never regresses", func(t *testing.T) {
		w := RegressionWindow{Samples: 1, Mean: 0.50, Stdev: 0, Latest: 0.10}
		if IsRegression(w, 1.0) {
			t.Fatal("single sample must never regress")
		}
	})
}

func TestBuildPromotionCapsuleDeterministic(t *testing.T) {
	source := Capsule{
		ID:         "rollup-test-host-1-2026-06-11",
		Kind:       KindRollup,
		Machine:    "test-host-1",
		Day:        "2026-06-11",
		Improved:   3,
		RolledBack: 0,
		MeanDelta:  0.075,
		LastKPI:    0.612,
	}
	d := PromotionDecision{
		CapsuleID:    source.ID,
		Machine:      source.Machine,
		Day:          source.Day,
		Kind:         source.Kind,
		State:        PromotionStatePromoted,
		GateExitCode: 0,
	}
	now := func() time.Time { return time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC) }

	got := BuildPromotionCapsule(d, source, "test-operator-host", now)
	if got.AppID != "cursor-global-kb" {
		t.Fatalf("app_id: expected cursor-global-kb, got %q", got.AppID)
	}
	if got.UserID != "test-operator-host" {
		t.Fatalf("user_id: expected test-operator-host, got %q", got.UserID)
	}
	if !strings.Contains(got.Text, "evoloop promotion rollup-test-host-1-2026-06-11") {
		t.Fatalf("text: missing source id, got %q", got.Text)
	}
	if got.Metadata["kind"] != "evoloop_promotion" {
		t.Fatalf("metadata.kind: expected evoloop_promotion, got %q", got.Metadata["kind"])
	}
	if got.Metadata["source_capsule"] != source.ID {
		t.Fatalf("metadata.source_capsule mismatch: %q vs %q", got.Metadata["source_capsule"], source.ID)
	}
	if got.Metadata["machine"] != "test-host-1" {
		t.Fatalf("metadata.machine: %q", got.Metadata["machine"])
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("created_at must be set from now()")
	}

	again := BuildPromotionCapsule(d, source, "test-operator-host", now)
	if again.ID != got.ID {
		t.Fatalf("non-deterministic id: %q vs %q", again.ID, got.ID)
	}
}

func TestBuildRollbackCapsuleEncodesWindow(t *testing.T) {
	w := RegressionWindow{
		Machine: "test-host-1",
		From:    time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
		Samples: 6,
		Mean:    0.612,
		Stdev:   0.030,
		Latest:  0.560,
	}
	now := func() time.Time { return time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC) }
	cap := BuildRollbackCapsule("1234-evoloop-promotion-rollup-test-host-1", w, 1.0, "test-operator-host", now)
	if cap.Metadata["kind"] != "evoloop_rollback" {
		t.Fatalf("metadata.kind: expected evoloop_rollback, got %q", cap.Metadata["kind"])
	}
	if cap.Metadata["source_promotion"] == "" {
		t.Fatal("metadata.source_promotion must be populated")
	}
	if !strings.Contains(cap.Text, "machine=test-host-1") {
		t.Fatalf("text missing machine, got %q", cap.Text)
	}
	if cap.AppID != "cursor-global-kb" {
		t.Fatalf("app_id mismatch: %q", cap.AppID)
	}
	if cap.CreatedAt.IsZero() {
		t.Fatal("created_at must be set on rollback capsule")
	}
}

func TestAlreadyPromotedDetectsExistingCapsule(t *testing.T) {
	prior := []Capsule{
		{ID: "1", Metadata: map[string]string{"kind": "evoloop_promotion", "source_capsule": "rollup-test-host-1-2026-06-11"}},
		{ID: "2", Metadata: map[string]string{"kind": "evoloop_rollup"}},
	}
	if !AlreadyPromoted(prior, "rollup-test-host-1-2026-06-11") {
		t.Fatal("expected duplicate detection to fire on matching source_capsule")
	}
	if AlreadyPromoted(prior, "rollup-macbook-2026-06-11") {
		t.Fatal("non-matching source must not trigger duplicate detection")
	}
}

func TestFindPromotionForRollupReturnsLatest(t *testing.T) {
	t0 := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	prior := []Capsule{
		{
			ID:        "older",
			CreatedAt: t0.Add(-48 * time.Hour),
			Metadata: map[string]string{
				"kind":           "evoloop_promotion",
				"source_capsule": "rollup-test-host-1-2026-06-11",
			},
		},
		{
			ID:        "newer",
			CreatedAt: t0.Add(-1 * time.Hour),
			Metadata: map[string]string{
				"kind":           "evoloop_promotion",
				"source_capsule": "rollup-test-host-1-2026-06-11",
			},
		},
		{
			ID: "wrong-source",
			Metadata: map[string]string{
				"kind":           "evoloop_promotion",
				"source_capsule": "rollup-other-2026-06-10",
			},
		},
	}
	got := FindPromotionForRollup(prior, "rollup-test-host-1-2026-06-11")
	if got == nil {
		t.Fatal("expected to find promotion capsule")
	}
	if got.ID != "newer" {
		t.Fatalf("expected most recent capsule, got %q", got.ID)
	}
	if FindPromotionForRollup(prior, "rollup-nonexistent-2026") != nil {
		t.Fatal("expected nil for missing source")
	}
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
