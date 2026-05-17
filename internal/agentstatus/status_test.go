package agentstatus_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/agentstatus"
)

type stubSource struct {
	name     string
	statuses []agentstatus.AgentStatus
	err      error
}

func (s *stubSource) Name() string { return s.name }
func (s *stubSource) Read(_ context.Context) ([]agentstatus.AgentStatus, error) {
	return s.statuses, s.err
}

func TestCollector_Collect_AggregatesSources(t *testing.T) {
	t.Parallel()
	s1 := &stubSource{
		name: "s1",
		statuses: []agentstatus.AgentStatus{
			{Name: "agent-a", State: agentstatus.StateRunning, LastActivity: time.Now()},
		},
	}
	s2 := &stubSource{
		name: "s2",
		statuses: []agentstatus.AgentStatus{
			{Name: "agent-b", State: agentstatus.StateIdle, LastActivity: time.Now()},
		},
	}

	c := agentstatus.NewCollector(slog.Default(), 15*time.Minute, s1, s2)
	report := c.Collect(context.Background())

	if len(report.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(report.Agents))
	}
	if report.Agents[0].Name != "agent-a" || report.Agents[1].Name != "agent-b" {
		t.Errorf("unexpected agents: %+v", report.Agents)
	}
}

func TestCollector_HungDetection(t *testing.T) {
	t.Parallel()
	old := time.Now().Add(-30 * time.Minute)
	src := &stubSource{
		name: "hung-src",
		statuses: []agentstatus.AgentStatus{
			{Name: "hung-agent", State: agentstatus.StateRunning, LastActivity: old},
		},
	}

	c := agentstatus.NewCollector(slog.Default(), 15*time.Minute, src)
	report := c.Collect(context.Background())

	if report.Agents[0].State != agentstatus.StateHung {
		t.Errorf("expected hung, got %s", report.Agents[0].State)
	}
}

func TestCollector_RecentAgent_NotHung(t *testing.T) {
	t.Parallel()
	src := &stubSource{
		name: "active-src",
		statuses: []agentstatus.AgentStatus{
			{Name: "active-agent", State: agentstatus.StateRunning, LastActivity: time.Now()},
		},
	}

	c := agentstatus.NewCollector(slog.Default(), 15*time.Minute, src)
	report := c.Collect(context.Background())

	if report.Agents[0].State != agentstatus.StateRunning {
		t.Errorf("expected running, got %s", report.Agents[0].State)
	}
}

func TestCollector_EmptySources(t *testing.T) {
	t.Parallel()
	c := agentstatus.NewCollector(slog.Default(), 15*time.Minute)
	report := c.Collect(context.Background())
	if len(report.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(report.Agents))
	}
}

func TestHandler_StatusEndpoint(t *testing.T) {
	t.Parallel()
	src := &stubSource{
		name: "http-src",
		statuses: []agentstatus.AgentStatus{
			{Name: "api-agent", State: agentstatus.StateIdle, LastActivity: time.Now()},
		},
	}
	c := agentstatus.NewCollector(slog.Default(), 15*time.Minute, src)

	srv := httptest.NewServer(c.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content-type=%s", resp.Header.Get("Content-Type"))
	}

	var report agentstatus.StatusReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Agents) != 1 || report.Agents[0].Name != "api-agent" {
		t.Errorf("unexpected report: %+v", report)
	}
}

func TestHandler_HealthEndpoint(t *testing.T) {
	t.Parallel()
	c := agentstatus.NewCollector(slog.Default(), 15*time.Minute)
	srv := httptest.NewServer(c.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestAgentTraceSource_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := agentstatus.NewAgentTraceSource(dir, 24*time.Hour)
	statuses, err := src.Read(context.Background())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0, got %d", len(statuses))
	}
}

func TestAgentTraceSource_ReadsRecentTranscript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	agentDir := filepath.Join(dir, "test-agent-1234")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entry := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"role":      "assistant",
	}
	data, _ := json.Marshal(entry)
	jsonlPath := filepath.Join(agentDir, "test-agent-1234.jsonl")
	if err := os.WriteFile(jsonlPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	src := agentstatus.NewAgentTraceSource(dir, 24*time.Hour)
	statuses, err := src.Read(context.Background())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Name != "test-age" {
		t.Errorf("name=%s", statuses[0].Name)
	}
	if statuses[0].State != agentstatus.StateRunning {
		t.Errorf("state=%s, expected running (recent transcript)", statuses[0].State)
	}
}

func TestAgentTraceSource_SkipsOldTranscripts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	agentDir := filepath.Join(dir, "old-agent-5678")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	jsonlPath := filepath.Join(agentDir, "old-agent-5678.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{"role":"user"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(jsonlPath, oldTime, oldTime)

	src := agentstatus.NewAgentTraceSource(dir, 24*time.Hour)
	statuses, err := src.Read(context.Background())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected 0 (old transcript), got %d", len(statuses))
	}
}
