package supervisor

import "time"

// healthyResetThreshold is the minimum continuous-up duration that
// resets a service's restart counter so the next crash starts the
// backoff sequence from 1s again. Documented in
// `cursor-config/rules/resource-guard.mdc` and the ADR-033 acceptance
// criteria for v337-1.
const healthyResetThreshold = 5 * time.Minute

// computeBackoff maps a restart count to the documented delay sequence
// 1s, 2s, 4s, 8s, 16s, 30s (capped). restarts=0 means "first restart"
// (i.e. the service crashed once), so computeBackoff(0) returns 1s.
//
// Capping at 30s keeps a runaway service from monopolising the
// supervisor without blowing out the wall-clock between attempts.
func computeBackoff(restarts int) time.Duration {
	if restarts < 0 {
		restarts = 0
	}
	const cap = 30 * time.Second
	d := time.Second
	for i := 0; i < restarts; i++ {
		d *= 2
		if d >= cap {
			return cap
		}
	}
	return d
}

// Clock is the supervisor's view of time. Production wires RealClock();
// tests inject a fake clock so backoff sleeps are deterministic.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// RealClock returns a Clock backed by the standard time package.
func RealClock() Clock {
	return realClock{}
}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
