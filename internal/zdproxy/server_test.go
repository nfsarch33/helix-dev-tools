package zdproxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServer_BindFailsOnNonLoopback(t *testing.T) {
	cfg := Config{
		Bind:           "0.0.0.0:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	_, err := NewServer(cfg, Secrets{BedrockBearer: "a", OpenAIBearer: "b"}, "tok")
	if err == nil {
		t.Fatal("expected non-loopback bind to be rejected")
	}
}

func TestServer_HealthzOK(t *testing.T) {
	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	s, err := NewServer(cfg, Secrets{BedrockBearer: "a", OpenAIBearer: "b"}, "tok")
	if err != nil {
		t.Fatalf("unexpected NewServer error: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	url := "http://" + s.Addr() + "/healthz"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%q", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("expected status:ok body, got %q", body)
	}
}

func TestServer_MessagesRequiresLocalAuth(t *testing.T) {
	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	s, err := NewServer(cfg, Secrets{BedrockBearer: "a", OpenAIBearer: "b"}, "tok")
	if err != nil {
		t.Fatalf("unexpected NewServer error: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	resp, err := http.Post("http://"+s.Addr()+"/v1/messages", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST /v1/messages: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401, got %d (body=%q)", resp.StatusCode, body)
	}
}

// TestServer_MessagesProxiesToFakeUpstream is the end-to-end integration test
// proving the v256.5 hotfix: /v1/messages now returns a real Anthropic
// response from the upstream gateway (here a fake httptest.Server) instead of
// 501.
func TestServer_MessagesProxiesToFakeUpstream(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	s, err := NewServer(cfg, Secrets{BedrockBearer: "BEDROCK", OpenAIBearer: "OPENAI"}, "tok")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	// Re-point the bedrock transport at the fake httptest upstream. The
	// server still validates that cfg.BedrockBaseURL is https:// at boot,
	// preserving the loopback/https invariant for production callers.
	s.SetBedrockTransport(NewBedrockTransport(up.server.URL, "BEDROCK", &http.Client{Timeout: 5 * time.Second}))

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	body := `{"model":"us.anthropic.claude-3-5-haiku-20241022-v1:0","messages":[{"role":"user","content":"ping"}],"max_tokens":4}`
	req, _ := http.NewRequest("POST", "http://"+s.Addr()+"/v1/messages", strings.NewReader(body))
	req.Header.Set("X-Local-Auth", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from fake upstream, got %d (body=%q)", resp.StatusCode, out)
	}
	if !strings.Contains(string(out), `"type":"message"`) {
		t.Fatalf("expected upstream Anthropic envelope forwarded, got %q", out)
	}
	if strings.Contains(string(out), "BEDROCK") {
		t.Fatalf("response must not contain upstream bearer, got %q", out)
	}
	path, _, headers, _ := up.snapshot()
	if path != "/model/us.anthropic.claude-3-5-haiku-20241022-v1:0/invoke" {
		t.Fatalf("expected upstream invoke path, got %q", path)
	}
	if got := headers.Get("Authorization"); got != "Bearer BEDROCK" {
		t.Fatalf("expected upstream Authorization=Bearer BEDROCK, got %q", got)
	}
}

// TestServer_BedrockPassthroughEndToEnd validates the Bedrock-shape route is
// reachable through the auth gate and forwards correctly.
func TestServer_BedrockPassthroughEndToEnd(t *testing.T) {
	up := newFakeBedrockUpstream()
	defer up.close()

	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	s, err := NewServer(cfg, Secrets{BedrockBearer: "BEDROCK", OpenAIBearer: "OPENAI"}, "tok")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.SetBedrockTransport(NewBedrockTransport(up.server.URL, "BEDROCK", &http.Client{Timeout: 5 * time.Second}))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	body := `{"anthropic_version":"bedrock-2023-05-31","messages":[{"role":"user","content":"hi"}],"max_tokens":4}`
	req, _ := http.NewRequest("POST", "http://"+s.Addr()+"/bedrock/model/us.anthropic.claude-opus-4-7/invoke", strings.NewReader(body))
	req.Header.Set("X-Local-Auth", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		got, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d (body=%q)", resp.StatusCode, got)
	}
	path, _, _, _ := up.snapshot()
	if path != "/model/us.anthropic.claude-opus-4-7/invoke" {
		t.Fatalf("expected upstream path, got %q", path)
	}
}

// TestServer_ChatCompletionsEndToEnd verifies the /v1/chat/completions route
// is registered behind the auth gate and forwards correctly to a fake OpenAI
// upstream.
func TestServer_ChatCompletionsEndToEnd(t *testing.T) {
	up := newFakeOpenAIUpstream()
	defer up.close()

	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	s, err := NewServer(cfg, Secrets{BedrockBearer: "BEDROCK", OpenAIBearer: "OPENAI"}, "tok")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.SetOpenAITransport(NewOpenAITransport(up.server.URL, "OPENAI", &http.Client{Timeout: 5 * time.Second}))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	body := `{"model":"gpt-5.5","max_completion_tokens":4,"messages":[{"role":"user","content":"ping"}]}`
	req, _ := http.NewRequest("POST", "http://"+s.Addr()+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("X-Local-Auth", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		got, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d (%q)", resp.StatusCode, got)
	}
	path, _, headers, _ := up.snapshot()
	if path != "/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions, got %q", path)
	}
	if got := headers.Get("Authorization"); got != "Bearer OPENAI" {
		t.Fatalf("expected Authorization=Bearer OPENAI, got %q", got)
	}
}

// TestServer_ResponsesEndToEnd validates the /v1/responses route is reachable.
func TestServer_ResponsesEndToEnd(t *testing.T) {
	up := newFakeOpenAIUpstream()
	defer up.close()

	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	s, err := NewServer(cfg, Secrets{BedrockBearer: "BEDROCK", OpenAIBearer: "OPENAI"}, "tok")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.SetOpenAITransport(NewOpenAITransport(up.server.URL, "OPENAI", &http.Client{Timeout: 5 * time.Second}))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	body := `{"model":"gpt-5-codex","input":[{"role":"user","content":"refactor"}]}`
	req, _ := http.NewRequest("POST", "http://"+s.Addr()+"/v1/responses", strings.NewReader(body))
	req.Header.Set("X-Local-Auth", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		got, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d (%q)", resp.StatusCode, got)
	}
	path, _, _, _ := up.snapshot()
	if path != "/v1/responses" {
		t.Fatalf("expected /v1/responses, got %q", path)
	}
}

// TestServer_MessagesDispatchesToOpenAIChatRoute proves /v1/messages with a
// GPT-family model lands on the OpenAI fake upstream rather than the
// Bedrock fake.
func TestServer_MessagesDispatchesToOpenAIChatRoute(t *testing.T) {
	bedrockUp := newFakeBedrockUpstream()
	defer bedrockUp.close()
	openaiUp := newFakeOpenAIUpstream()
	defer openaiUp.close()

	cfg := Config{
		Bind:           "127.0.0.1:0",
		MetricsBind:    "127.0.0.1:0",
		BedrockBaseURL: "https://ai-gateway.zende.sk/bedrock",
		OpenAIBaseURL:  "https://ai-gateway.zende.sk/v1",
		OpBedrockItem:  "x",
		OpOpenAIItem:   "y",
	}
	s, err := NewServer(cfg, Secrets{BedrockBearer: "BEDROCK", OpenAIBearer: "OPENAI"}, "tok")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.SetBedrockTransport(NewBedrockTransport(bedrockUp.server.URL, "BEDROCK", &http.Client{Timeout: 5 * time.Second}))
	s.SetOpenAITransport(NewOpenAITransport(openaiUp.server.URL, "OPENAI", &http.Client{Timeout: 5 * time.Second}))
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop(context.Background())

	body := `{"model":"gpt-5.5","max_tokens":4,"messages":[{"role":"user","content":"ping"}]}`
	req, _ := http.NewRequest("POST", "http://"+s.Addr()+"/v1/messages", strings.NewReader(body))
	req.Header.Set("X-Local-Auth", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		got, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d (%q)", resp.StatusCode, got)
	}
	if p, _, _, _ := bedrockUp.snapshot(); p != "" {
		t.Fatalf("expected Bedrock NOT called, got path %q", p)
	}
	openaiPath, _, _, _ := openaiUp.snapshot()
	if openaiPath != "/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions dispatch, got %q", openaiPath)
	}
}

// keep httptest reference so the `time` import survives if the smoke test is
// later moved.
var _ = httptest.NewServer
