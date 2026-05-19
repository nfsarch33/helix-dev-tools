package mem0monitor

import (
	"testing"
	"time"
)

func TestNewMonitor(t *testing.T) {
	m := New(Config{Endpoint: "http://localhost:8080", Interval: 30 * time.Second})
	if m == nil {
		t.Fatal("expected non-nil monitor")
	}
}

func TestRecordProbe(t *testing.T) {
	m := New(Config{Endpoint: "http://localhost:8080"})
	m.RecordProbe(Probe{
		Timestamp: time.Now(),
		Latency:   250 * time.Millisecond,
		Healthy:   true,
		Operation: "add_no_infer",
	})
	probes := m.Probes()
	if len(probes) != 1 {
		t.Fatalf("got %d probes, want 1", len(probes))
	}
}

func TestAlertThresholds(t *testing.T) {
	m := New(Config{
		Endpoint:      "http://localhost:8080",
		LatencyWarnMs: 2000,
		LatencyCritMs: 30000,
	})
	m.RecordProbe(Probe{Latency: 500 * time.Millisecond, Healthy: true, Operation: "healthz"})
	m.RecordProbe(Probe{Latency: 3 * time.Second, Healthy: true, Operation: "search"})
	m.RecordProbe(Probe{Latency: 35 * time.Second, Healthy: false, Operation: "add_infer"})

	alerts := m.CheckAlerts()
	if len(alerts) != 2 {
		t.Fatalf("got %d alerts, want 2 (warn + crit)", len(alerts))
	}
}

func TestAvgLatency(t *testing.T) {
	m := New(Config{Endpoint: "http://localhost:8080"})
	m.RecordProbe(Probe{Latency: 200 * time.Millisecond, Operation: "healthz"})
	m.RecordProbe(Probe{Latency: 400 * time.Millisecond, Operation: "add"})

	avg := m.AvgLatency()
	if avg < 290*time.Millisecond || avg > 310*time.Millisecond {
		t.Errorf("got avg %v, want ~300ms", avg)
	}
}

func TestHealthRate(t *testing.T) {
	m := New(Config{Endpoint: "http://localhost:8080"})
	m.RecordProbe(Probe{Healthy: true})
	m.RecordProbe(Probe{Healthy: true})
	m.RecordProbe(Probe{Healthy: false})

	rate := m.HealthRate()
	if rate < 0.66 || rate > 0.67 {
		t.Errorf("got health rate %f, want ~0.667", rate)
	}
}
