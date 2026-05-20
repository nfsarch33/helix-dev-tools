package keyrotate

import (
	"testing"
)

func TestRoundRobinBasic(t *testing.T) {
	p := NewPool([]string{"key-a", "key-b", "key-c"}, nil, RoundRobin)
	k1, _ := p.Next()
	k2, _ := p.Next()
	k3, _ := p.Next()
	if k1 == k2 || k2 == k3 {
		t.Errorf("round-robin should rotate: %s, %s, %s", k1, k2, k3)
	}
}

func TestRoundRobinWraps(t *testing.T) {
	p := NewPool([]string{"a", "b"}, nil, RoundRobin)
	p.Next()
	p.Next()
	k, _ := p.Next()
	if k != "a" && k != "b" {
		t.Errorf("should wrap: got %s", k)
	}
}

func TestLeastUsed(t *testing.T) {
	p := NewPool([]string{"heavy", "light"}, nil, LeastUsed)
	p.keys[0].UseCount = 100
	k, _ := p.Next()
	if k != "light" {
		t.Errorf("expected light (least used), got %s", k)
	}
}

func TestMarkExhaustedSkips(t *testing.T) {
	p := NewPool([]string{"bad", "good"}, nil, RoundRobin)
	p.MarkExhausted("bad")
	for i := 0; i < 5; i++ {
		k, err := p.Next()
		if err != nil {
			t.Fatal(err)
		}
		if k == "bad" {
			t.Fatal("should skip exhausted key")
		}
	}
}

func TestAllExhausted(t *testing.T) {
	p := NewPool([]string{"a", "b"}, nil, RoundRobin)
	p.MarkExhausted("a")
	p.MarkExhausted("b")
	_, err := p.Next()
	if err == nil {
		t.Fatal("should error when all exhausted")
	}
}

func TestMarkHealthyRestores(t *testing.T) {
	p := NewPool([]string{"a", "b"}, nil, RoundRobin)
	p.MarkExhausted("a")
	p.MarkHealthy("a")
	if p.HealthyCount() != 2 {
		t.Errorf("expected 2 healthy, got %d", p.HealthyCount())
	}
}

func TestResetAll(t *testing.T) {
	p := NewPool([]string{"a", "b", "c"}, nil, RoundRobin)
	p.MarkExhausted("a")
	p.MarkExhausted("b")
	p.ResetAll()
	if p.HealthyCount() != 3 {
		t.Errorf("expected 3 after reset, got %d", p.HealthyCount())
	}
}

func TestEmptyPool(t *testing.T) {
	p := NewPool(nil, nil, RoundRobin)
	_, err := p.Next()
	if err == nil {
		t.Fatal("empty pool should error")
	}
}

func TestStatus(t *testing.T) {
	p := NewPool([]string{"secret-key-12345"}, []string{"minimax-1"}, RoundRobin)
	p.Next()
	status := p.Status()
	if len(status) != 1 {
		t.Fatal("expected 1 status entry")
	}
	if status[0].Label != "minimax-1" {
		t.Errorf("label: %s", status[0].Label)
	}
	if status[0].Value == "secret-key-12345" {
		t.Fatal("status should mask the key value")
	}
	if status[0].UseCount != 1 {
		t.Errorf("use count: %d", status[0].UseCount)
	}
}

func TestLabels(t *testing.T) {
	p := NewPool([]string{"a", "b"}, []string{"first", "second"}, RoundRobin)
	status := p.Status()
	if status[0].Label != "first" || status[1].Label != "second" {
		t.Errorf("labels: %s, %s", status[0].Label, status[1].Label)
	}
}

func TestLen(t *testing.T) {
	p := NewPool([]string{"a", "b", "c"}, nil, RoundRobin)
	if p.Len() != 3 {
		t.Errorf("expected 3, got %d", p.Len())
	}
}
