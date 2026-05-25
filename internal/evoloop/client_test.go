// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop tests verify machine and source filters using the literal canonical labels

package evoloop

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type captured struct {
	mu      sync.Mutex
	calls   []capturedCall
	handler func(call capturedCall) (int, []byte)
}

type capturedCall struct {
	Path    string
	Auth    string
	Payload map[string]any
}

func newFakeMem0(t *testing.T, handler func(call capturedCall) (int, []byte)) (*httptest.Server, *captured) {
	t.Helper()
	cap := &captured{handler: handler}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if len(body) > 0 {
			_ = json.Unmarshal(body, &payload)
		}
		call := capturedCall{
			Path:    r.URL.Path,
			Auth:    r.Header.Get("Authorization"),
			Payload: payload,
		}
		cap.mu.Lock()
		cap.calls = append(cap.calls, call)
		cap.mu.Unlock()

		status, body := http.StatusOK, []byte(`{"results":[]}`)
		if cap.handler != nil {
			status, body = cap.handler(call)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

func andFilters(call capturedCall) []map[string]any {
	filters, _ := call.Payload["filters"].(map[string]any)
	if filters == nil {
		return nil
	}
	raw, _ := filters["AND"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, m)
	}
	return out
}

func TestClient_Recent(t *testing.T) {
	t.Parallel()

	rollupRow := func(id, machine, day, ts string, cycles, improved int) map[string]any {
		return map[string]any{
			"id":         id,
			"memory":     "EvoLoop rollup " + day,
			"user_id":    "global",
			"app_id":     "cursor-global-kb",
			"created_at": ts,
			"metadata": map[string]any{
				"kind":        "evoloop_rollup",
				"machine":     machine,
				"day":         day,
				"cycles":      cycles,
				"improved":    improved,
				"rolled_back": 0,
				"incomplete":  0,
				"no_change":   0,
				"mean_delta":  0.12,
				"p50_delta":   0.1,
				"p95_delta":   0.2,
				"last_kpi":    0.88,
				"source":      "evoloop-daemon",
			},
		}
	}
	cycleRow := func(id, machine, cycleID, ts string, kpiBefore, kpiAfter float64, durationMS int64) map[string]any {
		return map[string]any{
			"id":         id,
			"memory":     "EvoLoop cycle " + cycleID,
			"user_id":    "global",
			"app_id":     "cursor-global-kb",
			"created_at": ts,
			"metadata": map[string]any{
				"kind":        "evoloop_cycle",
				"machine":     machine,
				"cycle_id":    cycleID,
				"kpi_before":  kpiBefore,
				"kpi_after":   kpiAfter,
				"duration_ms": durationMS,
				"source":      "evoloop-daemon",
			},
		}
	}

	rollupMacbook := rollupRow("r-mac-1", "macbook", "2026-04-23", "2026-04-23T10:00:00.000000", 4, 3)
	rollupWSL1 := rollupRow("r-test-host-1-1", "test-host-1", "2026-04-23", "2026-04-23T11:00:00.000000", 6, 5)
	rollupWSL1Old := rollupRow("r-test-host-1-0", "test-host-1", "2026-04-22", "2026-04-22T11:00:00.000000", 2, 1)
	cycleMacbook := cycleRow("c-mac-1", "macbook", "cyc-mac-1", "2026-04-23T10:05:00.000000", 0.7, 0.82, 1234)
	// capsule row with no kind metadata must be filtered out by default
	nonEvoloopRow := map[string]any{
		"id":         "cap-1",
		"memory":     "generic note",
		"user_id":    "global",
		"app_id":     "cursor-global-kb",
		"created_at": "2026-04-23T09:00:00.000000",
		"metadata": map[string]any{
			"kind": "capsule",
		},
	}
	// agent_outcome capsule from helixon-daemon's per-cycle hook. The
	// daemon writes these alongside the canonical kind=evoloop_cycle rows
	// for every loop iteration (including gated cycles that never reach
	// the cycle writer). The reader must surface these as synthetic
	// cycles when --kind=cycle is requested.
	cycleOutcomeWSL1 := map[string]any{
		"id":         "out-cyc-test-host-1-1",
		"memory":     "[helixon-daemon] cycle:ok on test-host-1: kpi=+0.04",
		"user_id":    "global",
		"app_id":     "cursor-global-kb",
		"created_at": "2026-04-23T11:30:00.000000",
		"metadata": map[string]any{
			"kind":        "agent_outcome",
			"actor":       "helixon-daemon",
			"machine":     "test-host-1",
			"event":       "helixon-daemon:cycle:ok",
			"cycle_id":    "cyc-test-host-1-99",
			"kpi_delta":   0.04,
			"duration_ms": 8200,
		},
	}
	// Legacy capsule shape: kind=capsule + source=evoloop-daemon. Older
	// daemon binaries (and any that downgrade temporarily) wrote cycles
	// with this shape; the reader must accept them when --kind=cycle is
	// requested so a fleet upgrade doesn't blind the reader.
	legacyCapsuleWSL1 := map[string]any{
		"id":         "cap-legacy-test-host-1-1",
		"memory":     "EvoLoop legacy cycle test-host-1",
		"user_id":    "global",
		"app_id":     "cursor-global-kb",
		"created_at": "2026-04-23T11:25:00.000000",
		"metadata": map[string]any{
			"kind":        "capsule",
			"source":      "evoloop-daemon",
			"machine":     "test-host-1",
			"cycle_id":    "cyc-test-host-1-98",
			"kpi_before":  0.50,
			"kpi_after":   0.55,
			"duration_ms": 7100,
		},
	}
	// agent_outcome from a non-cycle event must NOT be promoted to cycle.
	nonCycleOutcomeWSL1 := map[string]any{
		"id":         "out-fi-test-host-1-1",
		"memory":     "[helixon-daemon] feature_import:ok on test-host-1",
		"user_id":    "global",
		"app_id":     "cursor-global-kb",
		"created_at": "2026-04-23T11:35:00.000000",
		"metadata": map[string]any{
			"kind":    "agent_outcome",
			"actor":   "helixon-daemon",
			"machine": "test-host-1",
			"event":   "helixon-daemon:feature_import:ok",
		},
	}
	// Generic capsule (no source) must NOT be promoted to cycle even
	// when --kind=cycle is requested.
	plainCapsuleNote := map[string]any{
		"id":         "cap-misc-1",
		"memory":     "operator note",
		"user_id":    "global",
		"app_id":     "cursor-global-kb",
		"created_at": "2026-04-23T11:40:00.000000",
		"metadata": map[string]any{
			"kind":    "capsule",
			"machine": "test-host-1",
		},
	}

	allFleetRows := []map[string]any{
		rollupMacbook,
		rollupWSL1,
		rollupWSL1Old,
		cycleMacbook,
		nonEvoloopRow,
		cycleOutcomeWSL1,
		legacyCapsuleWSL1,
		nonCycleOutcomeWSL1,
		plainCapsuleNote,
	}

	tests := []struct {
		name     string
		opts     RecentOptions
		respond  func(call capturedCall) (int, []byte)
		wantIDs  []string
		wantErr  bool
		validate func(t *testing.T, cap *captured, got []Capsule)
	}{
		{
			name: "default returns rollups newest first, filters non-evoloop kinds",
			opts: RecentOptions{Limit: 5},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			wantIDs: []string{"r-test-host-1-1", "r-mac-1", "r-test-host-1-0"},
			validate: func(t *testing.T, cap *captured, got []Capsule) {
				if len(cap.calls) != 1 {
					t.Fatalf("expected exactly 1 Mem0 call, got %d", len(cap.calls))
				}
				got0 := got[0]
				if got0.Cycles != 6 || got0.Improved != 5 || got0.LastKPI != 0.88 {
					t.Fatalf("rollup numeric fields not parsed: %+v", got0)
				}
				if got0.Day != "2026-04-23" || got0.Machine != "test-host-1" {
					t.Fatalf("rollup labels not parsed: %+v", got0)
				}
				filters := andFilters(cap.calls[0])
				for _, f := range filters {
					for k := range f {
						if strings.HasPrefix(k, "metadata.") {
							t.Fatalf("metadata.* filter keys are rejected by Mem0 v2; got %q", k)
						}
					}
				}
				keys := map[string]bool{}
				for _, f := range filters {
					for k := range f {
						keys[k] = true
					}
				}
				if !keys["app_id"] || !keys["user_id"] {
					t.Fatalf("expected app_id+user_id filters, got %+v", filters)
				}
			},
		},
		{
			name: "machine filter applied client-side",
			opts: RecentOptions{Limit: 3, Machine: "test-host-1"},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			wantIDs: []string{"r-test-host-1-1", "r-test-host-1-0"},
			validate: func(t *testing.T, cap *captured, _ []Capsule) {
				if len(cap.calls) != 1 {
					t.Fatalf("expected exactly 1 Mem0 call, got %d", len(cap.calls))
				}
			},
		},
		{
			name: "kind=cycle returns canonical and synthetic cycle capsules",
			opts: RecentOptions{Limit: 10, Kinds: []CapsuleKind{KindCycle}},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			// newest first: outcome (11:30), legacy (11:25), canonical (10:05).
			// nonCycleOutcomeWSL1 (feature_import) and plainCapsuleNote
			// must not be promoted to cycle.
			wantIDs: []string{"out-cyc-test-host-1-1", "cap-legacy-test-host-1-1", "c-mac-1"},
			validate: func(t *testing.T, cap *captured, got []Capsule) {
				if len(cap.calls) != 1 {
					t.Fatalf("expected 1 call, got %d", len(cap.calls))
				}
				if len(got) < 3 {
					t.Fatalf("expected ≥3 cycle capsules (canonical + outcome + legacy), got %d: %+v", len(got), got)
				}
				for _, c := range got {
					if c.Kind != KindCycle {
						t.Fatalf("synthesised capsule must have Kind=cycle, got %q on %s", c.Kind, c.ID)
					}
				}
				// canonical row keeps kpi_before/kpi_after.
				var canonical, outcome, legacy *Capsule
				for i := range got {
					switch got[i].ID {
					case "c-mac-1":
						canonical = &got[i]
					case "out-cyc-test-host-1-1":
						outcome = &got[i]
					case "cap-legacy-test-host-1-1":
						legacy = &got[i]
					}
				}
				if canonical == nil || canonical.KPIBefore != 0.7 || canonical.KPIAfter != 0.82 || canonical.DurationMS != 1234 {
					t.Fatalf("canonical cycle numeric fields not parsed: %+v", canonical)
				}
				if outcome == nil {
					t.Fatalf("expected agent_outcome cycle to be surfaced as KindCycle")
				}
				if outcome.CycleID != "cyc-test-host-1-99" {
					t.Fatalf("outcome cycle_id not propagated: got %q", outcome.CycleID)
				}
				if outcome.KPIDelta == 0 {
					t.Fatalf("outcome kpi_delta not parsed: %+v", outcome)
				}
				if outcome.Event != "helixon-daemon:cycle:ok" {
					t.Fatalf("outcome event tag not preserved: %q", outcome.Event)
				}
				if outcome.DurationMS != 8200 {
					t.Fatalf("outcome duration not parsed: %d", outcome.DurationMS)
				}
				if legacy == nil || legacy.KPIBefore != 0.50 || legacy.KPIAfter != 0.55 {
					t.Fatalf("legacy capsule fields not parsed: %+v", legacy)
				}
				// non-cycle outcome and plain capsule must be excluded.
				for _, c := range got {
					if c.ID == "out-fi-test-host-1-1" || c.ID == "cap-misc-1" {
						t.Fatalf("non-cycle row leaked into kind=cycle results: %s", c.ID)
					}
				}
			},
		},
		{
			name: "kind=all merges rollups, canonical cycles, and synthetic cycles",
			opts: RecentOptions{Limit: 4, Kinds: []CapsuleKind{KindRollup, KindCycle}},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			// newest-first order:
			//   out-cyc-test-host-1-1 (synthetic cycle, 11:30)
			//   cap-legacy-test-host-1-1 (legacy cycle, 11:25)
			//   r-test-host-1-1 (rollup, 11:00)
			//   c-mac-1 (canonical cycle, 10:05)
			wantIDs: []string{"out-cyc-test-host-1-1", "cap-legacy-test-host-1-1", "r-test-host-1-1", "c-mac-1"},
			validate: func(t *testing.T, cap *captured, _ []Capsule) {
				if len(cap.calls) != 1 {
					t.Fatalf("expected 1 call regardless of kinds, got %d", len(cap.calls))
				}
			},
		},
		{
			name: "limit truncates after merge+sort",
			opts: RecentOptions{Limit: 1, Kinds: []CapsuleKind{KindRollup, KindCycle}},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			wantIDs: []string{"out-cyc-test-host-1-1"},
		},
		{
			name: "mem0 400 surfaces as error",
			opts: RecentOptions{Limit: 3},
			respond: func(call capturedCall) (int, []byte) {
				return http.StatusBadRequest, []byte(`{"detail":"bad filters"}`)
			},
			wantErr: true,
		},
		{
			name: "flat list response shape is accepted",
			opts: RecentOptions{Limit: 3},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal([]map[string]any{rollupMacbook, rollupWSL1})
				return http.StatusOK, raw
			},
			wantIDs: []string{"r-test-host-1-1", "r-mac-1"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, cap := newFakeMem0(t, tc.respond)
			client := NewClient("fake-key", "global", srv.URL, "")
			client.HTTP = srv.Client()

			got, err := client.Recent(context.Background(), tc.opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; capsules=%+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotIDs := make([]string, 0, len(got))
			for _, c := range got {
				gotIDs = append(gotIDs, c.ID)
			}
			if !equalStrings(gotIDs, tc.wantIDs) {
				t.Fatalf("want IDs %v, got %v", tc.wantIDs, gotIDs)
			}
			for _, call := range cap.calls {
				if call.Path != "/v2/memories/" {
					t.Errorf("expected /v2/memories/ path, got %q", call.Path)
				}
				if !strings.HasPrefix(call.Auth, "Token ") {
					t.Errorf("expected Token auth, got %q", call.Auth)
				}
			}
			if tc.validate != nil {
				tc.validate(t, cap, got)
			}
		})
	}
}

func TestClient_DebugWriter(t *testing.T) {
	t.Parallel()
	srv, cap := newFakeMem0(t, func(call capturedCall) (int, []byte) {
		return http.StatusOK, []byte(`{"results":[]}`)
	})
	client := NewClient("fake-key", "global", srv.URL, "")
	client.HTTP = srv.Client()
	var buf strings.Builder
	client.Debug = &buf

	if _, err := client.Recent(context.Background(), RecentOptions{Limit: 5, Machine: "test-host-1", Kinds: []CapsuleKind{KindCycle}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"POST /v2/memories/",
		`"app_id":"cursor-global-kb"`,
		`"user_id":"global"`,
		"machine=test-host-1",
		"kind=evoloop_cycle",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("debug output missing %q in %q", want, got)
		}
	}
	if len(cap.calls) != 1 {
		t.Fatalf("expected 1 Mem0 call, got %d", len(cap.calls))
	}
}

func TestCycleLikeRow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		meta map[string]string
		want bool
	}{
		{name: "canonical evoloop_cycle", meta: map[string]string{"kind": "evoloop_cycle"}, want: false}, // already kind=cycle
		{name: "agent_outcome cycle event", meta: map[string]string{"kind": "agent_outcome", "actor": "helixon-daemon", "event": "helixon-daemon:cycle:ok"}, want: true},
		{name: "agent_outcome non-cycle event", meta: map[string]string{"kind": "agent_outcome", "actor": "helixon-daemon", "event": "helixon-daemon:feature_import:ok"}, want: false},
		{name: "agent_outcome wrong actor", meta: map[string]string{"kind": "agent_outcome", "actor": "fleet-cli", "event": "helixon-daemon:cycle:ok"}, want: false},
		{name: "legacy capsule with evoloop-daemon source", meta: map[string]string{"kind": "capsule", "source": "evoloop-daemon"}, want: true},
		{name: "plain capsule no source", meta: map[string]string{"kind": "capsule"}, want: false},
		{name: "plain capsule wrong source", meta: map[string]string{"kind": "capsule", "source": "fleet-cli"}, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := cycleLikeRow(tc.meta); got != tc.want {
				t.Fatalf("cycleLikeRow(%v) = %v; want %v", tc.meta, got, tc.want)
			}
		})
	}
}

func TestRowToCapsule_LegacyEvoLoopCapsuleFallbacks(t *testing.T) {
	t.Parallel()

	got := rowToCapsule(mem0Row{
		ID:        "legacy-desktop",
		Memory:    "EvoLoop capsule sentinel-1777125097-19088 was added to machine DESKTOP-078M990 on 2026-04-25, with a duration of 1.780603 ms, improving KPI from 12.977925 to 12.98115, no rollback, five stages.",
		CreatedAt: "2026-04-25T13:51:45.000000",
		Metadata: map[string]any{
			"kind":       "capsule",
			"source":     "evoloop-daemon",
			"machine":    "DESKTOP-078M990",
			"capsule_id": "sentinel-1777125097-19088",
		},
	}, KindCycle)

	if got.CycleID != "sentinel-1777125097-19088" {
		t.Fatalf("CycleID = %q, want legacy capsule_id fallback", got.CycleID)
	}
	if got.KPIBefore != 12.977925 || got.KPIAfter != 12.98115 {
		t.Fatalf("legacy KPI values not parsed: before=%v after=%v", got.KPIBefore, got.KPIAfter)
	}
	if got.DurationMS != 1 {
		t.Fatalf("DurationMS = %d, want truncated millisecond value from legacy text", got.DurationMS)
	}
}

func TestParseMem0Time(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in    string
		valid bool
	}{
		{"2026-04-23T07:59:53.123456", true},
		{"2026-04-23T07:59:53Z", true},
		{"2026-04-23T07:59:53+00:00", true},
		{"not-a-time", false},
		{"", false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			_, err := parseMem0Time(tc.in)
			if tc.valid && err != nil {
				t.Fatalf("expected %q to parse, got %v", tc.in, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("expected %q to fail, got success", tc.in)
			}
		})
	}
}

func TestMetaStringCoversTypes(t *testing.T) {
	t.Parallel()
	if got := metaString("foo"); got != "foo" {
		t.Fatalf("string: %q", got)
	}
	if got := metaString(float64(3)); got != "3" {
		t.Fatalf("int-like float: %q", got)
	}
	if got := metaString(true); got != "true" {
		t.Fatalf("bool: %q", got)
	}
	if got := metaString(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
}

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()
	c := NewClient("k", "u", "", "")
	if c.BaseURL != DefaultBaseURL {
		t.Fatalf("baseURL default: %q", c.BaseURL)
	}
	if c.AppID != DefaultAppID {
		t.Fatalf("appID default: %q", c.AppID)
	}
	if c.HTTP == nil || c.HTTP.Timeout == 0 {
		t.Fatalf("http client not initialised: %+v", c.HTTP)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestClient_RecentHonoursContext(t *testing.T) {
	t.Parallel()
	srv, _ := newFakeMem0(t, func(call capturedCall) (int, []byte) {
		time.Sleep(200 * time.Millisecond)
		return http.StatusOK, []byte(`{"results":[]}`)
	})
	client := NewClient("k", "u", srv.URL, "")
	client.HTTP = srv.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.Recent(ctx, RecentOptions{Limit: 1}); err == nil {
		t.Fatalf("expected context timeout error")
	}
}
