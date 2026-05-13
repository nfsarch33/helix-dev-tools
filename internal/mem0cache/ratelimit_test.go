package mem0cache

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_AllowBurst(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{AddPerMinute: 3, SearchPerMinute: 5})

	for i := 0; i < 3; i++ {
		if !rl.AllowAdd() {
			t.Fatalf("add %d should be allowed (burst)", i)
		}
	}

	if rl.AllowAdd() {
		t.Fatal("add should be denied after burst exhausted")
	}

	for i := 0; i < 5; i++ {
		if !rl.AllowSearch() {
			t.Fatalf("search %d should be allowed (burst)", i)
		}
	}

	if rl.AllowSearch() {
		t.Fatal("search should be denied after burst exhausted")
	}
}

func TestRateLimiter_WaitAdd(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{AddPerMinute: 60, SearchPerMinute: 60})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rl.WaitAdd(ctx); err != nil {
		t.Fatalf("WaitAdd should succeed: %v", err)
	}
}

func TestRateLimiter_WaitSearch(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{AddPerMinute: 60, SearchPerMinute: 60})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rl.WaitSearch(ctx); err != nil {
		t.Fatalf("WaitSearch should succeed: %v", err)
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{AddPerMinute: 1, SearchPerMinute: 1})

	rl.AllowAdd()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := rl.WaitAdd(ctx); err == nil {
		t.Fatal("WaitAdd should fail with cancelled context")
	}
}

func TestRateLimiter_DefaultConfig(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{})

	for i := 0; i < 5; i++ {
		if !rl.AllowAdd() {
			t.Fatalf("default add %d should be allowed", i)
		}
	}

	for i := 0; i < 10; i++ {
		if !rl.AllowSearch() {
			t.Fatalf("default search %d should be allowed", i)
		}
	}
}
