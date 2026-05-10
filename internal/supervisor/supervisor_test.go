package supervisor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeService struct {
	name    string
	started atomic.Bool
	stopped atomic.Bool
	runErr  error
	runFunc func(ctx context.Context) error
}

func (f *fakeService) Name() string { return f.name }

func (f *fakeService) Run(ctx context.Context) error {
	f.started.Store(true)
	if f.runFunc != nil {
		return f.runFunc(ctx)
	}
	if f.runErr != nil {
		return f.runErr
	}
	<-ctx.Done()
	f.stopped.Store(true)
	return ctx.Err()
}

func TestSupervisor_RegisterAndRun(t *testing.T) {
	s := New()
	svc := &fakeService{name: "test-svc"}
	if err := s.Register(svc); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := s.Run(ctx)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected run error: %v", err)
	}

	if !svc.started.Load() {
		t.Error("service was not started")
	}
}

func TestSupervisor_DuplicateRegister(t *testing.T) {
	s := New()
	svc := &fakeService{name: "dup"}
	if err := s.Register(svc); err != nil {
		t.Fatalf("first register should succeed: %v", err)
	}
	if err := s.Register(svc); err == nil {
		t.Error("duplicate register should return error")
	}
}

func TestSupervisor_PanicRecovery(t *testing.T) {
	s := New()
	panicked := &fakeService{
		name: "panicker",
		runFunc: func(ctx context.Context) error {
			panic("test panic")
		},
	}
	if err := s.Register(panicked); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := s.Run(ctx)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("supervisor should recover from panic, got: %v", err)
	}
}

func TestSupervisor_MultipleServices(t *testing.T) {
	s := New()
	svcA := &fakeService{name: "alpha"}
	svcB := &fakeService{name: "beta"}

	for _, svc := range []Service{svcA, svcB} {
		if err := s.Register(svc); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = s.Run(ctx)

	if !svcA.started.Load() {
		t.Error("alpha was not started")
	}
	if !svcB.started.Load() {
		t.Error("beta was not started")
	}
}

func TestSupervisor_ServiceError(t *testing.T) {
	s := New()
	svc := &fakeService{
		name:   "failing",
		runErr: errors.New("service crashed"),
	}
	if err := s.Register(svc); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := s.Run(ctx)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSupervisor_Names(t *testing.T) {
	s := New()
	s.Register(&fakeService{name: "a"})
	s.Register(&fakeService{name: "b"})

	names := s.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
}
