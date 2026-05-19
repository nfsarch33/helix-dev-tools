package daemonctl

import (
	"testing"
)

func TestNewController(t *testing.T) {
	c := New(Config{PlistDir: "~/Library/LaunchAgents"})
	if c == nil {
		t.Fatal("expected non-nil controller")
	}
}

func TestRegisterDaemon(t *testing.T) {
	c := New(Config{})
	c.Register(Daemon{Name: "sprintboard-mcp", BinaryPath: "~/runs/sprintboard-mcp", AutoStart: true})
	daemons := c.Daemons()
	if len(daemons) != 1 {
		t.Fatalf("got %d daemons, want 1", len(daemons))
	}
}

func TestDaemonStatus(t *testing.T) {
	c := New(Config{})
	c.Register(Daemon{Name: "mem0-tunnel", BinaryPath: "runx tunnel start mem0-oracle"})
	c.SetStatus("mem0-tunnel", StatusRunning)
	s := c.Status("mem0-tunnel")
	if s != StatusRunning {
		t.Errorf("got status %v, want running", s)
	}
}

func TestAutoStartDaemons(t *testing.T) {
	c := New(Config{})
	c.Register(Daemon{Name: "d1", AutoStart: true})
	c.Register(Daemon{Name: "d2", AutoStart: false})
	c.Register(Daemon{Name: "d3", AutoStart: true})
	auto := c.AutoStartDaemons()
	if len(auto) != 2 {
		t.Errorf("got %d auto-start, want 2", len(auto))
	}
}

func TestHealthCheck(t *testing.T) {
	c := New(Config{})
	c.Register(Daemon{Name: "bridge", HealthEndpoint: "http://localhost:8510/healthz"})
	c.SetStatus("bridge", StatusRunning)
	health := c.HealthSummary()
	if health.Running != 1 {
		t.Errorf("got %d running, want 1", health.Running)
	}
}
