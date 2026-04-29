// soak_test.go is the v259 W2 D4 outbox endurance suite. It does NOT
// literally take 24 hours; instead it synthesises the same workload a
// real 24h horizon would produce on the EvoLoop daemon (~3000 capsules
// across all writers) and drives the existing Flusher through every
// adversarial regime the plan calls out:
//
//  1. Synthetic 429 pressure: ~5 % of pushes return HTTP 429 with a
//     Retry-After. Cursor must NOT advance past a 429'd capsule, and
//     after the simulated wait the same capsule must land successfully
//     on the next flush.
//
//  2. PAYG-cap enforcement: the same load is run with MEM0_PAYG_USD_MAX
//     set to a low ceiling so the freeze ratio fires mid-soak. The
//     freeze MUST stop the flusher cold (ErrBudgetFrozen surfaces),
//     the cursor MUST be exactly where the freeze fired, and a
//     simulated operator-bumped cap MUST allow the remainder to drain.
//
//  3. End-state invariant: every capsule the producer wrote MUST have
//     made exactly one successful POST to Mem0 by the end of the soak,
//     no duplicates, no drops, no out-of-order surprises.
//
// The fake Mem0 client is deterministic (seed-driven) so a CI run on
// any machine produces the same sequence of 429s and the same final
// flushed-count. The test runs in seconds, not hours, by collapsing
// every Retry-After to a 1-microsecond shim that callers respect by
// looping Flush until io.EOF or ErrBudgetFrozen surfaces.
//
// All timing is virtual; no goroutines or real sleeps are used.

package mem0outbox

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// absSizeOf returns the byte size of path. Used by the soak suite to
// assert that the cursor advanced exactly to the end of pending.jsonl
// after a successful drain.
func absSizeOf(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// soakClient is a deterministic Mem0 fake. It records every successful
// push, returns 429s on the configured probability, and supports a
// per-test "promote freeze" hook so callers can assert that the
// PAYG-cap path exits the loop on its own.
type soakClient struct {
	mu          sync.Mutex
	r           *rand.Rand
	probability float64
	retryAfter  time.Duration

	pushed     []string // capsule IDs in order of successful POST
	rateLimits int      // number of 429s synthesised
	calls      int      // total Push() calls
}

func newSoakClient(seed int64, p float64, retryAfter time.Duration) *soakClient {
	return &soakClient{
		r:           rand.New(rand.NewSource(seed)),
		probability: p,
		retryAfter:  retryAfter,
	}
}

func (c *soakClient) Push(_ context.Context, capsule Capsule) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.r.Float64() < c.probability {
		c.rateLimits++
		return &RateLimitedError{RetryAfter: c.retryAfter}
	}
	c.pushed = append(c.pushed, capsule.ID)
	return nil
}

func (c *soakClient) snapshot() (pushed []string, rate, total int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.pushed))
	copy(out, c.pushed)
	return out, c.rateLimits, c.calls
}

// soakSetup encapsulates a freshly-seeded outbox + cursor + ledger
// triple inside t.TempDir() so each soak scenario is hermetic.
type soakSetup struct {
	root        string
	pendingPath string
	cursorPath  string
	ledgerPath  string
}

func newSoak(t *testing.T) soakSetup {
	t.Helper()
	dir := t.TempDir()
	return soakSetup{
		root:        dir,
		pendingPath: filepath.Join(dir, "pending.jsonl"),
		cursorPath:  filepath.Join(dir, "cursor"),
		ledgerPath:  filepath.Join(dir, "ledger"),
	}
}

// seedPending appends n synthetic capsules to the outbox, returning
// the IDs in the order they were written. IDs are zero-padded so a
// lex-sorted bug surfaces immediately.
func (s soakSetup) seedPending(t *testing.T, n int) []string {
	t.Helper()
	w, err := NewWriter(s.pendingPath)
	if err != nil {
		t.Fatalf("seedPending: NewWriter: %v", err)
	}
	defer func() {
		if cerr := w.Close(); cerr != nil {
			t.Fatalf("seedPending: Close: %v", cerr)
		}
	}()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("soak-%05d", i)
		ids[i] = id
		if err := w.Append(Capsule{
			ID:        id,
			AppID:     "cursor-global-kb",
			UserID:    "soak-runner",
			Text:      "soak " + strconv.Itoa(i),
			CreatedAt: time.Unix(int64(1700000000+i), 0),
		}); err != nil {
			t.Fatalf("seedPending: Append: %v", err)
		}
	}
	return ids
}

// drain repeatedly calls Flush() against client until either:
//   - flushReport reports zero progress and the underlying file has
//     been fully drained (cursor == size), in which case we return
//     a clean nil error,
//   - or a non-rate-limit, non-budget-frozen error surfaces, which
//     we propagate verbatim,
//   - or budgetFreezeAcceptable && ErrBudgetFrozen surfaces, which
//     we surface so the caller can assert the freeze cleanly.
//
// Per-flush 429s are silently retried with the soak client's virtual
// Retry-After (which is sub-microsecond, never a real sleep).
func drain(
	ctx context.Context,
	t *testing.T,
	flusher *Flusher,
	maxIterations int,
	allowFreeze bool,
) (totalFlushed int, frozenAt int, err error) {
	t.Helper()
	for i := 0; i < maxIterations; i++ {
		report, ferr := flusher.Flush(ctx)
		totalFlushed += report.Flushed
		switch {
		case ferr == nil:
			if report.Flushed == 0 && report.Skipped == 0 {
				return totalFlushed, 0, nil
			}
		case errors.Is(ferr, ErrRateLimited):
			// Fast-forward virtual time. The flusher already wrote
			// the cursor at the failure offset.
			continue
		case errors.Is(ferr, ErrBudgetFrozen):
			if allowFreeze {
				return totalFlushed, totalFlushed, ferr
			}
			return totalFlushed, totalFlushed, fmt.Errorf("unexpected budget freeze: %w", ferr)
		default:
			return totalFlushed, totalFlushed, ferr
		}
	}
	return totalFlushed, totalFlushed, fmt.Errorf("drain: exceeded %d iterations", maxIterations)
}

// TestSoak_24HEquivalent_PressureAnd429 covers the v259 W2 D4 gate's
// first half: under realistic 24h-of-traffic load with synthetic 429
// pressure, the cursor never advances past a failed capsule, the
// flusher resumes exactly, and every input capsule lands exactly once.
func TestSoak_24HEquivalent_PressureAnd429(t *testing.T) {
	if testing.Short() {
		t.Skip("soak test skipped under -short")
	}
	const totalCapsules = 3000
	setup := newSoak(t)
	ids := setup.seedPending(t, totalCapsules)

	client := newSoakClient(0xC0FFEE, 0.05, time.Microsecond)
	flusher := &Flusher{
		PendingPath: setup.pendingPath,
		CursorPath:  setup.cursorPath,
		Client:      client,
		BatchSize:   200,
	}

	flushed, _, err := drain(context.Background(), t, flusher, 5000, false)
	if err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if flushed != totalCapsules {
		t.Fatalf("flushed=%d want=%d", flushed, totalCapsules)
	}
	pushed, rateLimits, calls := client.snapshot()
	if len(pushed) != totalCapsules {
		t.Fatalf("client pushed=%d want=%d", len(pushed), totalCapsules)
	}
	if rateLimits == 0 {
		t.Fatal("soak generated zero 429s; probability seed broken")
	}
	if calls != totalCapsules+rateLimits {
		t.Fatalf("calls=%d should equal pushed(%d) + 429(%d) = %d",
			calls, totalCapsules, rateLimits, totalCapsules+rateLimits)
	}
	// Every capsule the producer wrote must land exactly once, in
	// FIFO order (NDJSON cursor is byte-monotonic so any reordering
	// would mean the flusher rewound).
	if len(pushed) != len(ids) {
		t.Fatalf("len(pushed)=%d != len(ids)=%d", len(pushed), len(ids))
	}
	for i, id := range ids {
		if pushed[i] != id {
			t.Fatalf("position %d: pushed=%s want=%s (FIFO violated)",
				i, pushed[i], id)
		}
	}
	// Final cursor must equal pending.jsonl size.
	cursor, err := readCursor(setup.cursorPath)
	if err != nil {
		t.Fatalf("readCursor final: %v", err)
	}
	info, err := absSizeOf(setup.pendingPath)
	if err != nil {
		t.Fatalf("size pending: %v", err)
	}
	if cursor != info {
		t.Fatalf("final cursor=%d != pending size=%d", cursor, info)
	}
}

// TestSoak_PAYGCapFreezeAndOperatorRecovery covers the second half:
// MEM0_PAYG_USD_MAX is set so the freeze fires mid-load. The flusher
// must stop with ErrBudgetFrozen, the cursor must be exactly where it
// fired (no rolled-back successful push), and bumping the cap must
// release the freeze and let the remainder drain.
func TestSoak_PAYGCapFreezeAndOperatorRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("soak test skipped under -short")
	}
	const totalCapsules = 1000
	setup := newSoak(t)
	ids := setup.seedPending(t, totalCapsules)

	const costPerCapsule = 0.005 // 0.5 cents
	const initialCap = 1.0       // $1 hard cap = $0.80 ceiling = ~160 capsules
	budget := &Budget{
		USDMax:            initialCap,
		FreezeRatio:       0.80,
		CostPerCapsuleUSD: costPerCapsule,
	}
	ledger := NewFileLedger(setup.ledgerPath)

	// Use 0% rate limit here so the budget gate is the only stopper.
	client := newSoakClient(0xBADCAFE, 0.0, time.Microsecond)
	flusher := &Flusher{
		PendingPath: setup.pendingPath,
		CursorPath:  setup.cursorPath,
		Client:      client,
		BatchSize:   100,
		Budget:      budget,
		Ledger:      ledger,
	}

	// Drain until the freeze fires.
	flushedFirst, _, err := drain(context.Background(), t, flusher, 200, true)
	if err == nil || !errors.Is(err, ErrBudgetFrozen) {
		t.Fatalf("expected ErrBudgetFrozen, got %v (flushed=%d)", err, flushedFirst)
	}

	pushedFirst, _, _ := client.snapshot()
	if len(pushedFirst) != flushedFirst {
		t.Fatalf("client pushed=%d != flushed=%d", len(pushedFirst), flushedFirst)
	}
	// Sanity: freeze must have fired BEFORE we drained the whole load.
	if flushedFirst >= totalCapsules {
		t.Fatalf("freeze never fired; flushed all %d capsules", totalCapsules)
	}
	// Ledger must reflect a spend at-or-above the ceiling that caused
	// the freeze.
	spent, err := ledger.Read()
	if err != nil {
		t.Fatalf("ledger.Read after freeze: %v", err)
	}
	if spent < budget.Ceiling() {
		t.Fatalf("freeze fired but spent $%.4f < ceiling $%.4f",
			spent, budget.Ceiling())
	}

	// Cursor must point at the byte after the LAST successful push.
	// The simplest invariant: cursor must NOT be past pending size and
	// must NOT be at zero.
	cursorAtFreeze, err := readCursor(setup.cursorPath)
	if err != nil {
		t.Fatalf("readCursor after freeze: %v", err)
	}
	if cursorAtFreeze == 0 {
		t.Fatal("cursor at zero after freeze (no progress recorded)")
	}
	pendingSize, err := absSizeOf(setup.pendingPath)
	if err != nil {
		t.Fatalf("size pending: %v", err)
	}
	if cursorAtFreeze >= pendingSize {
		t.Fatalf("cursor=%d >= pending=%d after freeze (drained too far)",
			cursorAtFreeze, pendingSize)
	}

	// Operator action: bump the USD cap. Same flusher instance, no
	// daemon restart; the next Flush() must observe the new ceiling.
	budget.USDMax = 100.0 // raise to a value the soak cannot exceed

	flushedSecond, _, err := drain(context.Background(), t, flusher, 200, false)
	if err != nil {
		t.Fatalf("post-recovery drain: %v", err)
	}
	if flushedFirst+flushedSecond != totalCapsules {
		t.Fatalf("total flushed=%d (pre=%d post=%d) want=%d",
			flushedFirst+flushedSecond, flushedFirst, flushedSecond, totalCapsules)
	}

	// End-state invariants: client saw every capsule exactly once, in
	// FIFO order; final cursor == pending size; ledger is monotonic.
	pushedAll, _, _ := client.snapshot()
	if len(pushedAll) != totalCapsules {
		t.Fatalf("client pushed total=%d want=%d", len(pushedAll), totalCapsules)
	}
	for i, id := range ids {
		if pushedAll[i] != id {
			t.Fatalf("position %d: pushed=%s want=%s (FIFO violated across freeze)",
				i, pushedAll[i], id)
		}
	}
	finalCursor, err := readCursor(setup.cursorPath)
	if err != nil {
		t.Fatalf("readCursor final: %v", err)
	}
	if finalCursor != pendingSize {
		t.Fatalf("final cursor=%d != pending size=%d", finalCursor, pendingSize)
	}
	finalSpend, err := ledger.Read()
	if err != nil {
		t.Fatalf("ledger.Read final: %v", err)
	}
	if finalSpend < spent {
		t.Fatalf("ledger went backwards: pre=%f post=%f", spent, finalSpend)
	}
}

// TestSoak_Combined_429AndBudgetFreeze stresses both regimes
// simultaneously: 5% rate-limit pressure AND a low PAYG cap that is
// bumped twice mid-soak by the operator. Asserts the same end-state
// invariants as above.
func TestSoak_Combined_429AndBudgetFreeze(t *testing.T) {
	if testing.Short() {
		t.Skip("soak test skipped under -short")
	}
	const totalCapsules = 1500
	setup := newSoak(t)
	ids := setup.seedPending(t, totalCapsules)

	budget := &Budget{
		USDMax:            2.0, // tiny initial cap to force at least one freeze
		FreezeRatio:       0.80,
		CostPerCapsuleUSD: 0.005,
	}
	ledger := NewFileLedger(setup.ledgerPath)
	client := newSoakClient(0xFEED, 0.05, time.Microsecond)

	flusher := &Flusher{
		PendingPath: setup.pendingPath,
		CursorPath:  setup.cursorPath,
		Client:      client,
		BatchSize:   100,
		Budget:      budget,
		Ledger:      ledger,
	}

	bumps := 0
	totalFlushed := 0
	for attempt := 0; attempt < 20; attempt++ {
		f, _, err := drain(context.Background(), t, flusher, 5000, true)
		totalFlushed += f
		if err == nil {
			break
		}
		if !errors.Is(err, ErrBudgetFrozen) {
			t.Fatalf("attempt %d unexpected drain error: %v", attempt, err)
		}
		// Operator bumps the cap. Each bump doubles the headroom so
		// we converge in O(log) bumps no matter the load.
		budget.USDMax *= 2
		bumps++
	}
	if bumps == 0 {
		t.Fatal("budget freeze never fired; soak too small")
	}
	if totalFlushed != totalCapsules {
		t.Fatalf("totalFlushed=%d want=%d (bumps=%d)", totalFlushed, totalCapsules, bumps)
	}
	pushedAll, rateLimits, _ := client.snapshot()
	if len(pushedAll) != totalCapsules {
		t.Fatalf("client pushed=%d want=%d", len(pushedAll), totalCapsules)
	}
	if rateLimits == 0 {
		t.Fatal("expected at least one 429 in combined soak")
	}
	for i, id := range ids {
		if pushedAll[i] != id {
			t.Fatalf("position %d: pushed=%s want=%s (FIFO violated)",
				i, pushedAll[i], id)
		}
	}
	finalCursor, err := readCursor(setup.cursorPath)
	if err != nil {
		t.Fatalf("readCursor final: %v", err)
	}
	pendingSize, err := absSizeOf(setup.pendingPath)
	if err != nil {
		t.Fatalf("size pending: %v", err)
	}
	if finalCursor != pendingSize {
		t.Fatalf("final cursor=%d != pending size=%d", finalCursor, pendingSize)
	}
}
