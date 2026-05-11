package supervisor

import (
	"context"
	"sync"
	"time"
)

// newWithClock builds a Supervisor with a custom clock for deterministic
// backoff sleeps in tests. The default real clock is replaced via
// WithClock.
func newWithClock(clk Clock) *Supervisor {
	return NewWithOptions(WithClock(clk))
}

// newWithClockAndProbe builds a Supervisor with a custom clock plus
// a fake MemoryPressureProbe. Used by TestSharedMemoryProbe.
func newWithClockAndProbe(clk Clock, probe MemoryPressureProbe) *Supervisor {
	return NewWithOptions(WithClock(clk), WithMemoryProbe(probe))
}

// fakeClock is a manual clock used in tests. Goroutines blocked on
// After are released when the test advances the clock past their
// trigger.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*fakeTimer
}

type fakeTimer struct {
	trigger time.Time
	ch      chan time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{trigger: c.now.Add(d), ch: make(chan time.Time, 1)}
	if d <= 0 {
		t.ch <- c.now
		close(t.ch)
		return t.ch
	}
	c.waiters = append(c.waiters, t)
	return t.ch
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	remaining := c.waiters[:0]
	fired := make([]*fakeTimer, 0, len(c.waiters))
	for _, w := range c.waiters {
		if !c.now.Before(w.trigger) {
			fired = append(fired, w)
		} else {
			remaining = append(remaining, w)
		}
	}
	c.waiters = remaining
	now := c.now
	c.mu.Unlock()
	for _, w := range fired {
		w.ch <- now
		close(w.ch)
	}
}

// waitForWaiters blocks until at least n waiters are registered on the
// clock. Used by tests that need to advance the clock for a backoff
// that has not yet been requested.
func (c *fakeClock) waitForWaiters(t interface{ Fatalf(string, ...any) }, n int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		got := len(c.waiters)
		c.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fakeClock: expected >=%d waiters within %v", n, timeout)
}

// fakeProbe is a manually-driven MemoryPressureProbe.
type fakeProbe struct {
	mu          sync.Mutex
	subscribers []chan MemoryPressure
}

func newFakeProbe() *fakeProbe {
	return &fakeProbe{}
}

func (p *fakeProbe) Subscribe() <-chan MemoryPressure {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan MemoryPressure, 4)
	p.subscribers = append(p.subscribers, ch)
	return ch
}

func (p *fakeProbe) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (p *fakeProbe) broadcast(mp MemoryPressure) {
	p.mu.Lock()
	subs := append([]chan MemoryPressure(nil), p.subscribers...)
	p.mu.Unlock()
	for _, s := range subs {
		select {
		case s <- mp:
		default:
		}
	}
}
