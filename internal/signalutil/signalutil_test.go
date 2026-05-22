package signalutil

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestContext_DefaultSignalsApplied(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, stop := Context(parent)
	defer stop()
	select {
	case <-ctx.Done():
		t.Fatal("context cancelled before any signal/parent cancel")
	default:
	}
}

func TestContext_PropagatesParentCancel(t *testing.T) {
	t.Parallel()
	parent, cancel := context.WithCancel(context.Background())
	ctx, stop := Context(parent)
	defer stop()

	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("child context did not see parent cancel within 1s")
	}
}

func TestRun_ReturnsFnError(t *testing.T) {
	t.Parallel()
	want := errors.New("boom")
	got := Run(context.Background(), 100*time.Millisecond, func(ctx context.Context) error {
		return want
	})
	if !errors.Is(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRun_HonoursSignalAndGracefulExit(t *testing.T) {
	t.Parallel()
	var observed atomic.Bool
	errCh := make(chan error, 1)

	go func() {
		errCh <- Run(context.Background(), 500*time.Millisecond, func(ctx context.Context) error {
			<-ctx.Done()
			observed.Store(true)
			return nil
		})
	}()

	// Send SIGTERM to the test process; signal.NotifyContext catches it.
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil graceful return, got %v", err)
		}
		if !observed.Load() {
			t.Fatal("fn never observed ctx.Done")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of SIGTERM")
	}
}

func TestRun_DeadlineExceededOnSlowShutdown(t *testing.T) {
	t.Parallel()
	errCh := make(chan error, 1)

	go func() {
		errCh <- Run(context.Background(), 50*time.Millisecond, func(ctx context.Context) error {
			<-ctx.Done()
			// Simulate a fn that ignores ctx and never returns.
			time.Sleep(2 * time.Second)
			return nil
		})
	}()

	time.Sleep(20 * time.Millisecond)
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case err := <-errCh:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected DeadlineExceeded, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not enforce grace deadline")
	}
}
