package coordination

import (
	"strings"
	"testing"
	"time"
)

func TestAlertmanagerBlockerSignal_AlertToMem0AndStartupPrompt(t *testing.T) {
	recordedAt := time.Date(2026, 5, 7, 23, 55, 0, 0, time.FixedZone("AEST", 10*60*60))
	signal := AlertmanagerBlockerSignal(AlertmanagerWebhook{
		Receiver: "cursor-tools-signal-blocker",
		Status:   "firing",
		GroupKey: "{}:{alertname=\"RouterP95LatencyFastBurn\"}",
		CommonLabels: map[string]string{
			"alertname": "RouterP95LatencyFastBurn",
			"severity":  "critical",
			"slo":       "router-p95-latency",
		},
		CommonAnnotations: map[string]string{
			"summary": "Router p95 latency is burning the fast SLO window",
		},
		Alerts: []AlertmanagerAlert{
			{
				Status:       "firing",
				Labels:       map[string]string{"burn_window": "fast"},
				GeneratorURL: "http://prometheus.example/graph?g0.expr=router",
			},
		},
	}, "v310", recordedAt)

	if signal.Type != SignalBlocker {
		t.Fatalf("Type = %q, want blocker", signal.Type)
	}
	if signal.Priority != "high" {
		t.Fatalf("Priority = %q, want high", signal.Priority)
	}
	if signal.Sprint != "v310" {
		t.Fatalf("Sprint = %q, want v310", signal.Sprint)
	}
	if !strings.Contains(signal.Mem0Text(), "Blocker: Alertmanager firing: RouterP95LatencyFastBurn") {
		t.Fatalf("Mem0Text missing alert name: %q", signal.Mem0Text())
	}

	meta := signal.Mem0Metadata()
	checks := map[string]string{
		"source":         "alertmanager",
		"receiver":       "cursor-tools-signal-blocker",
		"alertname":      "RouterP95LatencyFastBurn",
		"severity":       "critical",
		"slo":            "router-p95-latency",
		"status":         "firing",
		"alert_count":    "1",
		"firing_count":   "1",
		"startup_prompt": "true",
		"recorded_at":    "2026-05-07T23:55:00+10:00",
	}
	for key, want := range checks {
		if got := meta[key]; got != want {
			t.Fatalf("metadata[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestAlertmanagerBlockerSignal_ResolvedOnlyIsNormalPriority(t *testing.T) {
	signal := AlertmanagerBlockerSignal(AlertmanagerWebhook{
		Status: "resolved",
		Alerts: []AlertmanagerAlert{
			{
				Status:      "resolved",
				Labels:      map[string]string{"alertname": "Mem0OSSReadHitRateFastBurn", "severity": "critical"},
				Annotations: map[string]string{"summary": "Mem0 recovered"},
			},
		},
	}, "v310", time.Unix(0, 0).UTC())

	if signal.Priority != "normal" {
		t.Fatalf("Priority = %q, want normal", signal.Priority)
	}
	if signal.Metadata["firing_count"] != "0" {
		t.Fatalf("firing_count = %q, want 0", signal.Metadata["firing_count"])
	}
	if !strings.Contains(signal.Message, "Mem0 recovered") {
		t.Fatalf("Message = %q, want resolved summary", signal.Message)
	}
}
