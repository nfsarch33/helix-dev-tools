package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/evoloop"
	"github.com/nfsarch33/cursor-tools/internal/mem0outbox"
)

type fakePromoteClient struct {
	rollups []evoloop.Capsule
	history []evoloop.Capsule
	err     error
	calls   []evoloop.RecentOptions
}

func (f *fakePromoteClient) Recent(_ context.Context, opts evoloop.RecentOptions) ([]evoloop.Capsule, error) {
	f.calls = append(f.calls, opts)
	if f.err != nil {
		return nil, f.err
	}
	if containsKind(opts.Kinds, evoloop.KindPromotion) || containsKind(opts.Kinds, evoloop.KindRollback) {
		return f.history, nil
	}
	// Default rollup pass: honour --machine and --limit so tests can
	// verify flag plumbing without re-implementing the production filter.
	filtered := make([]evoloop.Capsule, 0, len(f.rollups))
	for _, c := range f.rollups {
		if opts.Machine != "" && c.Machine != opts.Machine {
			continue
		}
		filtered = append(filtered, c)
	}
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered, nil
}

func containsKind(set []evoloop.CapsuleKind, want evoloop.CapsuleKind) bool {
	for _, k := range set {
		if k == want {
			return true
		}
	}
	return false
}

type recordingWriter struct {
	caps   []mem0outbox.Capsule
	closed bool
	err    error
}

func (r *recordingWriter) Append(c mem0outbox.Capsule) error {
	if r.err != nil {
		return r.err
	}
	r.caps = append(r.caps, c)
	return nil
}

func (r *recordingWriter) Close() error {
	r.closed = true
	return nil
}

func resetEvoloopPromoteFlags() {
	evoloopPromoteFlags.auto = false
	evoloopPromoteFlags.machine = ""
	evoloopPromoteFlags.gateCmd = ""
	evoloopPromoteFlags.sigma = 1.0
	evoloopPromoteFlags.window = 24 * time.Hour
	evoloopPromoteFlags.limit = 20
	evoloopPromoteFlags.outbox = ""
	evoloopPromoteFlags.cursor = ""
	evoloopPromoteFlags.userID = ""
	evoloopPromoteFlags.json = false
	evoloopPromoteFlags.debug = false
}

func runPromoteForTest(t *testing.T, args []string) (string, error) {
	t.Helper()
	resetEvoloopPromoteFlags()
	rootCmd.SetArgs(append([]string{"evoloop", "promote"}, args...))
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	err := rootCmd.Execute()
	return buf.String(), err
}

type promoteFixture struct {
	client    *fakePromoteClient
	writer    *recordingWriter
	gateCalls *int
	gateExit  int
	gateErr   error
}

func withPromoteFixture(t *testing.T, fix promoteFixture) {
	t.Helper()
	deps := promotionDeps{
		clientFactory: func(_ config.Paths, _ io.Writer) (evoloopClient, error) {
			return fix.client, nil
		},
		resolveUserID: func(_ config.Paths) (string, error) {
			return "test-user", nil
		},
		gateRunner: func(_ string) evoloop.GateRunner {
			return func(_ context.Context, _ evoloop.Capsule) (evoloop.GateResult, error) {
				if fix.gateCalls != nil {
					*fix.gateCalls++
				}
				if fix.gateErr != nil {
					return evoloop.GateResult{ExitCode: -1}, fix.gateErr
				}
				return evoloop.GateResult{ExitCode: fix.gateExit}, nil
			}
		},
		openWriter: func(_ string) (writeCloser, error) {
			if fix.writer == nil {
				fix.writer = &recordingWriter{}
			}
			return fix.writer, nil
		},
		now: func() time.Time { return time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC) },
	}
	prev := promotionDepsOverride
	promotionDepsOverride = &deps
	t.Cleanup(func() { promotionDepsOverride = prev })
}

func TestEvoloopPromoteFlagValidation(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		contains string
	}{
		{"auto without gate refuses", []string{"--auto"}, "--auto requires --gate-cmd"},
		{"zero limit is rejected", []string{"--limit=0"}, "--limit must be >= 1"},
		{"zero sigma is rejected", []string{"--sigma=0"}, "--sigma must be > 0"},
		{"zero window is rejected", []string{"--window=0s"}, "--window must be > 0"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			withPromoteFixture(t, promoteFixture{
				client: &fakePromoteClient{},
				writer: &recordingWriter{},
			})
			_, err := runPromoteForTest(t, tc.args)
			if err == nil {
				t.Fatalf("expected error for %q", tc.name)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.contains)
			}
		})
	}
}

func TestEvoloopPromoteDryRunByDefault(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := evoloop.Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: evoloop.KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, LastKPI: 0.62, CreatedAt: now,
	}
	writer := &recordingWriter{}
	gateCalls := 0
	withPromoteFixture(t, promoteFixture{
		client:    &fakePromoteClient{rollups: []evoloop.Capsule{candidate}},
		writer:    writer,
		gateCalls: &gateCalls,
		gateExit:  0,
	})
	out, err := runPromoteForTest(t, []string{"--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["applied"].(bool) {
		t.Fatal("dry-run should report applied=false")
	}
	summary := got["summary"].(map[string]any)
	if summary["promoted"].(float64) != 1 {
		t.Fatalf("dry-run promoted: %+v", summary)
	}
	if len(writer.caps) != 0 {
		t.Fatalf("dry-run must not write outbox, got %d", len(writer.caps))
	}
}

func TestEvoloopPromoteAutoWritesOutbox(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := evoloop.Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: evoloop.KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, LastKPI: 0.62, CreatedAt: now,
	}
	writer := &recordingWriter{}
	gateCalls := 0
	withPromoteFixture(t, promoteFixture{
		client:    &fakePromoteClient{rollups: []evoloop.Capsule{candidate}},
		writer:    writer,
		gateCalls: &gateCalls,
		gateExit:  0,
	})
	out, err := runPromoteForTest(t, []string{
		"--auto",
		"--gate-cmd=true",
		"--user-id=jason-lian-macbook",
		"--outbox=/tmp/test-outbox.jsonl",
		"--json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gateCalls != 1 {
		t.Fatalf("expected gate to run once, got %d", gateCalls)
	}
	if len(writer.caps) != 1 {
		t.Fatalf("expected one outbox write, got %d", len(writer.caps))
	}
	if !writer.closed {
		t.Fatalf("writer must be closed after run")
	}
	got := writer.caps[0]
	if got.UserID != "jason-lian-macbook" {
		t.Fatalf("user_id flag override ignored, got %q", got.UserID)
	}
	if got.Metadata["kind"] != "evoloop_promotion" {
		t.Fatalf("metadata.kind: %q", got.Metadata["kind"])
	}
	if !strings.Contains(out, `"applied": true`) {
		t.Fatalf("expected applied=true in output, got\n%s", out)
	}
}

func TestEvoloopPromoteSkipsAlreadyPromoted(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := evoloop.Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: evoloop.KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, LastKPI: 0.62, CreatedAt: now,
	}
	prior := []evoloop.Capsule{
		{
			ID:        "earlier-promo",
			CreatedAt: now.Add(-1 * time.Hour),
			Metadata: map[string]string{
				"kind":           "evoloop_promotion",
				"source_capsule": candidate.ID,
			},
		},
	}
	writer := &recordingWriter{}
	gateCalls := 0
	withPromoteFixture(t, promoteFixture{
		client:    &fakePromoteClient{rollups: []evoloop.Capsule{candidate}, history: prior},
		writer:    writer,
		gateCalls: &gateCalls,
	})
	_, err := runPromoteForTest(t, []string{"--auto", "--gate-cmd=true", "--user-id=u"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gateCalls != 0 {
		t.Fatalf("gate must not run on already-promoted, got %d calls", gateCalls)
	}
	if len(writer.caps) != 0 {
		t.Fatalf("no writes expected, got %d", len(writer.caps))
	}
}

func TestEvoloopPromoteFailsWhenGateFails(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := evoloop.Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: evoloop.KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, LastKPI: 0.62, CreatedAt: now,
	}
	writer := &recordingWriter{}
	withPromoteFixture(t, promoteFixture{
		client:   &fakePromoteClient{rollups: []evoloop.Capsule{candidate}},
		writer:   writer,
		gateExit: 7,
	})
	out, err := runPromoteForTest(t, []string{"--auto", "--gate-cmd=false", "--user-id=u", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"failed": 1`) {
		t.Fatalf("expected one failed decision, got\n%s", out)
	}
	if len(writer.caps) != 0 {
		t.Fatalf("no writes expected on gate failure, got %d", len(writer.caps))
	}
}

func TestEvoloopPromoteEmitsRollback(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	rollups := []evoloop.Capsule{
		{ID: "r1", Kind: evoloop.KindRollup, Machine: "wsl1", LastKPI: 0.80, MeanDelta: 0.0, Improved: 0, CreatedAt: now.Add(-20 * time.Hour)},
		{ID: "r2", Kind: evoloop.KindRollup, Machine: "wsl1", LastKPI: 0.82, MeanDelta: 0.0, Improved: 0, CreatedAt: now.Add(-15 * time.Hour)},
		{ID: "r3", Kind: evoloop.KindRollup, Machine: "wsl1", LastKPI: 0.80, MeanDelta: 0.0, Improved: 0, CreatedAt: now.Add(-10 * time.Hour)},
		{ID: "r4", Kind: evoloop.KindRollup, Machine: "wsl1", LastKPI: 0.55, MeanDelta: 0.0, Improved: 0, CreatedAt: now.Add(-1 * time.Hour)},
	}
	prior := []evoloop.Capsule{
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
	writer := &recordingWriter{}
	withPromoteFixture(t, promoteFixture{
		client: &fakePromoteClient{rollups: rollups, history: prior},
		writer: writer,
	})
	out, err := runPromoteForTest(t, []string{"--auto", "--gate-cmd=true", "--user-id=u", "--sigma=1.0", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"rollbacks": 1`) {
		t.Fatalf("expected one rollback, got\n%s", out)
	}
	if len(writer.caps) != 1 {
		t.Fatalf("expected one outbox capsule, got %d", len(writer.caps))
	}
	if writer.caps[0].Metadata["kind"] != "evoloop_rollback" {
		t.Fatalf("metadata.kind: %q", writer.caps[0].Metadata["kind"])
	}
}

func TestEvoloopPromoteClientErrorPropagates(t *testing.T) {
	withPromoteFixture(t, promoteFixture{
		client: &fakePromoteClient{err: errors.New("mem0 unavailable")},
		writer: &recordingWriter{},
	})
	_, err := runPromoteForTest(t, nil)
	if err == nil {
		t.Fatalf("expected client error to propagate")
	}
	if !strings.Contains(err.Error(), "mem0 unavailable") {
		t.Errorf("error %q missing client error", err)
	}
}

func TestEvoloopPromoteSubcommandRegistered(t *testing.T) {
	want := map[string]bool{"promote": true}
	for _, c := range evoloopCmd.Commands() {
		delete(want, c.Name())
	}
	if len(want) > 0 {
		t.Fatalf("missing subcommands: %v", want)
	}
}

func TestEvoloopPromoteFiltersHistoryByKind(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	candidate := evoloop.Capsule{
		ID: "rollup-wsl1-2026-06-11", Kind: evoloop.KindRollup, Machine: "wsl1",
		Improved: 3, MeanDelta: 0.05, LastKPI: 0.62, CreatedAt: now,
	}
	prior := []evoloop.Capsule{
		// Promotion against a DIFFERENT source -- must not block this candidate.
		{ID: "p1", Metadata: map[string]string{"kind": "evoloop_promotion", "source_capsule": "different-rollup"}},
		// Unrelated kind -- must not block this candidate.
		{ID: "u1", Metadata: map[string]string{"kind": "evoloop_rollup", "source_capsule": candidate.ID}},
	}
	writer := &recordingWriter{}
	gateCalls := 0
	withPromoteFixture(t, promoteFixture{
		client:    &fakePromoteClient{rollups: []evoloop.Capsule{candidate}, history: prior},
		writer:    writer,
		gateCalls: &gateCalls,
	})
	_, err := runPromoteForTest(t, []string{"--auto", "--gate-cmd=true", "--user-id=u"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gateCalls != 1 {
		t.Fatalf("expected one gate call (history not relevant), got %d", gateCalls)
	}
	if len(writer.caps) != 1 {
		t.Fatalf("expected one promotion write, got %d", len(writer.caps))
	}
}
