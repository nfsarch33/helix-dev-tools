package outcomes

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// recorderEmitter captures every call for assertions.
type recorderEmitter struct {
	mu       sync.Mutex
	received []Outcome
	failNext error
}

func (r *recorderEmitter) Emit(_ context.Context, o Outcome) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNext != nil {
		err := r.failNext
		r.failNext = nil
		return err
	}
	r.received = append(r.received, o)
	return nil
}

func (r *recorderEmitter) Snapshot() []Outcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]Outcome, len(r.received))
	copy(cp, r.received)
	return cp
}

func newValidOutcome() Outcome {
	return Outcome{
		Timestamp: time.Date(2026, 4, 24, 22, 30, 0, 0, time.UTC),
		Kind:      KindAgentOutcome,
		Actor:     ActorCursorHook,
		Machine:   "macbook",
		Event:     "guard-shell:allow",
	}
}

func TestNoopEmitter(t *testing.T) {
	e := NoopEmitter{}
	if err := e.Emit(context.Background(), newValidOutcome()); err != nil {
		t.Fatalf("noop should not error: %v", err)
	}
}

func TestMultiEmitter_FanOut(t *testing.T) {
	r1 := &recorderEmitter{}
	r2 := &recorderEmitter{}
	m := NewMultiEmitter(r1, r2)

	o := newValidOutcome()
	if err := m.Emit(context.Background(), o); err != nil {
		t.Fatalf("multi emit: %v", err)
	}

	if len(r1.Snapshot()) != 1 {
		t.Errorf("r1 missed event")
	}
	if len(r2.Snapshot()) != 1 {
		t.Errorf("r2 missed event")
	}
}

func TestMultiEmitter_PartialFailure(t *testing.T) {
	failErr := errors.New("network down")
	r1 := &recorderEmitter{failNext: failErr}
	r2 := &recorderEmitter{}
	m := NewMultiEmitter(r1, r2)

	o := newValidOutcome()
	err := m.Emit(context.Background(), o)
	if err == nil {
		t.Fatalf("expected aggregated error, got nil")
	}
	if !errors.Is(err, failErr) {
		t.Errorf("expected %v wrapped, got %v", failErr, err)
	}
	if len(r2.Snapshot()) != 1 {
		t.Errorf("r2 should still receive even when r1 fails")
	}
}

func TestMultiEmitter_Empty(t *testing.T) {
	m := NewMultiEmitter()
	if err := m.Emit(context.Background(), newValidOutcome()); err != nil {
		t.Errorf("empty multi emit should succeed: %v", err)
	}
}

func TestMultiEmitter_NilGuard(t *testing.T) {
	r1 := &recorderEmitter{}
	m := NewMultiEmitter(nil, r1, nil)
	if err := m.Emit(context.Background(), newValidOutcome()); err != nil {
		t.Fatalf("nil sub-emitters should be skipped: %v", err)
	}
	if len(r1.Snapshot()) != 1 {
		t.Errorf("r1 missed event")
	}
}

func TestMultiEmitter_ValidatesBeforeFanOut(t *testing.T) {
	r := &recorderEmitter{}
	m := NewMultiEmitter(r)

	bad := Outcome{Kind: KindAgentOutcome}
	if err := m.Emit(context.Background(), bad); err == nil {
		t.Errorf("expected validation error")
	}
	if len(r.Snapshot()) != 0 {
		t.Errorf("invalid outcome should NOT reach sub-emitters, got %d", len(r.Snapshot()))
	}
}

func TestEmitFunc(t *testing.T) {
	calls := 0
	e := EmitFunc(func(ctx context.Context, o Outcome) error {
		calls++
		return nil
	})
	_ = e.Emit(context.Background(), newValidOutcome())
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestLocalMachineLabel(t *testing.T) {
	m := LocalMachineLabel()
	if m == "" {
		t.Errorf("LocalMachineLabel returned empty string")
	}
}
