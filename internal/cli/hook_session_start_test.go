package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
)

func TestSessionStartHandler_ReturnsStructuredJSON(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	p := config.DefaultPaths()
	_ = os.MkdirAll(p.HooksDir, 0o755)

	h := &sessionStartHandler{
		log:   logger.New(filepath.Join(home, "session-start.log")),
		paths: p,
	}

	resp, err := h.Handle(context.Background(), &hookio.Input{})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp == nil {
		t.Fatal("Handle() returned nil response")
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}
	var check hookio.Response
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestSessionStartHandler_RunsWorkspaceDoctor(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldHelper := os.Getenv(helperEnv)
	oldHelperLog := os.Getenv("CURSOR_TOOLS_HELPER_LOG")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv(helperEnv, "1"); err != nil {
		t.Fatal(err)
	}
	helperLogPath := filepath.Join(home, "helper.log")
	if err := os.Setenv("CURSOR_TOOLS_HELPER_LOG", helperLogPath); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv(helperEnv, oldHelper)
		_ = os.Setenv("CURSOR_TOOLS_HELPER_LOG", oldHelperLog)
	}()

	p := config.DefaultPaths()
	_ = os.MkdirAll(p.HooksDir, 0o755)

	h := &sessionStartHandler{
		log:   logger.New(filepath.Join(home, "session-start.log")),
		paths: p,
	}

	_, err := h.Handle(context.Background(), &hookio.Input{})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	helperData, err := os.ReadFile(helperLogPath)
	if err != nil {
		t.Fatal(err)
	}
	helperText := string(helperData)
	for _, want := range []string{"workspace doctor --json", "sync-counts --apply"} {
		if !strings.Contains(helperText, want) {
			t.Errorf("helper log missing %q in %q", want, helperText)
		}
	}
}

func TestSessionStartHandler_ChecksResourceProbe(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	probeDir := filepath.Join(home, "logs", "runx")
	_ = os.MkdirAll(probeDir, 0o755)
	probePath := filepath.Join(probeDir, "resource-probe.ndjson")
	probeEntry := `{"ts":"2026-05-12T23:50:00Z","free_pct":46,"tier":"GREEN"}` + "\n"
	_ = os.WriteFile(probePath, []byte(probeEntry), 0o644)

	p := config.DefaultPaths()
	_ = os.MkdirAll(p.HooksDir, 0o755)

	h := &sessionStartHandler{
		log:   logger.New(filepath.Join(home, "session-start.log")),
		paths: p,
	}

	result := h.readResourceProbe()
	if result.Tier != "GREEN" {
		t.Errorf("resource probe tier = %q, want GREEN", result.Tier)
	}
	if result.FreePct != 46 {
		t.Errorf("resource probe free_pct = %d, want 46", result.FreePct)
	}
}

func TestSessionStartHandler_ReportsRedResourceProbe(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	probeDir := filepath.Join(home, "logs", "runx")
	_ = os.MkdirAll(probeDir, 0o755)
	probePath := filepath.Join(probeDir, "resource-probe.ndjson")
	probeEntry := `{"ts":"2026-05-12T23:50:00Z","free_pct":8,"tier":"RED"}` + "\n"
	_ = os.WriteFile(probePath, []byte(probeEntry), 0o644)

	p := config.DefaultPaths()
	_ = os.MkdirAll(p.HooksDir, 0o755)

	h := &sessionStartHandler{
		log:   logger.New(filepath.Join(home, "session-start.log")),
		paths: p,
	}

	resp, err := h.Handle(context.Background(), &hookio.Input{})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.UserMessage == "" {
		t.Error("expected user message for RED resource probe")
	}
	if !strings.Contains(resp.UserMessage, "RED") {
		t.Errorf("UserMessage should mention RED tier, got %q", resp.UserMessage)
	}
}

func TestSessionStartHandler_MissingProbeReturnsUnknown(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	p := config.DefaultPaths()
	_ = os.MkdirAll(p.HooksDir, 0o755)

	h := &sessionStartHandler{
		log:   logger.New(filepath.Join(home, "session-start.log")),
		paths: p,
	}

	result := h.readResourceProbe()
	if result.Tier != "UNKNOWN" {
		t.Errorf("missing probe tier = %q, want UNKNOWN", result.Tier)
	}
}
