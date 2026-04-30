package mem0outbox

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned by Flusher.Flush when the breaker is in
// the Open state at the moment the daemon attempts a flush. Callers
// MAY treat this identically to ErrRateLimited (sleep and retry) but
// the sentinel is distinct so dashboards can plot
// breaker-trip-vs-upstream-429 separately.
var ErrCircuitOpen = errors.New("mem0 outbox circuit breaker open")

// CircuitState enumerates the three reachable breaker states.
type CircuitState int

const (
	// CircuitClosed is the steady-state: pushes pass through and
	// breaker accounting only flips to Open after consecutive
	// failures cross TripThreshold.
	CircuitClosed CircuitState = iota
	// CircuitOpen rejects every push until the current backoff
	// window expires. Once Allow() is called past the deadline the
	// breaker transitions to HalfOpen automatically.
	CircuitOpen
	// CircuitHalfOpen permits exactly one probe through. The probe's
	// outcome (RecordSuccess / RecordFailure) decides whether the
	// breaker closes again (success) or re-opens with a deepened
	// backoff window (failure).
	CircuitHalfOpen
)

// String returns the short name for state, matching the names used in
// telemetry capsules.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitConfig captures the breaker's tuning knobs. All durations are
// in real wall-clock terms; tests inject a virtual Now() so we can
// fast-forward without sleeping.
type CircuitConfig struct {
	// TripThreshold is the number of consecutive failures (in
	// CircuitClosed) that flip the breaker to Open. Defaults to 3
	// when zero.
	TripThreshold int
	// BaseBackoff is the first Open-state hold time. Subsequent trips
	// double this value (capped at MaxBackoff).
	BaseBackoff time.Duration
	// MaxBackoff is the upper bound on the exponential backoff. The
	// breaker never holds Open longer than this.
	MaxBackoff time.Duration
	// Now is the clock source. When nil, time.Now is used.
	Now func() time.Time
}

// CircuitBreaker is the per-flusher state machine described above.
// Concurrent callers must hold the receiver's mutex through any
// public method; the type is therefore safe for concurrent use.
type CircuitBreaker struct {
	mu sync.Mutex

	cfg            CircuitConfig
	state          CircuitState
	consecFails    int
	tripIndex      int       // doubling exponent; reset on close
	openedAt       time.Time // wall-clock when state flipped to Open
	currentBackoff time.Duration

	tripWindows int // total Open-transitions ever
	recoveries  int // total HalfOpen->Closed transitions ever
	probeIssued bool

	// transitions records every state change with the wall-clock
	// (per cfg.Now) at the moment of transition. Tests use this to
	// pair trip events with recoveries without observing State()
	// while another method holds the mutex.
	transitions []Transition
}

// Transition is a single state-change event in a breaker's history.
// At points to the wall-clock (per cfg.Now) when the transition
// happened.
type Transition struct {
	From CircuitState
	To   CircuitState
	At   time.Time
}

// NewCircuitBreaker constructs a closed breaker with the given config.
// Defaults: TripThreshold=3, BaseBackoff=30s, MaxBackoff=5m, Now=time.Now.
func NewCircuitBreaker(cfg CircuitConfig) *CircuitBreaker {
	if cfg.TripThreshold <= 0 {
		cfg.TripThreshold = 3
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 30 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 5 * time.Minute
	}
	if cfg.MaxBackoff < cfg.BaseBackoff {
		cfg.MaxBackoff = cfg.BaseBackoff
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &CircuitBreaker{cfg: cfg, state: CircuitClosed}
}

// State returns the current breaker state.
func (c *CircuitBreaker) State() CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Allow advances the breaker's state machine in response to the
// caller's intent to issue a push. Returns true when the push should
// proceed. Allow is the only place where Open->HalfOpen transitions
// happen; success/failure of the resulting push is reported via
// RecordSuccess / RecordFailure.
func (c *CircuitBreaker) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if c.cfg.Now().Sub(c.openedAt) < c.currentBackoff {
			return false
		}
		c.transitionLocked(CircuitOpen, CircuitHalfOpen)
		c.probeIssued = true
		return true
	case CircuitHalfOpen:
		if c.probeIssued {
			return false
		}
		c.probeIssued = true
		return true
	}
	return false
}

// transitionLocked records a state change. Caller must hold c.mu.
func (c *CircuitBreaker) transitionLocked(from, to CircuitState) {
	c.state = to
	c.transitions = append(c.transitions, Transition{From: from, To: to, At: c.cfg.Now()})
}

// RecordSuccess advances the breaker on a successful push. Closed
// stays Closed (and resets the consecutive-fail counter). HalfOpen
// flips to Closed with a fully-reset backoff.
func (c *CircuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.state {
	case CircuitClosed:
		c.consecFails = 0
	case CircuitHalfOpen:
		c.transitionLocked(CircuitHalfOpen, CircuitClosed)
		c.consecFails = 0
		c.tripIndex = 0
		c.currentBackoff = 0
		c.probeIssued = false
		c.recoveries++
	case CircuitOpen:
		// Defensive: a success while Open should not happen, but if
		// it does (a stale probe completed) we treat it as a recovery.
		c.transitionLocked(CircuitOpen, CircuitClosed)
		c.consecFails = 0
		c.tripIndex = 0
		c.currentBackoff = 0
		c.probeIssued = false
		c.recoveries++
	}
}

// RecordFailure advances the breaker on a failed push. The semantics
// depend on the source state:
//
//   - Closed: increment the consecutive-fail counter; if it crosses
//     TripThreshold, flip to Open with the BaseBackoff hold.
//   - HalfOpen: re-open immediately with a doubled backoff (capped at
//     MaxBackoff).
//   - Open: this should not happen because Allow() guards entry, but
//     to be defensive we leave the state alone.
func (c *CircuitBreaker) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.state {
	case CircuitClosed:
		c.consecFails++
		if c.consecFails >= c.cfg.TripThreshold {
			c.openLocked()
		}
	case CircuitHalfOpen:
		c.tripIndex++
		c.openLocked()
	case CircuitOpen:
		// no-op
	}
}

// openLocked transitions to Open, doubling the backoff each successive
// trip. Caller must hold c.mu.
func (c *CircuitBreaker) openLocked() {
	prev := c.state
	c.transitionLocked(prev, CircuitOpen)
	c.openedAt = c.cfg.Now()
	c.probeIssued = false
	backoff := c.cfg.BaseBackoff
	for i := 0; i < c.tripIndex; i++ {
		next := backoff * 2
		if next > c.cfg.MaxBackoff || next <= 0 {
			backoff = c.cfg.MaxBackoff
			break
		}
		backoff = next
	}
	if backoff > c.cfg.MaxBackoff {
		backoff = c.cfg.MaxBackoff
	}
	c.currentBackoff = backoff
	c.tripWindows++
	c.consecFails = 0
}

// BackoffRemaining returns how much wall-clock time the breaker has
// left in its Open window. Returns 0 when Closed or when the deadline
// has already elapsed.
func (c *CircuitBreaker) BackoffRemaining() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != CircuitOpen {
		return 0
	}
	rem := c.currentBackoff - c.cfg.Now().Sub(c.openedAt)
	if rem < 0 {
		return 0
	}
	return rem
}

// TripWindows returns the cumulative number of Open transitions.
// Tests assert this matches the soak plan's "≥5 windows" gate.
func (c *CircuitBreaker) TripWindows() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tripWindows
}

// RecoveryCount returns the cumulative number of HalfOpen->Closed
// transitions. A recovery proves the breaker exited Open within one
// full backoff window.
func (c *CircuitBreaker) RecoveryCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.recoveries
}

// Transitions returns a copy of the recorded state-change history.
// Tests use this to pair Open transitions with the matching
// HalfOpen->Closed recoveries without observing State() while
// another method holds the breaker's mutex.
func (c *CircuitBreaker) Transitions() []Transition {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Transition, len(c.transitions))
	copy(out, c.transitions)
	return out
}
