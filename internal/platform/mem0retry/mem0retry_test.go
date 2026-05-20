package mem0retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), DefaultConfig(), func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	cfg := Config{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond, JitterPct: 0}
	calls := 0
	err := Do(context.Background(), cfg, func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return NewRetryableError(errors.New("502"), 502)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoMaxRetriesExceeded(t *testing.T) {
	cfg := Config{MaxAttempts: 2, BaseDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond, JitterPct: 0}
	err := Do(context.Background(), cfg, func(ctx context.Context) error {
		return NewRetryableError(errors.New("always fails"), 502)
	})
	if !errors.Is(err, ErrMaxRetries) {
		t.Fatalf("expected ErrMaxRetries, got: %v", err)
	}
}

func TestDoNonRetryableError(t *testing.T) {
	cfg := Config{MaxAttempts: 5, BaseDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond}
	calls := 0
	err := Do(context.Background(), cfg, func(ctx context.Context) error {
		calls++
		return errors.New("auth failure 401")
	})
	if calls != 1 {
		t.Errorf("expected 1 call (no retry on non-retryable), got %d", calls)
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDoContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Do(ctx, DefaultConfig(), func(ctx context.Context) error {
		return NewRetryableError(errors.New("should not run"), 502)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{NewRetryableError(errors.New("x"), 502), true},
		{NewRetryableError(errors.New("x"), 503), true},
		{NewRetryableError(errors.New("x"), 429), true},
		{NewRetryableError(errors.New("x"), 401), false},
		{NewRetryableError(errors.New("x"), 500), false},
		{errors.New("plain error"), false},
	}
	for _, tt := range tests {
		if got := IsRetryable(tt.err); got != tt.want {
			t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestComputeDelay(t *testing.T) {
	cfg := Config{BaseDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second, JitterPct: 0}
	d0 := computeDelay(cfg, 0)
	d1 := computeDelay(cfg, 1)
	d2 := computeDelay(cfg, 2)
	if d0 != 100*time.Millisecond {
		t.Errorf("attempt 0: %v", d0)
	}
	if d1 != 200*time.Millisecond {
		t.Errorf("attempt 1: %v", d1)
	}
	if d2 != 400*time.Millisecond {
		t.Errorf("attempt 2: %v", d2)
	}
}

func TestComputeDelayCapped(t *testing.T) {
	cfg := Config{BaseDelay: 1 * time.Second, MaxDelay: 2 * time.Second, JitterPct: 0}
	d := computeDelay(cfg, 10)
	if d != 2*time.Second {
		t.Errorf("expected cap at 2s, got %v", d)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxAttempts != 3 {
		t.Errorf("max attempts: %d", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 500*time.Millisecond {
		t.Errorf("base delay: %v", cfg.BaseDelay)
	}
}
