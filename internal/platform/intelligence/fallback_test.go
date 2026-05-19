package intelligence

import (
	"errors"
	"testing"
	"time"
)

func TestFallbackChain_FirstProviderSuccess(t *testing.T) {
	chain := NewFallbackChain(3, 30*time.Second)
	chain.AddProvider(Provider{Name: "primary", CallFn: func(p string) (string, error) {
		return "primary-response", nil
	}})

	result, err := chain.Call("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "primary-response" {
		t.Errorf("expected primary-response, got %s", result)
	}
}

func TestFallbackChain_FallbackOnFailure(t *testing.T) {
	chain := NewFallbackChain(3, 30*time.Second)
	chain.AddProvider(Provider{Name: "broken", CallFn: func(p string) (string, error) {
		return "", errors.New("down")
	}})
	chain.AddProvider(Provider{Name: "backup", CallFn: func(p string) (string, error) {
		return "backup-response", nil
	}})

	result, err := chain.Call("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "backup-response" {
		t.Errorf("expected backup-response, got %s", result)
	}
}

func TestFallbackChain_AllExhausted(t *testing.T) {
	chain := NewFallbackChain(3, 30*time.Second)
	chain.AddProvider(Provider{Name: "a", CallFn: func(p string) (string, error) {
		return "", errors.New("fail")
	}})

	_, err := chain.Call("test")
	if err == nil {
		t.Error("expected error when all providers fail")
	}
}

func TestFallbackChain_CircuitBreaker(t *testing.T) {
	chain := NewFallbackChain(2, 1*time.Hour)
	calls := 0
	chain.AddProvider(Provider{Name: "flaky", CallFn: func(p string) (string, error) {
		calls++
		return "", errors.New("fail")
	}})
	chain.AddProvider(Provider{Name: "stable", CallFn: func(p string) (string, error) {
		return "ok", nil
	}})

	chain.Call("1")
	chain.Call("2")
	chain.Call("3")

	state := chain.GetState("flaky")
	if state != ProviderOpen {
		t.Errorf("expected circuit OPEN after threshold, got %d", state)
	}
}

func TestFallbackChain_ProviderCount(t *testing.T) {
	chain := NewFallbackChain(3, 30*time.Second)
	chain.AddProvider(Provider{Name: "a", CallFn: func(p string) (string, error) { return "", nil }})
	chain.AddProvider(Provider{Name: "b", CallFn: func(p string) (string, error) { return "", nil }})

	if chain.ProviderCount() != 2 {
		t.Errorf("expected 2, got %d", chain.ProviderCount())
	}
}

func TestFallbackChain_CircuitReset(t *testing.T) {
	chain := NewFallbackChain(1, 1*time.Millisecond)
	chain.AddProvider(Provider{Name: "recover", CallFn: func(p string) (string, error) {
		return "ok", nil
	}})

	chain.recordFailure("recover")
	time.Sleep(5 * time.Millisecond)

	if !chain.isAvailable("recover") {
		t.Error("expected circuit to reset after timeout")
	}
}
