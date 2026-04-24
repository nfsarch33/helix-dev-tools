package outcomes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Emitter publishes Outcomes to a backing store (Mem0, NDJSON, etc).
//
// Implementations MUST be safe for concurrent use.
type Emitter interface {
	Emit(ctx context.Context, o Outcome) error
}

// EmitFunc adapts a plain function to the Emitter interface.
type EmitFunc func(ctx context.Context, o Outcome) error

// Emit implements Emitter.
func (f EmitFunc) Emit(ctx context.Context, o Outcome) error {
	if f == nil {
		return nil
	}
	return f(ctx, o)
}

// NoopEmitter discards every Outcome. Used as the default when no real sink
// is wired (e.g. unit tests).
type NoopEmitter struct{}

// Emit implements Emitter.
func (NoopEmitter) Emit(_ context.Context, _ Outcome) error { return nil }

// MultiEmitter fans an Outcome out to several sub-emitters. Sub-emitter
// failures are aggregated; one failure does NOT short-circuit the others.
type MultiEmitter struct {
	subs []Emitter
}

// NewMultiEmitter constructs a fan-out emitter. Nil sub-emitters are
// silently skipped so callers can build the slice unconditionally.
func NewMultiEmitter(subs ...Emitter) *MultiEmitter {
	clean := make([]Emitter, 0, len(subs))
	for _, s := range subs {
		if s == nil {
			continue
		}
		clean = append(clean, s)
	}
	return &MultiEmitter{subs: clean}
}

// Emit normalises and validates the outcome once, then fans out.
func (m *MultiEmitter) Emit(ctx context.Context, o Outcome) error {
	o.Normalize()
	if err := o.Validate(); err != nil {
		return fmt.Errorf("multi emit: %w", err)
	}

	if len(m.subs) == 0 {
		return nil
	}

	var errs []error
	for i, s := range m.subs {
		if err := s.Emit(ctx, o); err != nil {
			errs = append(errs, fmt.Errorf("sub[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// LocalMachineLabel returns a short, stable hostname for outcomes.
//
// Lookup order:
//  1. $CURSOR_TOOLS_MACHINE
//  2. $FLEET_MACHINE
//  3. /etc/cursor-tools/machine (single line, trimmed)
//  4. os.Hostname()
//  5. runtime.GOOS fallback
func LocalMachineLabel() string {
	for _, env := range []string{"CURSOR_TOOLS_MACHINE", "FLEET_MACHINE"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	if data, err := os.ReadFile("/etc/cursor-tools/machine"); err == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			return s
		}
	}
	if h, err := os.Hostname(); err == nil {
		h = strings.TrimSpace(h)
		if h != "" {
			return shortHostname(h)
		}
	}
	return runtime.GOOS
}

func shortHostname(h string) string {
	h = strings.ToLower(h)
	if dot := strings.Index(h, "."); dot >= 0 {
		h = h[:dot]
	}
	if strings.HasSuffix(h, ".local") {
		h = strings.TrimSuffix(h, ".local")
	}
	return h
}
