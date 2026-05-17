package outcomes

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/coordination"
)

// DefaultEmitter returns the process-wide singleton emitter. Hooks call this
// in their handler init so the same instance is reused for the lifetime of the
// process.
//
// Behaviour:
//   - When CURSOR_TOOLS_OUTCOMES_DISABLED=1 -> NoopEmitter (kill switch).
//   - Always wrap the chosen base emitter in a RateLimitedEmitter to protect
//     downstream sinks (Mem0 in particular).
//   - Sink selection:
//   - CURSOR_TOOLS_OUTCOMES_SINK=memory  -> MemoryEmitter (Mem0 direct;
//     resolves credentials from env or MCP config).
//   - CURSOR_TOOLS_OUTCOMES_SINK=multi   -> Buffered + Memory (both fan-out).
//   - default                            -> Buffered NDJSON (fast path).
//
// The function is safe to call concurrently.
func DefaultEmitter(mcpConfigPath string) Emitter {
	defaultOnce.Do(func() {
		defaultEmitter = buildDefaultEmitter(mcpConfigPath)
	})
	return defaultEmitter
}

var (
	defaultOnce    sync.Once
	defaultEmitter Emitter
)

// ResetDefaultEmitter is exposed for tests; not used in production.
func ResetDefaultEmitter() {
	defaultOnce = sync.Once{}
	defaultEmitter = nil
}

func buildDefaultEmitter(mcpConfigPath string) Emitter {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CURSOR_TOOLS_OUTCOMES_DISABLED")), "1") {
		return NoopEmitter{}
	}

	sink := strings.ToLower(strings.TrimSpace(os.Getenv("CURSOR_TOOLS_OUTCOMES_SINK")))
	if sink == "" {
		sink = "buffered"
	}

	var base Emitter
	switch sink {
	case "memory":
		if mem := buildMemoryEmitter(mcpConfigPath); mem != nil {
			base = mem
		}
	case "multi":
		bf, _ := buildBufferedEmitter()
		mem := buildMemoryEmitter(mcpConfigPath)
		switch {
		case bf != nil && mem != nil:
			base = NewMultiEmitter(bf, mem)
		case bf != nil:
			base = bf
		case mem != nil:
			base = mem
		}
	default:
		bf, _ := buildBufferedEmitter()
		if bf != nil {
			base = bf
		}
	}

	if base == nil {
		base = NoopEmitter{}
	}

	return NewRateLimitedEmitter(base, RateLimitConfig{
		PerEventWindow: rateLimitWindowFromEnv(),
		AlwaysAllowEvents: []string{
			"guard-shell:deny",
			"guard-mcp:deny",
			"sanitize-read:deny",
			"post-edit:reject",
		},
	})
}

func buildBufferedEmitter() (*BufferedEmitter, error) {
	cfg := BufferedConfig{
		Path: strings.TrimSpace(os.Getenv("CURSOR_TOOLS_OUTCOMES_PATH")),
	}
	be, err := NewBufferedEmitter(cfg)
	if err != nil {
		return nil, err
	}
	return be, nil
}

func buildMemoryEmitter(mcpConfigPath string) *MemoryEmitter {
	apiKey, userID, err := coordination.ResolveCredentials(mcpConfigPath)
	if err != nil {
		return nil
	}
	if apiKey == "" || userID == "" {
		return nil
	}
	cfg := MemoryEmitterConfig{
		APIKey: apiKey,
		UserID: outcomeUserID(userID),
		AppID:  AppIDFleetOutcomes,
	}
	return NewMemoryEmitter(cfg)
}

// outcomeUserID lets us namespace outcomes (default user_id=fleet-evoloop)
// while keeping Mem0 coordination signals on the user's primary id.
func outcomeUserID(fallback string) string {
	if v := strings.TrimSpace(os.Getenv("CURSOR_TOOLS_OUTCOMES_USER")); v != "" {
		return v
	}
	return defaultOutcomeUserID(fallback)
}

func defaultOutcomeUserID(fallback string) string {
	return "fleet-evoloop"
}

func rateLimitWindowFromEnv() time.Duration {
	v := strings.TrimSpace(os.Getenv("CURSOR_TOOLS_OUTCOMES_RATE_S"))
	if v == "" {
		return DefaultPerEventWindow
	}
	d, err := time.ParseDuration(v + "s")
	if err != nil || d < 0 {
		return DefaultPerEventWindow
	}
	return d
}

// EmitSafe wraps a single Emit call with a short timeout and silently drops
// errors. Hooks must NEVER block or fail the user because of an outcome
// publish.
func EmitSafe(emitter Emitter, o Outcome) {
	if emitter == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = emitter.Emit(ctx, o)
}

// EmitSafeWithLog is like EmitSafe but returns the error for callers who want
// to log/instrument it (e.g. CLI subcommands).
func EmitSafeWithLog(emitter Emitter, o Outcome) error {
	if emitter == nil {
		return fmt.Errorf("nil emitter")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return emitter.Emit(ctx, o)
}
