package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Service is the interface that long-running daemon components implement.
// Run blocks until ctx is cancelled or the service exits. The supervisor
// recovers panics and logs service exits without tearing down siblings.
type Service interface {
	Name() string
	Run(ctx context.Context) error
}

// Supervisor manages a set of services, running them concurrently with
// independent panic recovery. When the parent context is cancelled, all
// services are signalled to stop.
type Supervisor struct {
	mu       sync.Mutex
	services []Service
	names    map[string]struct{}
}

func New() *Supervisor {
	return &Supervisor{
		names: make(map[string]struct{}),
	}
}

func (s *Supervisor) Register(svc Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := svc.Name()
	if _, dup := s.names[name]; dup {
		return fmt.Errorf("supervisor: service %q already registered", name)
	}
	s.names[name] = struct{}{}
	s.services = append(s.services, svc)
	return nil
}

// Names returns the names of all registered services.
func (s *Supervisor) Names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, 0, len(s.services))
	for _, svc := range s.services {
		out = append(out, svc.Name())
	}
	return out
}

// Run starts all registered services concurrently and blocks until the
// context is cancelled. Each service runs in its own goroutine with
// panic recovery. Service failures are logged but do not stop siblings.
func (s *Supervisor) Run(ctx context.Context) error {
	s.mu.Lock()
	svcs := make([]Service, len(s.services))
	copy(svcs, s.services)
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, svc := range svcs {
		wg.Add(1)
		go func(svc Service) {
			defer wg.Done()
			s.runService(ctx, svc)
		}(svc)
	}

	wg.Wait()
	return ctx.Err()
}

func (s *Supervisor) runService(ctx context.Context, svc Service) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("supervisor: service panicked",
				"service", svc.Name(),
				"panic", fmt.Sprintf("%v", r),
			)
		}
	}()

	if err := svc.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("supervisor: service exited with error",
			"service", svc.Name(),
			"error", err,
		)
	}
}
