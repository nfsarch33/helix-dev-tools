package outcomes

import (
	"context"
	"testing"
	"time"
)

func TestRateLimitedEmitter_DropsHotEvents(t *testing.T) {
	rec := &recorderEmitter{}
	rl := NewRateLimitedEmitter(rec, RateLimitConfig{
		PerEventWindow: 100 * time.Millisecond,
	})

	o := newValidOutcome()
	for i := 0; i < 10; i++ {
		_ = rl.Emit(context.Background(), o)
	}
	if got := len(rec.Snapshot()); got != 1 {
		t.Errorf("expected 1 emit (rest deduped), got %d", got)
	}
}

func TestRateLimitedEmitter_AllowsAfterWindow(t *testing.T) {
	rec := &recorderEmitter{}
	rl := NewRateLimitedEmitter(rec, RateLimitConfig{
		PerEventWindow: 10 * time.Millisecond,
	})

	o := newValidOutcome()
	_ = rl.Emit(context.Background(), o)
	time.Sleep(20 * time.Millisecond)
	_ = rl.Emit(context.Background(), o)
	if got := len(rec.Snapshot()); got != 2 {
		t.Errorf("expected 2 emits across windows, got %d", got)
	}
}

func TestRateLimitedEmitter_ImportantEventsBypass(t *testing.T) {
	rec := &recorderEmitter{}
	rl := NewRateLimitedEmitter(rec, RateLimitConfig{
		PerEventWindow:    1 * time.Hour,
		AlwaysAllowEvents: []string{"guard-shell:deny", "post-edit:reject"},
	})

	deny := newValidOutcome()
	deny.Event = "guard-shell:deny"
	for i := 0; i < 5; i++ {
		_ = rl.Emit(context.Background(), deny)
	}
	if got := len(rec.Snapshot()); got != 5 {
		t.Errorf("important events must bypass: got %d", got)
	}
}

func TestRateLimitedEmitter_PerActorIsolation(t *testing.T) {
	rec := &recorderEmitter{}
	rl := NewRateLimitedEmitter(rec, RateLimitConfig{
		PerEventWindow: 1 * time.Hour,
	})

	o1 := newValidOutcome()
	o1.Actor = ActorCursorHook
	o2 := newValidOutcome()
	o2.Actor = ActorFleetCLI

	_ = rl.Emit(context.Background(), o1)
	_ = rl.Emit(context.Background(), o2)
	if got := len(rec.Snapshot()); got != 2 {
		t.Errorf("different actors must NOT share quota: got %d", got)
	}
}

func TestRateLimitedEmitter_PerMachineIsolation(t *testing.T) {
	rec := &recorderEmitter{}
	rl := NewRateLimitedEmitter(rec, RateLimitConfig{
		PerEventWindow: 1 * time.Hour,
	})

	o1 := newValidOutcome()
	o1.Machine = "macbook"
	o2 := newValidOutcome()
	o2.Machine = "wsl1"
	_ = rl.Emit(context.Background(), o1)
	_ = rl.Emit(context.Background(), o2)
	if got := len(rec.Snapshot()); got != 2 {
		t.Errorf("different machines must NOT share quota: got %d", got)
	}
}
