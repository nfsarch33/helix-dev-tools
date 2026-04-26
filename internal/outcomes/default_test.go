package outcomes

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// fakeEmitter records Emit calls; lets us assert against EmitSafe / EmitSafeWithLog
// without relying on a real sink.
type fakeEmitter struct {
	emits   []Outcome
	emitErr error
}

func (f *fakeEmitter) Emit(_ context.Context, o Outcome) error {
	f.emits = append(f.emits, o)
	return f.emitErr
}

func sampleOutcome() Outcome {
	return Outcome{
		Kind:      KindAgentOutcome,
		Actor:     ActorCursorTools,
		Machine:   "default-test-host",
		Event:     "test:event",
		Timestamp: time.Now().UTC(),
		Meta:      map[string]string{"k": "v"},
	}
}

func TestDefaultEmitter_SingletonAcrossCalls(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_DISABLED", "1")
	ResetDefaultEmitter()
	t.Cleanup(ResetDefaultEmitter)

	first := DefaultEmitter("")
	second := DefaultEmitter("")

	if first == nil {
		t.Fatal("DefaultEmitter returned nil")
	}
	if first != second {
		t.Errorf("DefaultEmitter is not a singleton across calls (%p vs %p)", first, second)
	}
}

func TestBuildDefaultEmitter_KillSwitch(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_DISABLED", "1")
	em := buildDefaultEmitter("")
	if _, ok := em.(NoopEmitter); !ok {
		t.Errorf("expected NoopEmitter when kill switch set; got %T", em)
	}
}

func TestBuildDefaultEmitter_BufferedDefault(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_DISABLED", "")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_SINK", "")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_PATH", filepath.Join(t.TempDir(), "outcomes.ndjson"))

	em := buildDefaultEmitter("")
	rl, ok := em.(*RateLimitedEmitter)
	if !ok {
		t.Fatalf("expected *RateLimitedEmitter wrapping the buffered sink; got %T", em)
	}
	if rl == nil {
		t.Fatal("rate-limited emitter is nil")
	}
}

func TestBuildDefaultEmitter_MemorySinkWithoutCreds(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_DISABLED", "")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_SINK", "memory")
	t.Setenv("MEM0_API_KEY", "")
	t.Setenv("MEM0_USER_ID", "")

	em := buildDefaultEmitter(filepath.Join(t.TempDir(), "no-such-config.json"))
	if em == nil {
		t.Fatal("buildDefaultEmitter returned nil")
	}
	// Without creds the memory sink resolution should fall through to NoopEmitter
	// wrapped in RateLimitedEmitter.
	if _, ok := em.(*RateLimitedEmitter); !ok {
		t.Errorf("expected RateLimitedEmitter wrapper; got %T", em)
	}
}

func TestBuildDefaultEmitter_MultiSinkFallback(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_DISABLED", "")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_SINK", "multi")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_PATH", filepath.Join(t.TempDir(), "outcomes-multi.ndjson"))
	t.Setenv("MEM0_API_KEY", "")
	t.Setenv("MEM0_USER_ID", "")

	em := buildDefaultEmitter("")
	if em == nil {
		t.Fatal("buildDefaultEmitter returned nil for multi sink")
	}
}

func TestBuildBufferedEmitter_Defaults(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_PATH", filepath.Join(t.TempDir(), "buffered.ndjson"))
	bf, err := buildBufferedEmitter()
	if err != nil {
		t.Fatalf("buildBufferedEmitter: %v", err)
	}
	if bf == nil {
		t.Fatal("buildBufferedEmitter returned nil")
	}
	if bf.Path() == "" {
		t.Error("expected non-empty Path()")
	}
}

func TestOutcomeUserID_HonoursOverride(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_USER", "custom-user")
	if got := outcomeUserID("ignored"); got != "custom-user" {
		t.Errorf("outcomeUserID = %q; want custom-user", got)
	}
}

func TestOutcomeUserID_FallsBackToDefault(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_USER", "")
	if got := outcomeUserID("fallback-user"); got == "" {
		t.Error("outcomeUserID returned empty string")
	}
}

func TestDefaultOutcomeUserID(t *testing.T) {
	if got := defaultOutcomeUserID("anything"); got != "fleet-evoloop" {
		t.Errorf("defaultOutcomeUserID = %q; want fleet-evoloop", got)
	}
}

func TestRateLimitWindowFromEnv_Default(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_RATE_S", "")
	if got := rateLimitWindowFromEnv(); got != DefaultPerEventWindow {
		t.Errorf("rateLimitWindowFromEnv = %v; want %v", got, DefaultPerEventWindow)
	}
}

func TestRateLimitWindowFromEnv_Override(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_RATE_S", "30")
	if got := rateLimitWindowFromEnv(); got != 30*time.Second {
		t.Errorf("rateLimitWindowFromEnv = %v; want 30s", got)
	}
}

func TestRateLimitWindowFromEnv_InvalidFallsBack(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_OUTCOMES_RATE_S", "not-a-number")
	if got := rateLimitWindowFromEnv(); got != DefaultPerEventWindow {
		t.Errorf("rateLimitWindowFromEnv = %v; want default", got)
	}
}

func TestEmitSafe_NilEmitterIsNoop(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("EmitSafe panicked on nil emitter: %v", r)
		}
	}()
	EmitSafe(nil, sampleOutcome())
}

func TestEmitSafe_DropsErrors(t *testing.T) {
	em := &fakeEmitter{emitErr: errors.New("boom")}
	EmitSafe(em, sampleOutcome())
	if len(em.emits) != 1 {
		t.Errorf("expected 1 emit attempt; got %d", len(em.emits))
	}
}

func TestEmitSafeWithLog_NilEmitterReturnsError(t *testing.T) {
	if err := EmitSafeWithLog(nil, sampleOutcome()); err == nil {
		t.Error("EmitSafeWithLog(nil, ...) returned nil error; want error")
	}
}

func TestEmitSafeWithLog_PropagatesError(t *testing.T) {
	want := errors.New("upstream sink failed")
	em := &fakeEmitter{emitErr: want}
	if got := EmitSafeWithLog(em, sampleOutcome()); !errors.Is(got, want) {
		t.Errorf("EmitSafeWithLog error = %v; want %v", got, want)
	}
}

func TestEmitSafeWithLog_HappyPath(t *testing.T) {
	em := &fakeEmitter{}
	if err := EmitSafeWithLog(em, sampleOutcome()); err != nil {
		t.Errorf("EmitSafeWithLog returned unexpected error: %v", err)
	}
	if len(em.emits) != 1 {
		t.Errorf("expected 1 emit; got %d", len(em.emits))
	}
}
