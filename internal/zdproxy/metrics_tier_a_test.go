package zdproxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMetrics_TierAOffload_DumpExposition is an evidence-capture seam.
// When CURSOR_TIER_A_DUMP is set, the test renders a representative
// :9787-style exposition body and writes it to the path. The test is
// inert in normal runs (assertion-free).
func TestMetrics_TierAOffload_DumpExposition(t *testing.T) {
	dest := strings.TrimSpace(os.Getenv("CURSOR_TIER_A_DUMP"))
	if dest == "" {
		t.Skip("CURSOR_TIER_A_DUMP not set; skipping evidence dump")
	}
	m := NewMetrics()
	m.RecordTierAOffload("a", "offloaded", "claude_code_subagent", 1247*time.Millisecond)
	m.RecordTierAOffload("a", "offloaded", "claude_code_subagent", 982*time.Millisecond)
	m.RecordTierAOffload("a", "kept_local", "router_qwen36_27b", 28*time.Millisecond)
	m.RecordTierAOffload("a", "declined", "codex_subagent", 0)
	m.RecordRequest("openai_chat", "gpt-5.5", 200, 137*time.Millisecond)
	m.RecordTokens("gpt-5.5", "input", 16)

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if err := os.WriteFile(dest, rec.Body.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", dest, err)
	}
	t.Logf("wrote %d bytes to %s", rec.Body.Len(), dest)
}

// TestMetrics_TierAOffload_ExposesCounterAndHistogram drives the
// s1w1_tier_a_telemetry surface. The plan requires a counter and a
// histogram on the same :9787 scrape target as zd-claude-proxy. The
// metric family names are stable; label tuples are bounded; redaction
// rules forbid free-form payload labels (only tier/decision/route are
// emitted as labels).
func TestMetrics_TierAOffload_ExposesCounterAndHistogram(t *testing.T) {
	m := NewMetrics()

	m.RecordTierAOffload("a", "offloaded", "claude_code_subagent", 750*time.Millisecond)
	m.RecordTierAOffload("a", "offloaded", "claude_code_subagent", 1300*time.Millisecond)
	m.RecordTierAOffload("a", "kept_local", "router_qwen36_27b", 30*time.Millisecond)
	m.RecordTierAOffload("a", "declined", "codex_subagent", 0)

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()

	wantLines := []string{
		"# HELP zd_claude_proxy_tier_a_offload_total Total tier-A subagent offload decisions grouped by tier, decision and route.",
		"# TYPE zd_claude_proxy_tier_a_offload_total counter",
		`zd_claude_proxy_tier_a_offload_total{tier="a",decision="declined",route="codex_subagent"} 1`,
		`zd_claude_proxy_tier_a_offload_total{tier="a",decision="kept_local",route="router_qwen36_27b"} 1`,
		`zd_claude_proxy_tier_a_offload_total{tier="a",decision="offloaded",route="claude_code_subagent"} 2`,
		"# HELP zd_claude_proxy_tier_a_offload_latency_seconds Tier-A subagent offload wall-clock latency by tier and route.",
		"# TYPE zd_claude_proxy_tier_a_offload_latency_seconds histogram",
		`zd_claude_proxy_tier_a_offload_latency_seconds_bucket{tier="a",route="claude_code_subagent",le="1"} 1`,
		`zd_claude_proxy_tier_a_offload_latency_seconds_bucket{tier="a",route="claude_code_subagent",le="2.5"} 2`,
		`zd_claude_proxy_tier_a_offload_latency_seconds_count{tier="a",route="claude_code_subagent"} 2`,
	}
	for _, line := range wantLines {
		if !strings.Contains(body, line) {
			t.Fatalf("expected exposition to contain:\n  %s\n--- full body:\n%s", line, body)
		}
	}

	// Declined offloads have zero latency; the histogram must still record
	// the sample so we can compute decline-rate on the same series.
	if !strings.Contains(body, `zd_claude_proxy_tier_a_offload_latency_seconds_count{tier="a",route="codex_subagent"} 1`) {
		t.Fatalf("expected declined offload to record one histogram sample (zero latency)")
	}
}

// TestMetrics_TierAOffload_RejectsEmptyLabels guarantees that a caller
// supplying an empty tier/decision/route is dropped on the floor rather
// than emitted as an empty label (which would silently broaden the
// cardinality and bypass redaction). The recorder is forgiving: it is
// safe to call from hot paths without callers having to validate.
func TestMetrics_TierAOffload_RejectsEmptyLabels(t *testing.T) {
	m := NewMetrics()
	m.RecordTierAOffload("", "offloaded", "claude_code_subagent", time.Second)
	m.RecordTierAOffload("a", "", "claude_code_subagent", time.Second)
	m.RecordTierAOffload("a", "offloaded", "", time.Second)

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	if strings.Contains(body, "zd_claude_proxy_tier_a_offload_total{") {
		t.Fatalf("expected no tier-A series for empty-label inputs, got:\n%s", body)
	}
}

// TestMetrics_TierAOffload_LabelEscaping ensures pathological route
// values (newlines, quotes) cannot break the exposition format. The
// proxy never logs label values directly, so this only guards against
// hostile callers.
func TestMetrics_TierAOffload_LabelEscaping(t *testing.T) {
	m := NewMetrics()
	m.RecordTierAOffload("a", "offloaded", `route"with\quotes`+"\n", 100*time.Millisecond)

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `route="route\"with\\quotes\n"`) {
		t.Fatalf("expected escaped route label, got body:\n%s", body)
	}
}
