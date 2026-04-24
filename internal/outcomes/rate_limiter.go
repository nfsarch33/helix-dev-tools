package outcomes

import (
	"context"
	"sync"
	"time"
)

// DefaultPerEventWindow is the gap-debounce window applied per
// (actor, machine, event) tuple before another duplicate is forwarded.
//
// Defaults are intentionally generous (10s) because most hooks will fire many
// near-identical events per second; we only need one anchor per window to feed
// EvoLoop reliably.
const DefaultPerEventWindow = 10 * time.Second

// RateLimitConfig controls per-event debouncing for outcomes.
type RateLimitConfig struct {
	// PerEventWindow is the dedup window per (actor, machine, event).
	// Zero -> DefaultPerEventWindow.
	PerEventWindow time.Duration
	// AlwaysAllowEvents bypasses the rate limit (e.g. denies, errors).
	AlwaysAllowEvents []string
}

// RateLimitedEmitter wraps another Emitter with per-event debouncing.
//
// This is the lightweight rubric gate that prevents hot code paths (post-edit
// loops, guard-shell scans) from saturating Mem0.
type RateLimitedEmitter struct {
	cfg      RateLimitConfig
	inner    Emitter
	mu       sync.Mutex
	lastSeen map[string]time.Time
	bypass   map[string]struct{}
}

// NewRateLimitedEmitter wraps inner with the provided rate limit config.
func NewRateLimitedEmitter(inner Emitter, cfg RateLimitConfig) *RateLimitedEmitter {
	if cfg.PerEventWindow <= 0 {
		cfg.PerEventWindow = DefaultPerEventWindow
	}
	bypass := make(map[string]struct{}, len(cfg.AlwaysAllowEvents))
	for _, e := range cfg.AlwaysAllowEvents {
		if e != "" {
			bypass[e] = struct{}{}
		}
	}
	return &RateLimitedEmitter{
		cfg:      cfg,
		inner:    inner,
		lastSeen: map[string]time.Time{},
		bypass:   bypass,
	}
}

// Emit forwards o iff the (actor|machine|event) tuple has not been seen within
// the configured window. Validation/normalization is delegated to the inner
// emitter.
func (r *RateLimitedEmitter) Emit(ctx context.Context, o Outcome) error {
	o.Normalize()

	if _, ok := r.bypass[o.Event]; ok {
		return r.inner.Emit(ctx, o)
	}

	key := o.Actor + "|" + o.Machine + "|" + o.Event
	now := time.Now()

	r.mu.Lock()
	last, seen := r.lastSeen[key]
	if seen && now.Sub(last) < r.cfg.PerEventWindow {
		r.mu.Unlock()
		return nil
	}
	r.lastSeen[key] = now
	r.mu.Unlock()

	return r.inner.Emit(ctx, o)
}
