// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop tests verify machine and source filters using the literal canonical labels

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
)

type fakeEvoloopClient struct {
	capsules  []evoloop.Capsule
	err       error
	lastOpts  evoloop.RecentOptions
	calls     int
	lastDebug io.Writer
}

func (f *fakeEvoloopClient) Recent(_ context.Context, opts evoloop.RecentOptions) ([]evoloop.Capsule, error) {
	f.calls++
	f.lastOpts = opts
	if f.err != nil {
		return nil, f.err
	}
	if f.lastDebug != nil {
		_, _ = io.WriteString(f.lastDebug, "fake-evoloop: Recent called\n")
	}
	return f.capsules, nil
}

func withEvoloopFactory(t *testing.T, client *fakeEvoloopClient, factoryErr error) {
	t.Helper()
	orig := evoloopFactory
	evoloopFactory = func(_ config.Paths, debug io.Writer) (evoloopClient, error) {
		if factoryErr != nil {
			return nil, factoryErr
		}
		if client != nil {
			client.lastDebug = debug
		}
		return client, nil
	}
	t.Cleanup(func() { evoloopFactory = orig })
}

func resetEvoloopFlags() {
	evoloopRecentFlags.kind = "rollup"
	evoloopRecentFlags.machine = ""
	evoloopRecentFlags.limit = 10
	evoloopRecentFlags.json = false
	evoloopRecentFlags.debug = false
}

func runRecentForTest(t *testing.T, args []string) (string, error) {
	t.Helper()
	resetEvoloopFlags()

	// Dispatch via rootCmd so cobra actually resolves "evoloop recent" and
	// parses subcommand flags. Calling evoloopRecentCmd.Execute() directly
	// walks up to rootCmd but without the subcommand path, which makes cobra
	// print root help instead of routing our args. Tests that call this
	// helper are intentionally serial because we mutate global cobra state.
	rootCmd.SetArgs(append([]string{"evoloop", "recent"}, args...))
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

func TestParseEvoloopKinds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    []evoloop.CapsuleKind
		wantErr bool
	}{
		{"default rollup", "rollup", []evoloop.CapsuleKind{evoloop.KindRollup}, false},
		{"empty defaults to rollup", "", []evoloop.CapsuleKind{evoloop.KindRollup}, false},
		{"cycle", "cycle", []evoloop.CapsuleKind{evoloop.KindCycle}, false},
		{"cycles plural accepted", "cycles", []evoloop.CapsuleKind{evoloop.KindCycle}, false},
		{"all merges", "all", []evoloop.CapsuleKind{evoloop.KindRollup, evoloop.KindCycle}, false},
		{"both is synonym", "both", []evoloop.CapsuleKind{evoloop.KindRollup, evoloop.KindCycle}, false},
		{"case insensitive", "ROLLUP", []evoloop.CapsuleKind{evoloop.KindRollup}, false},
		{"invalid returns error", "nope", nil, true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseEvoloopKinds(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got none", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("kind[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestEvoloopRecent_PassesFlagsToClient(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory package-level var.
	tests := []struct {
		name        string
		args        []string
		wantKinds   []evoloop.CapsuleKind
		wantMachine string
		wantLimit   int
	}{
		{"defaults rollups limit 10", nil, []evoloop.CapsuleKind{evoloop.KindRollup}, "", 10},
		{"machine filter", []string{"--machine=wsl1"}, []evoloop.CapsuleKind{evoloop.KindRollup}, "wsl1", 10},
		{"kind=all merges", []string{"--kind=all"}, []evoloop.CapsuleKind{evoloop.KindRollup, evoloop.KindCycle}, "", 10},
		{"custom limit", []string{"--kind=cycle", "--limit=3"}, []evoloop.CapsuleKind{evoloop.KindCycle}, "", 3},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEvoloopClient{}
			withEvoloopFactory(t, fake, nil)
			if _, err := runRecentForTest(t, tc.args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fake.calls != 1 {
				t.Fatalf("expected 1 call, got %d", fake.calls)
			}
			if len(fake.lastOpts.Kinds) != len(tc.wantKinds) {
				t.Fatalf("kinds len = %d, want %d", len(fake.lastOpts.Kinds), len(tc.wantKinds))
			}
			for i, k := range tc.wantKinds {
				if fake.lastOpts.Kinds[i] != k {
					t.Errorf("Kinds[%d] = %q, want %q", i, fake.lastOpts.Kinds[i], k)
				}
			}
			if fake.lastOpts.Machine != tc.wantMachine {
				t.Errorf("Machine = %q, want %q", fake.lastOpts.Machine, tc.wantMachine)
			}
			if fake.lastOpts.Limit != tc.wantLimit {
				t.Errorf("Limit = %d, want %d", fake.lastOpts.Limit, tc.wantLimit)
			}
		})
	}
}

func TestEvoloopRecent_JSONOutput(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory.
	when := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)
	rollup := evoloop.Capsule{
		ID:        "r-1",
		Kind:      evoloop.KindRollup,
		Text:      "EvoLoop rollup 2026-04-23",
		Machine:   "wsl1",
		Day:       "2026-04-23",
		CreatedAt: when,
		Cycles:    6,
		Improved:  5,
		MeanDelta: 0.12,
		LastKPI:   0.88,
	}
	fake := &fakeEvoloopClient{capsules: []evoloop.Capsule{rollup}}
	withEvoloopFactory(t, fake, nil)

	out, err := runRecentForTest(t, []string{"--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []evoloop.Capsule
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\n--\n%s", err, out)
	}
	if len(got) != 1 || got[0].ID != "r-1" || got[0].Machine != "wsl1" {
		t.Fatalf("unexpected parsed capsule: %+v", got)
	}
}

func TestEvoloopRecent_TableOutput(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory.
	when := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)
	rollup := evoloop.Capsule{
		ID: "r-1", Kind: evoloop.KindRollup, Machine: "wsl1",
		Day: "2026-04-23", CreatedAt: when,
		Cycles: 6, Improved: 5, RolledBack: 0, MeanDelta: 0.12, LastKPI: 0.88,
	}
	cycle := evoloop.Capsule{
		ID: "c-1", Kind: evoloop.KindCycle, Machine: "macbook",
		CycleID: "cyc-1", CreatedAt: when,
		KPIBefore: 0.70, KPIAfter: 0.82, DurationMS: 1234,
	}

	tests := []struct {
		name     string
		args     []string
		capsules []evoloop.Capsule
		wantIn   []string
		wantNot  []string
	}{
		{
			name:     "rollup table",
			args:     nil,
			capsules: []evoloop.Capsule{rollup},
			wantIn:   []string{"Capsules: 1", "wsl1", "2026-04-23", "cycles=6", "improved=5", "mean_delta=+0.120", "last_kpi=0.880", "R ["},
		},
		{
			name:     "cycle table shows KPI delta",
			args:     []string{"--kind=cycle"},
			capsules: []evoloop.Capsule{cycle},
			wantIn:   []string{"cycle=cyc-1", "0.700", "0.820", "+0.120", "duration_ms=1234", "C ["},
		},
		{
			name:     "machine filter reflected in header",
			args:     []string{"--kind=all", "--machine=wsl1"},
			capsules: []evoloop.Capsule{rollup},
			wantIn:   []string{"machine=wsl1", "kind=evoloop_rollup,evoloop_cycle"},
		},
		{
			name:     "empty result prints info with no capsules rendered",
			args:     nil,
			capsules: nil,
			wantIn:   []string{"no EvoLoop capsules"},
			wantNot:  []string{"wsl1", "cycles="},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Serial: shared cobra state.
			fake := &fakeEvoloopClient{capsules: tc.capsules}
			withEvoloopFactory(t, fake, nil)
			out, err := runRecentForTest(t, tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, needle := range tc.wantIn {
				if !strings.Contains(out, needle) {
					t.Errorf("output missing %q\n--- output ---\n%s", needle, out)
				}
			}
			for _, needle := range tc.wantNot {
				if strings.Contains(out, needle) {
					t.Errorf("output unexpectedly contains %q\n--- output ---\n%s", needle, out)
				}
			}
		})
	}
}

func TestEvoloopRecent_InvalidFlags(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory.
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"bad kind", []string{"--kind=foo"}, "unknown --kind"},
		{"zero limit", []string{"--limit=0"}, "--limit must be >= 1"},
		{"negative limit", []string{"--limit=-4"}, "--limit must be >= 1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Serial: shared cobra state.
			withEvoloopFactory(t, &fakeEvoloopClient{}, nil)
			_, err := runRecentForTest(t, tc.args)
			if err == nil {
				t.Fatalf("expected error, got none")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestEvoloopRecent_FactoryErrorPropagates(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory.
	withEvoloopFactory(t, nil, errors.New("no credentials"))
	_, err := runRecentForTest(t, nil)
	if err == nil {
		t.Fatalf("expected factory error to propagate")
	}
	if !strings.Contains(err.Error(), "no credentials") {
		t.Errorf("error %q missing factory error", err)
	}
}

func TestEvoloopRecent_DebugFlagWiresStderr(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory.
	fake := &fakeEvoloopClient{}
	withEvoloopFactory(t, fake, nil)
	if _, err := runRecentForTest(t, []string{"--debug"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastDebug == nil {
		t.Fatalf("--debug should wire a non-nil writer to the client")
	}
}

func TestEvoloopRecent_NoDebugDefaultsToNil(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory.
	fake := &fakeEvoloopClient{}
	withEvoloopFactory(t, fake, nil)
	if _, err := runRecentForTest(t, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastDebug != nil {
		t.Fatalf("default invocation must not wire a debug writer")
	}
}

func TestEvoloopRecent_ClientErrorPropagates(t *testing.T) {
	// Serial: mutates cobra command state + evoloopFactory.
	fake := &fakeEvoloopClient{err: errors.New("mem0 unavailable")}
	withEvoloopFactory(t, fake, nil)
	_, err := runRecentForTest(t, nil)
	if err == nil {
		t.Fatalf("expected client error to propagate")
	}
	if !strings.Contains(err.Error(), "mem0 unavailable") {
		t.Errorf("error %q missing client error", err)
	}
}

func TestEvoloopCmdRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Use == "evoloop" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evoloop command not registered on rootCmd")
	}
}

func TestEvoloopSubcommandsRegistered(t *testing.T) {
	want := map[string]bool{"recent": true}
	for _, c := range evoloopCmd.Commands() {
		delete(want, c.Name())
	}
	if len(want) > 0 {
		t.Fatalf("missing subcommands: %v", want)
	}
}

func TestCapsuleIcon(t *testing.T) {
	t.Parallel()
	if capsuleIcon(evoloop.KindRollup) != "R" {
		t.Fatalf("rollup icon")
	}
	if capsuleIcon(evoloop.KindCycle) != "C" {
		t.Fatalf("cycle icon")
	}
	if capsuleIcon(evoloop.CapsuleKind("weird")) != "?" {
		t.Fatalf("unknown icon")
	}
}

func TestFormatCapsuleTimestamp(t *testing.T) {
	t.Parallel()
	if got := formatCapsuleTimestamp(time.Time{}); got != "--" {
		t.Fatalf("zero time: %q", got)
	}
	when := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)
	if got := formatCapsuleTimestamp(when); got != "2026-04-23 11:00Z" {
		t.Fatalf("formatted: %q", got)
	}
}
