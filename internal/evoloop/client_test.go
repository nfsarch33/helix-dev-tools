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
	rollupWSL1 := rollupRow("r-wsl1-1", "wsl1", "2026-04-23", "2026-04-23T11:00:00.000000", 6, 5)
	rollupWSL1Old := rollupRow("r-wsl1-0", "wsl1", "2026-04-22", "2026-04-22T11:00:00.000000", 2, 1)
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

	allFleetRows := []map[string]any{
		rollupMacbook,
		rollupWSL1,
		rollupWSL1Old,
		cycleMacbook,
		nonEvoloopRow,
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
			wantIDs: []string{"r-wsl1-1", "r-mac-1", "r-wsl1-0"},
			validate: func(t *testing.T, cap *captured, got []Capsule) {
				if len(cap.calls) != 1 {
					t.Fatalf("expected exactly 1 Mem0 call, got %d", len(cap.calls))
				}
				got0 := got[0]
				if got0.Cycles != 6 || got0.Improved != 5 || got0.LastKPI != 0.88 {
					t.Fatalf("rollup numeric fields not parsed: %+v", got0)
				}
				if got0.Day != "2026-04-23" || got0.Machine != "wsl1" {
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
			opts: RecentOptions{Limit: 3, Machine: "wsl1"},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			wantIDs: []string{"r-wsl1-1", "r-wsl1-0"},
			validate: func(t *testing.T, cap *captured, _ []Capsule) {
				if len(cap.calls) != 1 {
					t.Fatalf("expected exactly 1 Mem0 call, got %d", len(cap.calls))
				}
			},
		},
		{
			name: "kind=cycle returns only cycle capsules",
			opts: RecentOptions{Limit: 2, Kinds: []CapsuleKind{KindCycle}},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			wantIDs: []string{"c-mac-1"},
			validate: func(t *testing.T, cap *captured, got []Capsule) {
				if len(cap.calls) != 1 {
					t.Fatalf("expected 1 call, got %d", len(cap.calls))
				}
				c := got[0]
				if c.KPIBefore != 0.7 || c.KPIAfter != 0.82 || c.DurationMS != 1234 {
					t.Fatalf("cycle numeric fields not parsed: %+v", c)
				}
			},
		},
		{
			name: "kind=all merges rollups and cycles",
			opts: RecentOptions{Limit: 4, Kinds: []CapsuleKind{KindRollup, KindCycle}},
			respond: func(call capturedCall) (int, []byte) {
				raw, _ := json.Marshal(map[string]any{"results": allFleetRows})
				return http.StatusOK, raw
			},
			wantIDs: []string{"r-wsl1-1", "c-mac-1", "r-mac-1", "r-wsl1-0"},
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
			wantIDs: []string{"r-wsl1-1"},
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
			wantIDs: []string{"r-wsl1-1", "r-mac-1"},
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
