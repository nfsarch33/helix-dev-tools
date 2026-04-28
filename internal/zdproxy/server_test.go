package zdproxy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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

	resp, err := http.Post("http://"+s.Addr()+"/messages", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST /messages: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401, got %d (body=%q)", resp.StatusCode, body)
	}
}

func TestServer_MessagesReturns501ForMVP(t *testing.T) {
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

	req, _ := http.NewRequest("POST", "http://"+s.Addr()+"/messages", strings.NewReader(`{"model":"us.anthropic.claude-opus-4-7","messages":[]}`))
	req.Header.Set("X-Local-Auth", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("MVP /messages should return 501 until v257 spike, got %d (body=%q)", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "BEARERVALUE") || strings.Contains(string(body), "Bearer ") {
		t.Fatalf("response must not echo upstream credentials, got %q", body)
	}
}
