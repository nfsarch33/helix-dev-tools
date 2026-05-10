package pool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_ExecutesConcurrently(t *testing.T) {
	p := New(3)
	var running atomic.Int32
	var maxSeen atomic.Int32

	tasks := make([]Task, 10)
	for i := range tasks {
		tasks[i] = func(ctx context.Context) error {
			cur := running.Add(1)
			for {
				old := maxSeen.Load()
				if cur <= old || maxSeen.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
			return nil
		}
	}

	errs := p.Run(context.Background(), tasks)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	if maxSeen.Load() > 3 {
		t.Errorf("concurrency exceeded limit: max %d seen", maxSeen.Load())
	}
}

func TestPool_CollectsErrors(t *testing.T) {
	p := New(2)
	tasks := []Task{
		func(ctx context.Context) error { return errors.New("fail-1") },
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return errors.New("fail-2") },
	}

	errs := p.Run(context.Background(), tasks)
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}
}

func TestPool_RespectsContext(t *testing.T) {
	p := New(2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var ran atomic.Int32
	tasks := []Task{
		func(ctx context.Context) error {
			ran.Add(1)
			return ctx.Err()
		},
	}

	_ = p.Run(ctx, tasks)
	// task may or may not execute depending on scheduling, but no hang
}

func TestPool_ZeroConcurrency(t *testing.T) {
	p := New(0)
	if p.concurrency != 1 {
		t.Errorf("zero concurrency should default to 1, got %d", p.concurrency)
	}
}

func TestPool_EmptyTasks(t *testing.T) {
	p := New(4)
	errs := p.Run(context.Background(), nil)
	if len(errs) != 0 {
		t.Errorf("empty tasks should return no errors, got %d", len(errs))
	}
}
