package tunnelkeep

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultMem0Config(t *testing.T) {
	cfg := DefaultMem0Config()
	if cfg.Name != "mem0-oracle" {
		t.Errorf("unexpected name: %s", cfg.Name)
	}
	if cfg.Interval != 5*time.Minute {
		t.Errorf("unexpected interval: %v", cfg.Interval)
	}
}

func TestProbe_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cfg := TunnelConfig{Name: "test", LocalURL: srv.URL, Timeout: 5 * time.Second}
	result := Probe(cfg)

	if !result.Healthy {
		t.Error("expected healthy probe")
	}
	if result.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestProbe_Unhealthy(t *testing.T) {
	cfg := TunnelConfig{Name: "test", LocalURL: "http://127.0.0.1:1/healthz", Timeout: 1 * time.Second}
	result := Probe(cfg)

	if result.Healthy {
		t.Error("expected unhealthy probe for unreachable endpoint")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestShouldRestart_ConsecutiveFailures(t *testing.T) {
	results := []ProbeResult{
		{Healthy: true},
		{Healthy: false},
		{Healthy: false},
		{Healthy: false},
	}

	if !ShouldRestart(results, 3) {
		t.Error("expected restart after 3 consecutive failures")
	}
}

func TestShouldRestart_NotEnoughFailures(t *testing.T) {
	results := []ProbeResult{
		{Healthy: true},
		{Healthy: false},
		{Healthy: false},
	}

	if ShouldRestart(results, 3) {
		t.Error("should not restart with only 2 consecutive failures (threshold 3)")
	}
}

func TestShouldRestart_InsufficientData(t *testing.T) {
	results := []ProbeResult{{Healthy: false}}
	if ShouldRestart(results, 3) {
		t.Error("should not restart with insufficient data")
	}
}

func TestShouldRestart_RecoveredInMiddle(t *testing.T) {
	results := []ProbeResult{
		{Healthy: false},
		{Healthy: false},
		{Healthy: true},
		{Healthy: false},
		{Healthy: false},
	}

	if ShouldRestart(results, 3) {
		t.Error("should not restart when recovery happened in between")
	}
}

func TestGenerateLaunchdPlist(t *testing.T) {
	cfg := DefaultMem0Config()
	plist := GenerateLaunchdPlist(cfg)

	if !strings.Contains(plist, "mem0-oracle") {
		t.Error("expected tunnel name in plist")
	}
	if !strings.Contains(plist, "cursor-tools") {
		t.Error("expected cursor-tools binary in plist")
	}
	if !strings.Contains(plist, "300") {
		t.Error("expected 300s interval in plist")
	}
}
