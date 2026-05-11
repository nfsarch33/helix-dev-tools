package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Supervisor manages a set of Services, running them concurrently with
// per-service panic recovery, exponential backoff on crashes, optional
// shared MemoryPressureProbe, and graceful release on parent-context
// cancel.
//
// Construction goes through New() or NewWithOptions(...). All public
// methods are safe for concurrent use.
type Supervisor struct {
	mu       sync.Mutex
	services []Service
	names    map[string]struct{}
	health   map[string]*serviceState
	running  bool

	clock Clock
	probe MemoryPressureProbe
	log   *slog.Logger
}

// serviceState tracks per-service health independently of the running
// supervisor loop so Health() can return without contending on the
// service goroutine.
type serviceState struct {
	mu          sync.Mutex
	name        string
	started     bool
	lastStarted time.Time
	lastExited  time.Time
	lastError   error
	restarts    int
}

// Option mutates Supervisor construction.
type Option func(*Supervisor)

// WithClock injects a clock implementation. Production uses RealClock();
// tests inject a fake to drive backoff deterministically.
func WithClock(c Clock) Option {
	return func(s *Supervisor) { s.clock = c }
}

// WithMemoryProbe attaches a shared MemoryPressureProbe. The supervisor
// runs the probe as part of Run() and ensures it stops when the parent
// context is cancelled.
func WithMemoryProbe(p MemoryPressureProbe) Option {
	return func(s *Supervisor) { s.probe = p }
}

// WithLogger swaps the default slog destination.
func WithLogger(l *slog.Logger) Option {
	return func(s *Supervisor) { s.log = l }
}

// New constructs a Supervisor with default options (wall clock, no
// probe, package logger).
func New() *Supervisor {
	return NewWithOptions()
}

// NewWithOptions constructs a Supervisor and applies opts in order.
func NewWithOptions(opts ...Option) *Supervisor {
	s := &Supervisor{
		names:  make(map[string]struct{}),
		health: make(map[string]*serviceState),
		clock:  RealClock(),
		log:    slog.Default(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Register adds a Service to the supervisor. Returns ErrDuplicateService
// if a service with the same name is already registered. Registration
// is allowed while Run is in flight (the new service is picked up on
// the next iteration); however the typical pattern is to Register all
// services before calling Run.
func (s *Supervisor) Register(svc Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := svc.Name()
	if _, dup := s.names[name]; dup {
		return fmt.Errorf("%w: %s", ErrDuplicateService, name)
	}
	s.names[name] = struct{}{}
	s.services = append(s.services, svc)
	s.health[name] = &serviceState{name: name}
	return nil
}

// Names returns the names of all registered services in registration
// order.
func (s *Supervisor) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, 0, len(s.services))
	for _, svc := range s.services {
		out = append(out, svc.Name())
	}
	return out
}

// Health returns the most recent HealthState for the named service, or
// ErrUnknownService if the name was never registered.
func (s *Supervisor) Health(name string) (HealthState, error) {
	s.mu.Lock()
	st, ok := s.health[name]
	s.mu.Unlock()
	if !ok {
		return HealthState{}, fmt.Errorf("%w: %s", ErrUnknownService, name)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return HealthState{
		Name:        st.name,
		Started:     st.started,
		LastStarted: st.lastStarted,
		LastExited:  st.lastExited,
		LastError:   st.lastError,
		Restarts:    st.restarts,
	}, nil
}

// markHealthy is a test seam that resets the restart counter for the
// named service. Production code never calls this directly: the
// supervisor itself resets the counter after a service has run cleanly
// for >= healthyResetThreshold.
func (s *Supervisor) markHealthy(name string) {
	s.mu.Lock()
	st, ok := s.health[name]
	s.mu.Unlock()
	if !ok {
		return
	}
	st.mu.Lock()
	st.restarts = 0
	st.mu.Unlock()
}

// Run starts every registered service plus the optional MemoryPressureProbe
// concurrently and blocks until ctx is cancelled. Each service runs
// inside its own goroutine with panic recovery; on a non-nil error
// (other than context.Canceled / context.DeadlineExceeded) the
// supervisor sleeps for computeBackoff(restarts) and restarts the
// service. The error returned by Run is always ctx.Err().
func (s *Supervisor) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.running = true
	svcs := make([]Service, len(s.services))
	copy(svcs, s.services)
	probe := s.probe
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	g, gctx := errgroup.WithContext(ctx)

	if probe != nil {
		g.Go(func() error {
			defer recoverPanic(s.log, "memory-probe")
			if err := probe.Run(gctx); err != nil && gctx.Err() == nil {
				s.log.Warn("supervisor: probe exited", "error", err)
			}
			return nil // never propagate; errgroup cancels siblings
		})
	}

	for _, svc := range svcs {
		svc := svc
		g.Go(func() error {
			s.superviseOne(gctx, svc)
			return nil
		})
	}

	_ = g.Wait()
	return ctx.Err()
}

// superviseOne runs a single service in a restart loop. Each iteration:
//
//  1. Notes the start time and Started=true on the service state.
//  2. Calls svc.Run inside a panic-recovering closure.
//  3. On clean ctx-cancel exit, returns.
//  4. On error, increments Restarts and sleeps computeBackoff(restarts).
//
// If the service runs healthy (no error returned) for at least
// healthyResetThreshold the restart counter resets so the next
// failure starts back at 1s.
func (s *Supervisor) superviseOne(ctx context.Context, svc Service) {
	name := svc.Name()
	st := s.stateFor(name)

	for {
		if ctx.Err() != nil {
			return
		}

		st.markStarted(s.clock.Now())

		err := s.invokeOnce(ctx, svc)

		exitTime := s.clock.Now()
		st.markExited(exitTime, err)

		if ctx.Err() != nil {
			return
		}

		// healthy reset: if the service stayed up for at least the
		// threshold, drop the restart counter back to 0.
		runFor := exitTime.Sub(st.startedAt())
		if runFor >= healthyResetThreshold {
			st.resetRestarts()
		}

		restarts := st.incrementRestarts()
		backoff := computeBackoff(restarts - 1) // first restart uses computeBackoff(0)=1s

		s.log.Info("supervisor: scheduling restart",
			"service", name,
			"restarts", restarts,
			"backoff", backoff.String(),
			"error", errString(err),
		)

		select {
		case <-ctx.Done():
			return
		case <-s.clock.After(backoff):
		}
	}
}

func (s *Supervisor) invokeOnce(ctx context.Context, svc Service) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
			s.log.Error("supervisor: service panicked",
				"service", svc.Name(),
				"panic", fmt.Sprintf("%v", r),
			)
		}
	}()
	return svc.Run(ctx)
}

func (s *Supervisor) stateFor(name string) *serviceState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.health[name]
}

func recoverPanic(log *slog.Logger, name string) {
	if r := recover(); r != nil {
		log.Error("supervisor: subsystem panicked",
			"subsystem", name,
			"panic", fmt.Sprintf("%v", r),
		)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (st *serviceState) markStarted(t time.Time) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.started = true
	st.lastStarted = t
}

func (st *serviceState) markExited(t time.Time, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.lastExited = t
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		st.lastError = err
	}
}

func (st *serviceState) startedAt() time.Time {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.lastStarted
}

func (st *serviceState) resetRestarts() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.restarts = 0
}

func (st *serviceState) incrementRestarts() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.restarts++
	return st.restarts
}
