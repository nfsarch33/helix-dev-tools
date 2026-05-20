package tunnelmonitor

import (
	"net"
	"strconv"
	"testing"
	"time"
)

func TestNewMonitor(t *testing.T) {
	m := NewMonitor("mem0-oracle", "127.0.0.1", 18888, 5*time.Second)
	if m.Name != "mem0-oracle" {
		t.Errorf("name: %s", m.Name)
	}
	if m.Healthy {
		t.Error("should start unhealthy until probed")
	}
}

func TestProbeHealthy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	m := NewMonitor("test", "127.0.0.1", port, 2*time.Second)
	result := m.Probe()
	if !result.Healthy {
		t.Errorf("probe should be healthy for open port %d", port)
	}
	if m.Healthy != true {
		t.Error("monitor state should be healthy after successful probe")
	}
}

func TestProbeUnhealthy(t *testing.T) {
	m := NewMonitor("test", "127.0.0.1", 59999, 500*time.Millisecond)
	result := m.Probe()
	if result.Healthy {
		t.Error("probe should be unhealthy for closed port")
	}
	if result.Error == "" {
		t.Error("should have error message")
	}
}

func TestConsecutiveFailures(t *testing.T) {
	m := NewMonitor("test", "127.0.0.1", 59999, 200*time.Millisecond)
	m.Probe()
	m.Probe()
	m.Probe()
	if m.ConsecutiveFails != 3 {
		t.Errorf("consecutive fails: %d", m.ConsecutiveFails)
	}
}

func TestFailureResetOnSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	m := NewMonitor("test", "127.0.0.1", port, 2*time.Second)
	m.ConsecutiveFails = 5
	m.Probe()
	if m.ConsecutiveFails != 0 {
		t.Errorf("should reset on success: %d", m.ConsecutiveFails)
	}
}

func TestNeedsReconnect(t *testing.T) {
	m := NewMonitor("test", "127.0.0.1", 59999, 200*time.Millisecond)
	m.ReconnectThreshold = 3
	m.Probe()
	m.Probe()
	if m.NeedsReconnect() {
		t.Error("should not need reconnect with only 2 failures")
	}
	m.Probe()
	if !m.NeedsReconnect() {
		t.Error("should need reconnect after 3 failures")
	}
}

func TestProbeLatency(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	m := NewMonitor("test", "127.0.0.1", port, 2*time.Second)
	result := m.Probe()
	if result.Latency <= 0 {
		t.Errorf("latency should be positive: %v", result.Latency)
	}
}

func TestAddress(t *testing.T) {
	m := NewMonitor("test", "127.0.0.1", 18888, time.Second)
	expected := "127.0.0.1:" + strconv.Itoa(18888)
	if m.Address() != expected {
		t.Errorf("address: %s", m.Address())
	}
}

func TestHistory(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	m := NewMonitor("test", "127.0.0.1", port, 2*time.Second)
	m.Probe()
	m.Probe()
	h := m.History()
	if len(h) != 2 {
		t.Errorf("history: %d", len(h))
	}
}
