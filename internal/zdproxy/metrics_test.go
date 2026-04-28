package zdproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestMetrics_ExpositionFormat verifies the text exposition format and the
// per-series labels are stable. The proxy emits a small, fixed set of
// counters/histograms by design.
func TestMetrics_ExpositionFormat(t *testing.T) {
	m := NewMetrics()

	m.RecordRequest("openai_chat", "gpt-5.5", 200, 137*time.Millisecond)
	m.RecordRequest("openai_chat", "gpt-5.5", 200, 200*time.Millisecond)
	m.RecordRequest("openai_chat", "gpt-5.5", 401, 12*time.Millisecond)
	m.RecordRequest("bedrock_invoke", "anthropic.claude-opus-4-7", 200, 850*time.Millisecond)
	m.RecordTokens("gpt-5.5", "input", 16)
	m.RecordTokens("gpt-5.5", "output", 20)

	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("want text/plain content-type, got %q", got)
	}
	body := rec.Body.String()
	wants := []string{
		`# TYPE zd_claude_proxy_requests_total counter`,
		`# TYPE zd_claude_proxy_latency_seconds histogram`,
		`# TYPE zd_claude_proxy_tokens_total counter`,
		`# TYPE zd_claude_proxy_inflight gauge`,
		`zd_claude_proxy_requests_total{route="openai_chat",model="gpt-5.5",upstream_status="200"} 2`,
		`zd_claude_proxy_requests_total{route="openai_chat",model="gpt-5.5",upstream_status="401"} 1`,
		`zd_claude_proxy_requests_total{route="bedrock_invoke",model="anthropic.claude-opus-4-7",upstream_status="200"} 1`,
		`zd_claude_proxy_tokens_total{model="gpt-5.5",direction="input"} 16`,
		`zd_claude_proxy_tokens_total{model="gpt-5.5",direction="output"} 20`,
		`zd_claude_proxy_latency_seconds_bucket{route="openai_chat",model="gpt-5.5",le="0.1"} 1`,
		`zd_claude_proxy_latency_seconds_bucket{route="openai_chat",model="gpt-5.5",le="0.25"} 3`,
		`zd_claude_proxy_latency_seconds_count{route="openai_chat",model="gpt-5.5"} 3`,
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("missing line:\n  %s\nin body:\n%s", w, body)
		}
	}
}

// TestMetrics_LabelEscape proves that a malicious-looking model id with
// quote/backslash/newline cannot break the exposition format.
func TestMetrics_LabelEscape(t *testing.T) {
	m := NewMetrics()
	m.RecordRequest("openai_chat", `bad"model\nv\"`, 200, time.Millisecond)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	want := `zd_claude_proxy_requests_total{route="openai_chat",model="bad\"model\\nv\\\"",upstream_status="200"} 1`
	if !strings.Contains(body, want) {
		t.Fatalf("want escaped line:\n  %s\nin body:\n%s", want, body)
	}
}

// TestMetrics_InflightGauge proves the gauge increments on Begin and
// decrements on End.
func TestMetrics_InflightGauge(t *testing.T) {
	m := NewMetrics()
	m.BeginInflight("openai_chat")
	m.BeginInflight("openai_chat")
	m.BeginInflight("bedrock_invoke")
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `zd_claude_proxy_inflight{route="openai_chat"} 2`) {
		t.Fatalf("want openai inflight 2, got body:\n%s", body)
	}
	if !strings.Contains(body, `zd_claude_proxy_inflight{route="bedrock_invoke"} 1`) {
		t.Fatalf("want bedrock inflight 1, got body:\n%s", body)
	}

	m.EndInflight("openai_chat")
	m.EndInflight("openai_chat")
	m.EndInflight("bedrock_invoke")
	rec = httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body = rec.Body.String()
	if !strings.Contains(body, `zd_claude_proxy_inflight{route="openai_chat"} 0`) {
		t.Fatalf("want openai inflight 0, got body:\n%s", body)
	}
}

// TestServer_MetricsBindIsLoopbackOnly proves the metrics listener also
// rejects non-loopback binds even when the main listener is loopback.
func TestServer_MetricsBindIsLoopbackOnly(t *testing.T) {
	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "0.0.0.0:0",
		BedrockBaseURL: "https://example.test/bedrock",
		OpenAIBaseURL:  "https://example.test/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("want error for non-loopback metrics bind, got nil")
	}
}

// TestServer_MetricsEndpointObservesRequests boots the full server with a
// real metrics listener, fires one /v1/chat/completions through a fake
// upstream, scrapes /metrics and checks that requests_total + latency
// histogram show the request.
func TestServer_MetricsEndpointObservesRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-x","model":"gpt-5.5","usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}`))
	}))
	defer upstream.Close()

	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://example.test/bedrock",
		OpenAIBaseURL:  "https://example.test/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	srv, err := NewServer(cfg, Secrets{BedrockBearer: "irrelevant", OpenAIBearer: "irrelevant"}, "tok")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.SetOpenAITransport(NewOpenAITransport(upstream.URL, "irrelevant", upstream.Client()))
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	body, _ := json.Marshal(map[string]any{
		"model":                 "gpt-5.5",
		"max_completion_tokens": 8,
		"messages":              []map[string]any{{"role": "user", "content": "ping"}},
	})
	req, _ := http.NewRequest(http.MethodPost, "http://"+srv.Addr()+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("X-Local-Auth", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxy POST: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 from proxy, got %d", resp.StatusCode)
	}

	// Scrape /metrics on the metrics listener.
	mAddr := srv.MetricsAddr()
	if mAddr == "" {
		t.Fatalf("want metrics addr, got empty")
	}
	if host, _, _ := net.SplitHostPort(mAddr); host != "127.0.0.1" {
		t.Fatalf("metrics listener not loopback: %s", mAddr)
	}
	mResp, err := http.Get("http://" + mAddr + "/metrics")
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	mb, _ := io.ReadAll(mResp.Body)
	mResp.Body.Close()
	got := string(mb)

	wants := []string{
		`zd_claude_proxy_requests_total{route="openai_chat",model="gpt-5.5",upstream_status="200"} 1`,
		`zd_claude_proxy_latency_seconds_count{route="openai_chat",model="gpt-5.5"} 1`,
		`zd_claude_proxy_tokens_total{model="gpt-5.5",direction="input"} 11`,
		`zd_claude_proxy_tokens_total{model="gpt-5.5",direction="output"} 3`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in metrics:\n%s", w, got)
		}
	}
}
