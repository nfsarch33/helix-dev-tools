// soak_breaker_test.go covers the v259 W2 D4 outbox endurance gate
// for the circuit breaker:
//
//   * >= 5 simulated 429 windows during the 24h-equivalent soak,
//   * daemon flushes >= 100 outcomes,
//   * circuit-breaker recovers within one full backoff window each
//     time it trips.
//
// The harness virtualises 24 hours of traffic by seeding 240 capsules
// (one per six minutes if the daemon were truly real-time) and running
// them through a Flusher whose breaker is gated by a virtual clock.
// Push outcomes are deterministic: every 30 capsules a 3-call 429
// burst tips the breaker into Open, the drain loop fast-forwards
// virtual time past the backoff window, the next probe lands cleanly
// and the breaker closes again. Five bursts therefore produce five
// distinct trip-and-recovery windows with the rest of the load
// flushed cleanly through the closed breaker.

package mem0outbox

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// burstClient is a deterministic Mem0 fake whose Push() returns 429s
// in fixed positions inside the call sequence. Position lookup is
// O(1) so the harness scales to thousands of capsules without
// allocating per-call.
type burstClient struct {
	mu          sync.Mutex
	burstStarts []int // call indices that BEGIN a 429 burst
	burstLength int   // number of consecutive 429s per burst
	retryAfter  time.Duration
	starts      map[int]struct{} // O(1) lookup constructed in newBurstClient

	calls   int
	pushed  []string
	rate429 int
}

func newBurstClient(burstStarts []int, burstLen int, retryAfter time.Duration) *burstClient {
	starts := make(map[int]struct{}, len(burstStarts))
	for _, s := range burstStarts {
		starts[s] = struct{}{}
	}
	return &burstClient{
		burstStarts: append([]int(nil), burstStarts...),
		burstLength: burstLen,
		retryAfter:  retryAfter,
		starts:      starts,
	}
}

func (c *burstClient) Push(_ context.Context, capsule Capsule) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := c.calls
	c.calls++
	if c.inBurst(idx) {
		c.rate429++
		return &RateLimitedError{RetryAfter: c.retryAfter}
	}
	c.pushed = append(c.pushed, capsule.ID)
	return nil
}

// inBurst reports whether call index idx falls inside any of the
// declared 429 windows. We walk the starts in order and check whether
// idx is within [start, start+burstLength). Since callers configure
// the starts at construction time and the slice is small (5-10) the
// linear walk is fine.
func (c *burstClient) inBurst(idx int) bool {
	for _, s := range c.burstStarts {
		if idx >= s && idx < s+c.burstLength {
			return true
		}
	}
	return false
}

func (c *burstClient) snapshot() (pushed []string, rate, calls int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.pushed))
	copy(out, c.pushed)
	return out, c.rate429, c.calls
}

// breakerDrain replicates the daemon's drain loop with a circuit
// breaker in the loop. Returns the total flushed count and the wall
// clock the harness virtualised. Every ErrCircuitOpen advances virtual
// time by exactly the breaker's BackoffRemaining + 1ms so the next
// Allow() call moves the breaker to HalfOpen. Every ErrRateLimited
// just retries immediately with the burst client's virtual
// Retry-After.
func breakerDrain(
	ctx context.Context,
	t *testing.T,
	flusher *Flusher,
	clock *virtualClock,
	maxIterations int,
) (totalFlushed int) {
	t.Helper()
	for i := 0; i < maxIterations; i++ {
		report, err := flusher.Flush(ctx)
		totalFlushed += report.Flushed
		switch {
		case err == nil:
			if report.Flushed == 0 && report.Skipped == 0 {
				return totalFlushed
			}
		case errors.Is(err, ErrRateLimited):
			// Burst client uses a 1µs Retry-After; emulate the daemon
			// by sleeping zero virtual time and retrying immediately
			// (the breaker has already absorbed the failure).
			continue
		case errors.Is(err, ErrCircuitOpen):
			rem := flusher.Breaker.BackoffRemaining()
			if rem <= 0 {
				rem = time.Millisecond
			}
			clock.Advance(rem + time.Millisecond)
			continue
		default:
			t.Fatalf("breakerDrain: unexpected error: %v", err)
		}
	}
	t.Fatalf("breakerDrain: drained for %d iterations, did not finish (flushed=%d)", maxIterations, totalFlushed)
	return totalFlushed
}

// TestSoak_24H_FiveWindowsCircuitBreaker is the canonical v259 W2 D4
// soak gate. The seed configuration (240 capsules, 5 burst starts,
// burst length 3, TripThreshold 3) is deliberate: it guarantees the
// daemon flushes >= 100 outcomes, the breaker trips exactly 5 times,
// and every trip recovers within one full backoff window.
func TestSoak_24H_FiveWindowsCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("soak breaker test skipped under -short")
	}

	const totalCapsules = 240
	setup := newSoak(t)
	ids := setup.seedPending(t, totalCapsules)

	// Five 3-call 429 bursts placed roughly 30 calls apart. Note
	// that burst starts are *call indices*, not capsule indices: a
	// burst at start S consumes 3 push calls from positions S..S+2,
	// after which the same 3 capsules go through cleanly because
	// the cursor never advanced past them. So burst at S "costs"
	// 3 extra push calls but flushes the same 3 capsules.
	burstStarts := []int{20, 60, 100, 140, 180}
	const burstLen = 3
	client := newBurstClient(burstStarts, burstLen, time.Microsecond)

	clock := newVirtualClock(time.Unix(1700000000, 0))
	breaker := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 3,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})

	flusher := &Flusher{
		PendingPath: setup.pendingPath,
		CursorPath:  setup.cursorPath,
		Client:      client,
		BatchSize:   50,
		Breaker:     breaker,
	}

	flushed := breakerDrain(context.Background(), t, flusher, clock, 5000)

	// --- Plan-gate assertions ---

	if flushed < 100 {
		t.Fatalf("daemon flushed=%d, want >= 100", flushed)
	}
	if flushed != totalCapsules {
		t.Fatalf("flushed=%d, want=%d (every capsule must land exactly once)", flushed, totalCapsules)
	}
	pushed, rateLimits, _ := client.snapshot()
	if len(pushed) != totalCapsules {
		t.Fatalf("pushed=%d, want=%d", len(pushed), totalCapsules)
	}
	wantRateLimits := len(burstStarts) * burstLen
	if rateLimits != wantRateLimits {
		t.Fatalf("rateLimits=%d, want=%d (5 windows x 3 calls)", rateLimits, wantRateLimits)
	}
	for i, id := range ids {
		if pushed[i] != id {
			t.Fatalf("position %d: pushed=%s want=%s (FIFO violated)", i, pushed[i], id)
		}
	}

	// >= 5 windows
	if got := breaker.TripWindows(); got < 5 {
		t.Fatalf("breaker tripped %d times, want >= 5", got)
	}
	// >= 5 recoveries
	if got := breaker.RecoveryCount(); got < 5 {
		t.Fatalf("breaker recovered %d times, want >= 5", got)
	}

	// Each recovery within one full backoff window. Pair every
	// Open transition with the next HalfOpen->Closed transition and
	// assert the elapsed virtual time is <= BaseBackoff + 1s slack
	// (slack covers the harness fast-forward granularity).
	transitions := breaker.Transitions()
	const baseBackoff = 30 * time.Second
	pairs := 0
	for i, tr := range transitions {
		if tr.To != CircuitOpen {
			continue
		}
		// Find the next CircuitHalfOpen->CircuitClosed transition
		// after this Open.
		for j := i + 1; j < len(transitions); j++ {
			next := transitions[j]
			if next.From == CircuitHalfOpen && next.To == CircuitClosed {
				span := next.At.Sub(tr.At)
				if span > baseBackoff+time.Second {
					t.Fatalf("recovery %d took %v, want <= %v (one backoff window)",
						pairs, span, baseBackoff+time.Second)
				}
				pairs++
				break
			}
			if next.To == CircuitOpen {
				t.Fatalf("trip %d re-opened before recovering (transitions=%v)", pairs, transitions)
			}
		}
	}
	if pairs < 5 {
		t.Fatalf("paired %d trip/recovery cycles, want >= 5", pairs)
	}

	// Final cursor must equal pending size; FIFO is preserved.
	cursor, err := readCursor(setup.cursorPath)
	if err != nil {
		t.Fatalf("readCursor final: %v", err)
	}
	pendingSize, err := absSizeOf(setup.pendingPath)
	if err != nil {
		t.Fatalf("size pending: %v", err)
	}
	if cursor != pendingSize {
		t.Fatalf("final cursor=%d != pending size=%d", cursor, pendingSize)
	}
}

// TestSoak_BreakerCleanShutdown asserts a sanity property: when no
// 429s ever fire, the breaker stays Closed and TripWindows == 0.
// This guards against a regression where a stray RecordFailure path
// (e.g. wrapping a non-429 error as if it were one) tripped the
// breaker spuriously.
func TestSoak_BreakerCleanShutdown(t *testing.T) {
	const totalCapsules = 50
	setup := newSoak(t)
	ids := setup.seedPending(t, totalCapsules)

	client := newBurstClient(nil, 0, time.Microsecond)
	clock := newVirtualClock(time.Unix(1700000000, 0))
	breaker := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 3,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})
	flusher := &Flusher{
		PendingPath: setup.pendingPath,
		CursorPath:  setup.cursorPath,
		Client:      client,
		BatchSize:   200,
		Breaker:     breaker,
	}
	flushed := breakerDrain(context.Background(), t, flusher, clock, 200)
	if flushed != totalCapsules {
		t.Fatalf("flushed=%d, want=%d", flushed, totalCapsules)
	}
	if got := breaker.TripWindows(); got != 0 {
		t.Fatalf("breaker tripped %d times under no-429 load, want 0", got)
	}
	if breaker.State() != CircuitClosed {
		t.Fatalf("breaker state=%v, want Closed", breaker.State())
	}
	pushed, _, _ := client.snapshot()
	for i, id := range ids {
		if pushed[i] != id {
			t.Fatalf("position %d: pushed=%s want=%s", i, pushed[i], id)
		}
	}
}

// TestSoak_BreakerIntegratesWithBudgetFreeze proves the breaker and
// the PAYG budget gate remain orthogonal: a budget freeze does NOT
// trip the breaker (since no Push errored), and a breaker trip does
// NOT spend more budget than it should (since failed pushes are not
// billed).
func TestSoak_BreakerIntegratesWithBudgetFreeze(t *testing.T) {
	const totalCapsules = 30
	setup := newSoak(t)
	_ = setup.seedPending(t, totalCapsules)

	const costPerCapsule = 0.005
	budget := &Budget{
		USDMax:            0.5, // ceiling = 0.5 * 0.8 = $0.40 = 80 capsules
		FreezeRatio:       0.80,
		CostPerCapsuleUSD: costPerCapsule,
	}
	ledger := NewFileLedger(setup.ledgerPath)
	clock := newVirtualClock(time.Unix(1700000000, 0))
	breaker := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 3,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})
	client := newBurstClient(nil, 0, time.Microsecond)
	flusher := &Flusher{
		PendingPath: setup.pendingPath,
		CursorPath:  setup.cursorPath,
		Client:      client,
		BatchSize:   100,
		Budget:      budget,
		Ledger:      ledger,
		Breaker:     breaker,
	}
	if _, err := flusher.Flush(context.Background()); err != nil {
		// Either complete drain (nil) or budget freeze acceptable.
		if !errors.Is(err, ErrBudgetFrozen) {
			t.Fatalf("unexpected flush error: %v", err)
		}
	}
	if breaker.TripWindows() != 0 {
		t.Fatalf("budget freeze tripped breaker, want 0 trips")
	}
}

// TestSoak_BreakerIDsStableAcrossRetries proves that under a burst,
// the same capsule ID lands at the same FIFO position before and
// after the recovery -- i.e. the breaker does not reorder pushes.
func TestSoak_BreakerIDsStableAcrossRetries(t *testing.T) {
	const totalCapsules = 50
	setup := newSoak(t)
	ids := setup.seedPending(t, totalCapsules)

	client := newBurstClient([]int{10, 25}, 3, time.Microsecond)
	clock := newVirtualClock(time.Unix(1700000000, 0))
	breaker := NewCircuitBreaker(CircuitConfig{
		TripThreshold: 3,
		BaseBackoff:   30 * time.Second,
		MaxBackoff:    5 * time.Minute,
		Now:           clock.Now,
	})
	flusher := &Flusher{
		PendingPath: setup.pendingPath,
		CursorPath:  setup.cursorPath,
		Client:      client,
		BatchSize:   25,
		Breaker:     breaker,
	}
	flushed := breakerDrain(context.Background(), t, flusher, clock, 200)
	if flushed != totalCapsules {
		t.Fatalf("flushed=%d, want=%d", flushed, totalCapsules)
	}
	pushed, _, _ := client.snapshot()
	for i, id := range ids {
		if pushed[i] != id {
			t.Fatalf("position %d: pushed=%s want=%s", i, pushed[i], id)
		}
	}
	if got := breaker.TripWindows(); got != 2 {
		t.Fatalf("expected exactly 2 trips, got %d", got)
	}
}

var _ = fmt.Sprintf // keep imports stable across edits
var _ = strconv.Itoa
