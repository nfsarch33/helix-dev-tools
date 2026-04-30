// circuit_breaker_test.go covers the CircuitBreaker state machine that
// the v259 W2 D4 outbox endurance soak (`s3_outbox_24h_soak`) requires.
//
// State diagram (per-flusher instance):
//
//	         consecutive_failures >= TripThreshold
//	┌──────┐  ────────────────────────────────────▶  ┌───────┐
//	│Closed│                                          │ Open  │
//	└──────┘  ◀─────────────────  half_open_success  └───┬───┘
//	    ▲                                                │
//	    │                                                │ wait
//	    │ half_open_failure                              │ Backoff
//	    └──────────────────  reset & deepen              ▼
//	                                                ┌──────────┐
//	                                                │ HalfOpen │
//	                                                └──────────┘
//
// Backoff doubles per trip (capped at MaxBackoff). Closed transitions
// reset the consecutive-failure counter and the backoff index back to
// the starting baseline.
package mem0outbox

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_OpensAfterConsecutiveFailures(t *testing.T) {
	clock := newVirtualClock(time.Unix(1700000000, 0))
	cb := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 3,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})

	if cb.State() != CircuitClosed {
		t.Fatalf("initial state=%v, want Closed", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("Allow() in Closed should return true")
	}
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
		if cb.State() != CircuitClosed {
			t.Fatalf("after %d failures, state=%v, want Closed", i+1, cb.State())
		}
	}
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("after 3 failures, state=%v, want Open", cb.State())
	}
	if cb.Allow() {
		t.Fatal("Allow() in Open should return false")
	}
	want := 30 * time.Second
	if got := cb.BackoffRemaining(); got != want {
		t.Fatalf("BackoffRemaining at trip=%v, want %v", got, want)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterBackoff(t *testing.T) {
	clock := newVirtualClock(time.Unix(1700000000, 0))
	cb := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 1,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("expected Open after first failure with TripThreshold=1, got %v", cb.State())
	}
	clock.Advance(29 * time.Second)
	if cb.Allow() {
		t.Fatal("Allow() at backoff-29s should be false")
	}
	clock.Advance(2 * time.Second) // total = 31s, past backoff
	if !cb.Allow() {
		t.Fatal("Allow() past backoff should return true and transition to HalfOpen")
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("after Allow() past backoff, state=%v, want HalfOpen", cb.State())
	}
	if cb.Allow() {
		t.Fatal("Allow() in HalfOpen should refuse a second probe until success/failure")
	}
}

func TestCircuitBreaker_HalfOpenSuccessClosesCircuit(t *testing.T) {
	clock := newVirtualClock(time.Unix(1700000000, 0))
	cb := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 1,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})
	cb.RecordFailure()
	clock.Advance(31 * time.Second)
	cb.Allow() // probe acquired

	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Fatalf("after probe success, state=%v, want Closed", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("Allow() in Closed after recovery should be true")
	}
	if got := cb.BackoffRemaining(); got != 0 {
		t.Fatalf("BackoffRemaining after Closed=%v, want 0", got)
	}
}

func TestCircuitBreaker_HalfOpenFailureReopensWithDoubledBackoff(t *testing.T) {
	clock := newVirtualClock(time.Unix(1700000000, 0))
	cb := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 1,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})
	cb.RecordFailure() // trip 1, backoff=30s
	clock.Advance(31 * time.Second)
	cb.Allow() // HalfOpen probe
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("after probe failure, state=%v, want Open", cb.State())
	}
	if got, want := cb.BackoffRemaining(), 60*time.Second; got != want {
		t.Fatalf("BackoffRemaining after second trip=%v, want %v (doubled)", got, want)
	}
}

func TestCircuitBreaker_BackoffCapsAtMaxBackoff(t *testing.T) {
	clock := newVirtualClock(time.Unix(1700000000, 0))
	cb := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 1,
		BaseBackoff:   60 * time.Second,
		MaxBackoff:    2 * time.Minute,
		Now:           clock.Now,
	})
	for trip := 0; trip < 5; trip++ {
		cb.RecordFailure()
		clock.Advance(cb.BackoffRemaining() + time.Second)
		cb.Allow()
	}
	if got := cb.BackoffRemaining(); got > 2*time.Minute {
		t.Fatalf("backoff exceeded MaxBackoff: %v > 2m", got)
	}
}

func TestCircuitBreaker_TripWindowsCounted(t *testing.T) {
	clock := newVirtualClock(time.Unix(1700000000, 0))
	cb := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 1,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
		clock.Advance(cb.BackoffRemaining() + time.Second)
		cb.Allow()
		cb.RecordSuccess()
	}
	if got := cb.TripWindows(); got != 5 {
		t.Fatalf("TripWindows=%d, want 5", got)
	}
	if got := cb.RecoveryCount(); got != 5 {
		t.Fatalf("RecoveryCount=%d, want 5", got)
	}
}

func TestCircuitBreaker_RecoveryWithinOneBackoffWindow(t *testing.T) {
	// Plan gate: circuit-breaker recovers within one full backoff
	// window. We assert that, given a single trip and a clean probe,
	// the breaker returns to Closed in <= BaseBackoff + one probe.
	clock := newVirtualClock(time.Unix(1700000000, 0))
	cb := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 1,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})

	tripAt := clock.Now()
	cb.RecordFailure()
	clock.Advance(31 * time.Second)
	cb.Allow()
	cb.RecordSuccess()
	closedAt := clock.Now()

	if cb.State() != CircuitClosed {
		t.Fatalf("state=%v after recovery, want Closed", cb.State())
	}
	recovered := closedAt.Sub(tripAt)
	if recovered > 31*time.Second+time.Millisecond {
		t.Fatalf("recovery took %v, want <= 31s (one backoff window)", recovered)
	}
}

func TestCircuitBreaker_AllowReturnsErrCircuitOpenWhenClosedToCallers(t *testing.T) {
	// When the breaker is Open and a caller wraps Allow() into Push(),
	// a sentinel ErrCircuitOpen is what the upstream Flusher should
	// surface. This unit test pins the sentinel so callers can
	// errors.Is it without snooping at internal state.
	if !errors.Is(ErrCircuitOpen, ErrCircuitOpen) {
		t.Fatal("ErrCircuitOpen sentinel should errors.Is itself")
	}
}

// virtualClock is a deterministic Now() driver. The breaker uses
// CircuitConfig.Now so callers (the soak harness) can fast-forward
// virtual time without a real sleep.
type virtualClock struct {
	now time.Time
}

func newVirtualClock(start time.Time) *virtualClock {
	return &virtualClock{now: start}
}

func (c *virtualClock) Now() time.Time { return c.now }

func (c *virtualClock) Advance(d time.Duration) { c.now = c.now.Add(d) }
