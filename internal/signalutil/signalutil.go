// Package signalutil consolidates the SIGINT/SIGTERM graceful-shutdown
// pattern that every daemon main duplicates: signal.NotifyContext +
// defer cancel + run-loop + deadline-bounded shutdown.
//
// Surveyed in v8200 T-B12 audit; instances live in cursor-tools/cmd/mcp-proxy,
// ai-agent-business-stack/go/cmd/fleet-slo-watcher, and
// helixon-mcp/cmd/helixon-mcp. This package is the canonical home.
package signalutil

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// DefaultSignals is the standard interrupt set: Ctrl-C and SIGTERM.
var DefaultSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// Context returns a child context cancelled on any of the given signals,
// plus a stop function the caller must defer.
//
// If sigs is empty, DefaultSignals is used.
func Context(parent context.Context, sigs ...os.Signal) (context.Context, context.CancelFunc) {
	if len(sigs) == 0 {
		sigs = DefaultSignals
	}
	return signal.NotifyContext(parent, sigs...)
}

// Run executes fn with a signal-aware context and waits for it to return.
// On context cancellation (signal received), Run waits up to grace for fn
// to return cleanly; if fn does not return within grace, Run returns the
// last error fn produced (or context.DeadlineExceeded if fn never returned).
//
// fn must honour ctx.Done() and exit promptly; Run does not force-kill.
func Run(parent context.Context, grace time.Duration, fn func(ctx context.Context) error) error {
	ctx, stop := Context(parent)
	defer stop()

	done := make(chan error, 1)
	go func() { done <- fn(ctx) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Signal received; give fn a bounded chance to clean up.
		if grace <= 0 {
			grace = 5 * time.Second
		}
		t := time.NewTimer(grace)
		defer t.Stop()
		select {
		case err := <-done:
			return err
		case <-t.C:
			return context.DeadlineExceeded
		}
	}
}
