package controlplane

import (
	"testing"
	"time"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(ServiceEntry{Name: "mem0", Endpoint: "localhost:18888", Status: StatusHealthy, TTL: 5 * time.Minute})

	e, ok := r.Get("mem0")
	if !ok {
		t.Fatal("expected to find registered service")
	}
	if e.Endpoint != "localhost:18888" {
		t.Errorf("unexpected endpoint: %s", e.Endpoint)
	}
}

func TestRegistry_Deregister(t *testing.T) {
	r := NewRegistry()
	r.Register(ServiceEntry{Name: "svc", Status: StatusHealthy})
	r.Deregister("svc")

	_, ok := r.Get("svc")
	if ok {
		t.Error("expected service to be deregistered")
	}
}

func TestRegistry_Healthy(t *testing.T) {
	r := NewRegistry()
	r.Register(ServiceEntry{Name: "up", Status: StatusHealthy, TTL: 5 * time.Minute})
	r.Register(ServiceEntry{Name: "down", Status: StatusDown, TTL: 5 * time.Minute})

	healthy := r.Healthy()
	if len(healthy) != 1 {
		t.Errorf("expected 1 healthy, got %d", len(healthy))
	}
}

func TestRegistry_ExpireTTL(t *testing.T) {
	r := NewRegistry()
	r.Register(ServiceEntry{Name: "old", Status: StatusHealthy, TTL: 1 * time.Millisecond})
	time.Sleep(5 * time.Millisecond)

	expired := r.ExpireTTL()
	if expired != 1 {
		t.Errorf("expected 1 expired, got %d", expired)
	}
	if r.Count() != 0 {
		t.Error("expected empty registry after expiry")
	}
}

func TestRegistry_Heartbeat(t *testing.T) {
	r := NewRegistry()
	r.Register(ServiceEntry{Name: "svc", Status: StatusHealthy, TTL: 1 * time.Hour})
	time.Sleep(1 * time.Millisecond)

	ok := r.Heartbeat("svc")
	if !ok {
		t.Error("expected heartbeat success")
	}

	ok = r.Heartbeat("nonexistent")
	if ok {
		t.Error("expected heartbeat failure for unknown service")
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()
	r.Register(ServiceEntry{Name: "a", Status: StatusHealthy})
	r.Register(ServiceEntry{Name: "b", Status: StatusDown})

	if r.Count() != 2 {
		t.Errorf("expected count 2, got %d", r.Count())
	}
}
