package mem0cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatch_FlushOnThreshold(t *testing.T) {
	var flushed int64
	b := NewBatch(BatchConfig{
		Threshold:     3,
		FlushInterval: time.Hour,
		FlushFunc: func(entries []AddEntry) error {
			atomic.AddInt64(&flushed, int64(len(entries)))
			return nil
		},
	})
	defer b.Shutdown()

	b.Add(AddEntry{Content: "a"})
	b.Add(AddEntry{Content: "b"})

	if atomic.LoadInt64(&flushed) != 0 {
		t.Fatal("should not flush before threshold")
	}

	b.Add(AddEntry{Content: "c"})

	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt64(&flushed) != 3 {
		t.Fatalf("expected 3 flushed, got %d", atomic.LoadInt64(&flushed))
	}
}

func TestBatch_FlushOnTimer(t *testing.T) {
	var flushed int64
	b := NewBatch(BatchConfig{
		Threshold:     100,
		FlushInterval: 50 * time.Millisecond,
		FlushFunc: func(entries []AddEntry) error {
			atomic.AddInt64(&flushed, int64(len(entries)))
			return nil
		},
	})
	defer b.Shutdown()

	b.Add(AddEntry{Content: "timer-test"})

	time.Sleep(120 * time.Millisecond)
	if atomic.LoadInt64(&flushed) != 1 {
		t.Fatalf("expected 1 flushed by timer, got %d", atomic.LoadInt64(&flushed))
	}
}

func TestBatch_FlushOnShutdown(t *testing.T) {
	var flushed int64
	b := NewBatch(BatchConfig{
		Threshold:     100,
		FlushInterval: time.Hour,
		FlushFunc: func(entries []AddEntry) error {
			atomic.AddInt64(&flushed, int64(len(entries)))
			return nil
		},
	})

	b.Add(AddEntry{Content: "shutdown-test"})
	b.Shutdown()

	if atomic.LoadInt64(&flushed) != 1 {
		t.Fatalf("expected 1 flushed on shutdown, got %d", atomic.LoadInt64(&flushed))
	}
}

func TestBatch_ConcurrentAdds(t *testing.T) {
	var flushed int64
	b := NewBatch(BatchConfig{
		Threshold:     5,
		FlushInterval: time.Hour,
		FlushFunc: func(entries []AddEntry) error {
			atomic.AddInt64(&flushed, int64(len(entries)))
			return nil
		},
	})
	defer b.Shutdown()

	var wg sync.WaitGroup
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			b.Add(AddEntry{Content: "concurrent"})
		}(i)
	}
	wg.Wait()

	_ = b.Flush()
	total := atomic.LoadInt64(&flushed) + int64(b.Stats().Pending)
	if total < 15 {
		t.Fatalf("expected at least 15 total entries, got %d", total)
	}
}

func TestBatch_FlushError(t *testing.T) {
	b := NewBatch(BatchConfig{
		Threshold:     2,
		FlushInterval: time.Hour,
		FlushFunc: func(entries []AddEntry) error {
			return errors.New("upstream error")
		},
	})
	defer b.Shutdown()

	b.Add(AddEntry{Content: "a"})
	b.Add(AddEntry{Content: "b"})

	time.Sleep(20 * time.Millisecond)
	stats := b.Stats()
	if stats.Errors == 0 {
		t.Fatal("expected error count > 0")
	}
}

func TestBatch_EmptyFlush(t *testing.T) {
	called := false
	b := NewBatch(BatchConfig{
		Threshold:     10,
		FlushInterval: time.Hour,
		FlushFunc: func(entries []AddEntry) error {
			called = true
			return nil
		},
	})
	defer b.Shutdown()

	if err := b.Flush(); err != nil {
		t.Fatal("empty flush should not error")
	}
	if called {
		t.Fatal("flush func should not be called for empty buffer")
	}
}

func TestBatch_DefaultConfig(t *testing.T) {
	b := NewBatch(BatchConfig{})
	defer b.Shutdown()

	b.Add(AddEntry{Content: "default-test"})
	stats := b.Stats()
	if stats.Pending != 1 {
		t.Fatalf("expected 1 pending, got %d", stats.Pending)
	}
}

func TestBatch_Stats(t *testing.T) {
	b := NewBatch(BatchConfig{
		Threshold:     10,
		FlushInterval: time.Hour,
		FlushFunc:     func(entries []AddEntry) error { return nil },
	})
	defer b.Shutdown()

	b.Add(AddEntry{Content: "a"})
	b.Add(AddEntry{Content: "b"})

	stats := b.Stats()
	if stats.Pending != 2 {
		t.Fatalf("expected 2 pending, got %d", stats.Pending)
	}

	_ = b.Flush()
	stats = b.Stats()
	if stats.Flushed != 2 {
		t.Fatalf("expected 2 flushed, got %d", stats.Flushed)
	}
	if stats.Pending != 0 {
		t.Fatalf("expected 0 pending after flush, got %d", stats.Pending)
	}
}
