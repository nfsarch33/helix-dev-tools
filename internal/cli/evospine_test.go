package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEvospineCommand_Registration(t *testing.T) {
	if evospineCmd == nil {
		t.Fatal("evospineCmd is nil")
	}
	if evospineCmd.Use != "evospine" {
		t.Errorf("evospineCmd.Use = %q, want %q", evospineCmd.Use, "evospine")
	}
	if evospineCmd.Short == "" {
		t.Error("evospineCmd.Short should not be empty")
	}

	daemon, _, err := evospineCmd.Find([]string{"daemon"})
	if err != nil {
		t.Fatalf("Find(daemon) error: %v", err)
	}
	if daemon == nil || daemon.Use != "daemon" {
		t.Fatalf("expected daemon subcommand, got %v", daemon)
	}
}

func TestEvospineDaemon_IntervalFlag(t *testing.T) {
	flag := evospineDaemonCmd.Flags().Lookup("interval")
	if flag == nil {
		t.Fatal("evospine daemon: missing --interval flag")
	}
	if flag.DefValue != "6h0m0s" {
		t.Errorf("interval default = %q, want %q", flag.DefValue, "6h0m0s")
	}
}

func TestEvospineDaemon_RejectsTooShortInterval(t *testing.T) {
	prev := evospineDaemonFlags.interval
	evospineDaemonFlags.interval = 30 * time.Second
	t.Cleanup(func() { evospineDaemonFlags.interval = prev })

	err := runEvospineDaemon(evospineDaemonCmd, nil)
	if err == nil {
		t.Fatal("expected error for 30s interval, got nil")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "too short")
	}
}

func TestReadAgentraceEvents_MissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.ndjson")
	events, err := readAgentraceEvents(missing)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events from missing file, got %d", len(events))
	}
}

func TestGenerateORHEP_HasAllSections(t *testing.T) {
	events := []agentraceEvent{
		{Timestamp: "2026-05-27T04:00:00+10:00", Tool: "engram_search", AgentID: "test", DurationMS: 250, Success: true},
		{Timestamp: "2026-05-27T04:00:01+10:00", Tool: "engram_add", AgentID: "test", DurationMS: 800, Success: false, Error: "boom"},
	}
	capsule := generateORHEP(events)
	for _, section := range []string{"## Observe", "## Reflect", "## Heal", "## Evolve", "## Promote"} {
		if !strings.Contains(capsule, section) {
			t.Errorf("capsule missing %q", section)
		}
	}
}

func TestRunEvospineCycle_NoFile(t *testing.T) {
	tmp := t.TempDir()
	if err := runEvospineCycle(tmp); err != nil {
		t.Fatalf("expected nil error when agentrace file is missing, got %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(tmp, "logs", "runx", "evospine-capsule-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected no capsule when no events, got %d files", len(matches))
	}
}

func TestRunEvospineCycle_WritesCapsule(t *testing.T) {
	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs", "runx")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ndjson := `{"ts":"2026-05-27T04:00:00+10:00","tool":"engram_search","agent_id":"test","duration_ms":250,"success":true}
{"ts":"2026-05-27T04:00:01+10:00","tool":"engram_add","agent_id":"test","duration_ms":800,"success":false,"error":"boom"}
`
	if err := os.WriteFile(filepath.Join(logDir, "agentrace-mcp.ndjson"), []byte(ndjson), 0o644); err != nil {
		t.Fatalf("write ndjson: %v", err)
	}

	if err := runEvospineCycle(tmp); err != nil {
		t.Fatalf("runEvospineCycle: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(logDir, "evospine-capsule-*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 capsule file, got %d", len(matches))
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read capsule: %v", err)
	}
	for _, section := range []string{"## Observe", "## Reflect", "## Heal", "## Evolve", "## Promote"} {
		if !strings.Contains(string(body), section) {
			t.Errorf("capsule missing %q", section)
		}
	}
}
