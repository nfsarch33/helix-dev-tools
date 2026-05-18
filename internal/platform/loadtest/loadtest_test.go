package loadtest

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestRun_AllSucceed(t *testing.T) {
	var counter int64
	cfg := Config{
		Concurrent: 4,
		Requests:   100,
		TargetFn: func() error {
			atomic.AddInt64(&counter, 1)
			return nil
		},
	}

	result := Run(cfg)

	if result.Total != 100 {
		t.Errorf("Expected total requests 100, got %d", result.Total)
	}
	if result.Succeeded != 100 {
		t.Errorf("Expected 100 succeeded requests, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("Expected 0 failed requests, got %d", result.Failed)
	}
	if counter != 100 {
		t.Errorf("Function not called expected number of times. Got %d, want 100", counter)
	}
}

func TestRun_SomeFail(t *testing.T) {
	var counter int64
	cfg := Config{
		Concurrent: 4,
		Requests:   100,
		TargetFn: func() error {
			atomic.AddInt64(&counter, 1)
			// Fail every 10th request
			if atomic.LoadInt64(&counter)%10 == 0 {
				return errors.New("simulated failure")
			}
			return nil
		},
	}

	result := Run(cfg)

	if result.Total != 100 {
		t.Errorf("Expected total requests 100, got %d", result.Total)
	}
	if result.Succeeded != 90 {
		t.Errorf("Expected 90 succeeded requests, got %d", result.Succeeded)
	}
	if result.Failed != 10 {
		t.Errorf("Expected 10 failed requests, got %d", result.Failed)
	}
}

func TestRun_ZeroConcurrency(t *testing.T) {
	var counter int64
	cfg := Config{
		Concurrent: 0,  // should become 1
		Requests:   50,
		TargetFn: func() error {
			atomic.AddInt64(&counter, 1)
			return nil
		},
	}

	result := Run(cfg)

	if result.Total != 50 {
		t.Errorf("Expected total requests 50, got %d", result.Total)
	}
	if result.Succeeded != 50 {
		t.Errorf("Expected 50 succeeded requests, got %d", result.Succeeded)
	}
	if counter != 50 {
		t.Errorf("Function not called expected number of times. Got %d, want 50", counter)
	}
}

func TestRun_RecordsP95(t *testing.T) {
	cfg := Config{
		Concurrent: 4,
		Requests:   100,
		TargetFn: func() error {
			return nil  // All operations succeed
		},
	}

	result := Run(cfg)

	if result.P95MS < 0 {
		t.Errorf("Expected P95 latency to be non-negative, got %d", result.P95MS)
	}
	if result.DurationMS <= 0 {
		t.Errorf("Expected non-zero total duration, got %d", result.DurationMS)
	}
}