package evalharness

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDaemon_NewAndHealth(t *testing.T) {
	d := NewDaemon(DefaultDaemonConfig())
	h := d.Health()
	if h.EventsGraded != 0 {
		t.Errorf("new daemon should have 0 events graded, got %d", h.EventsGraded)
	}
}

func TestDaemon_GradeEvent(t *testing.T) {
	d := NewDaemon(DefaultDaemonConfig())
	event := AgentTraceEvent{Event: "tool_call", LatencyMS: 100, Success: true}
	results := d.GradeEvent(event)
	graderCount := len(AllGraders(DefaultGraderConfig()))
	if len(results) != graderCount {
		t.Errorf("expected %d results (one per grader), got %d", graderCount, len(results))
	}
	h := d.Health()
	if h.EventsGraded != graderCount {
		t.Errorf("expected %d total grades stored, got %d", graderCount, h.EventsGraded)
	}
}

func TestDaemon_TailNDJSON(t *testing.T) {
	events := []AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 100, Success: true},
		{Event: "tool_call", LatencyMS: 200, Success: true, Error: "timeout"},
		{Event: "test_run", Coverage: 0.85},
	}

	var buf bytes.Buffer
	for _, e := range events {
		b, _ := json.Marshal(e)
		buf.Write(b)
		buf.WriteByte('\n')
	}

	d := NewDaemon(DefaultDaemonConfig())
	err := d.TailNDJSON(context.Background(), &buf)
	if err != nil {
		t.Fatalf("TailNDJSON error: %v", err)
	}

	results := d.Results()
	graderCount := len(AllGraders(DefaultGraderConfig()))
	expectedGrades := 3 * graderCount
	if len(results) != expectedGrades {
		t.Errorf("expected %d grade results, got %d", expectedGrades, len(results))
	}
}

func TestDaemon_TailNDJSON_InvalidLines(t *testing.T) {
	input := "not json\n{\"event\":\"tool_call\",\"latency_ms\":50,\"success\":true}\nalso invalid\n"
	d := NewDaemon(DefaultDaemonConfig())
	err := d.TailNDJSON(context.Background(), bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("TailNDJSON error: %v", err)
	}
	results := d.Results()
	graderCount := len(AllGraders(DefaultGraderConfig()))
	if len(results) != graderCount {
		t.Errorf("expected %d results from 1 valid event, got %d", graderCount, len(results))
	}
}

func TestDaemon_TailNDJSON_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input := "{\"event\":\"tool_call\",\"latency_ms\":50,\"success\":true}\n"
	d := NewDaemon(DefaultDaemonConfig())
	err := d.TailNDJSON(ctx, bytes.NewBufferString(input))
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDaemon_StartStop(t *testing.T) {
	cfg := DefaultDaemonConfig()
	cfg.PollInterval = 10 * time.Millisecond
	d := NewDaemon(cfg)

	ctx := context.Background()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	d.Stop()
}

func TestDaemon_PollFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agentrace.ndjson")

	event := AgentTraceEvent{Event: "tool_call", LatencyMS: 100, Success: true}
	b, _ := json.Marshal(event)
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultDaemonConfig()
	cfg.AgentracePath = path
	cfg.PollInterval = 10 * time.Millisecond
	d := NewDaemon(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	if err := d.Start(ctx); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()
	d.Stop()

	results := d.Results()
	graderCount := len(AllGraders(DefaultGraderConfig()))
	if len(results) < graderCount {
		t.Errorf("expected at least %d results from polled file, got %d", graderCount, len(results))
	}
}

func TestDaemon_FormatHealthJSON(t *testing.T) {
	d := NewDaemon(DefaultDaemonConfig())
	out := d.FormatHealthJSON()
	if out == "" {
		t.Error("expected non-empty JSON")
	}
	var h HealthStatus
	if err := json.Unmarshal([]byte(out), &h); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}
