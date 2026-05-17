package cli

import (
	"os"
	"strings"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/outcomes"
)

// hookOutcomeEmitter returns the singleton outcome emitter for hook handlers.
//
// Sprint v253 day 3: every hook handler also emits an `agent_outcome` capsule
// so the EvoLoop daemon (running on WSL1) can fan-in MacBook signals through
// the same Mem0 namespace (app_id=cursor-global-kb).
func hookOutcomeEmitter(p config.Paths) outcomes.Emitter {
	return outcomes.DefaultEmitter(p.CursorMCPConfig())
}

// recordHookOutcome builds an Outcome from hook-internal metrics and emits it
// non-blockingly. NEVER returns an error -- the user must never feel an outcome
// publish failure.
func recordHookOutcome(emitter outcomes.Emitter, params hookOutcomeParams) {
	if emitter == nil {
		return
	}
	o := outcomes.Outcome{
		Kind:      outcomes.KindAgentOutcome,
		Actor:     outcomes.ActorCursorHook,
		Machine:   outcomes.LocalMachineLabel(),
		Event:     params.hookName + ":" + params.action,
		LatencyMs: params.latencyMs,
		Detail:    truncateForOutcome(params.detail),
		Sprint:    sprintFromEnv(),
		Meta: map[string]string{
			"hook":     params.hookName,
			"category": params.category,
		},
		BytesIn:  params.bytesIn,
		McpTool:  params.mcpTool,
		SkillHit: params.skillHit,
	}
	if params.memoryLayer != "" {
		o.Meta["memory_layer"] = params.memoryLayer
	}
	if params.memoryOp != "" {
		o.Meta["memory_op"] = params.memoryOp
	}
	if params.memoryResult != "" {
		o.Meta["memory_result"] = params.memoryResult
	}
	for k, v := range params.extraMeta {
		if k == "" || v == "" {
			continue
		}
		if _, exists := o.Meta[k]; exists {
			continue
		}
		o.Meta[k] = v
	}
	outcomes.EmitSafe(emitter, o)
}

// hookOutcomeParams carries the call-site details forwarded into an Outcome.
type hookOutcomeParams struct {
	hookName     string
	action       string
	category     string
	latencyMs    int64
	detail       string
	bytesIn      int64
	mcpTool      string
	skillHit     *bool
	memoryLayer  string
	memoryOp     string
	memoryResult string
	extraMeta    map[string]string
}

func truncateForOutcome(s string) string {
	if len(s) > outcomes.MaxDetailChars {
		return s[:outcomes.MaxDetailChars]
	}
	return s
}

func sprintFromEnv() string {
	v := strings.TrimSpace(os.Getenv("CURSOR_TOOLS_SPRINT"))
	if v != "" {
		return v
	}
	return "v253"
}
