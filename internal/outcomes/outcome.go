// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop client filters Mem0 capsules by the canonical evoloop-daemon source label and wsl1 producer-machine name

// Package outcomes implements the agent_outcome capsule format used by every
// Cursor hook, fleet worker, and Helixon daemon to feed the EvoLoop daemon.
//
// Sprint v253 day 3: introduce the unified Outcome schema and Emitter
// interfaces. Outcomes land in Mem0 under app_id=cursor-global-kb with
// kind=agent_outcome (see internal/evoloop/kinds.go for the canonical Kind
// enum).
package outcomes

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Kind is the canonical metadata kind for agent outcomes.
const KindAgentOutcome = "agent_outcome"

// Actor enumerates the producers allowed to emit outcomes.
const (
	ActorCursorHook     = "cursor-hook"
	ActorCursorTools    = "cursor-tools"
	ActorFleetCLI       = "fleet-cli"
	ActorMCBridge       = "mc-bridge"
	ActorFallbackBridge = "fallback-bridge"
	ActorIronclawDaemon = "helixon-daemon"
	ActorEvoloopDaemon  = "evoloop-daemon"
)

// MaxDetailChars caps detail strings to keep Mem0 capsules small.
const MaxDetailChars = 240

// Validation errors.
var (
	ErrMissingKind    = errors.New("outcomes: kind is required")
	ErrInvalidKind    = errors.New("outcomes: kind must equal " + KindAgentOutcome)
	ErrMissingActor   = errors.New("outcomes: actor is required")
	ErrInvalidActor   = errors.New("outcomes: actor not recognised")
	ErrMissingMachine = errors.New("outcomes: machine is required")
	ErrMissingEvent   = errors.New("outcomes: event is required")
)

// Outcome is the canonical capsule emitted by every Helixon/Cursor worker.
//
// JSON keys are stable: changes here require a golden-file update and an ADR.
type Outcome struct {
	Timestamp  time.Time         `json:"ts"`
	Kind       string            `json:"kind"`
	Actor      string            `json:"actor"`
	Machine    string            `json:"machine"`
	Event      string            `json:"event"`
	McpTool    string            `json:"mcp_tool,omitempty"`
	LatencyMs  int64             `json:"latency_ms,omitempty"`
	SkillHit   *bool             `json:"skill_hit,omitempty"`
	KPIDelta   float64           `json:"kpi_delta,omitempty"`
	SessionID  string            `json:"session_id,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
	Sprint     string            `json:"sprint,omitempty"`
	ExitCode   *int              `json:"exit_code,omitempty"`
	BytesIn    int64             `json:"bytes_in,omitempty"`
	BytesOut   int64             `json:"bytes_out,omitempty"`
	DurationMs int64             `json:"duration_ms,omitempty"`
}

// KnownActors returns the canonical list in declaration order.
func KnownActors() []string {
	return []string{
		ActorCursorHook,
		ActorCursorTools,
		ActorFleetCLI,
		ActorMCBridge,
		ActorFallbackBridge,
		ActorIronclawDaemon,
		ActorEvoloopDaemon,
	}
}

// IsKnownActor reports whether the actor is one of the recognised producers.
func IsKnownActor(a string) bool {
	for _, k := range KnownActors() {
		if a == k {
			return true
		}
	}
	return false
}

// Validate checks required fields for the unified namespace.
func (o Outcome) Validate() error {
	if strings.TrimSpace(o.Kind) == "" {
		return ErrMissingKind
	}
	if o.Kind != KindAgentOutcome {
		return fmt.Errorf("%w: got %q", ErrInvalidKind, o.Kind)
	}
	if strings.TrimSpace(o.Actor) == "" {
		return ErrMissingActor
	}
	if !IsKnownActor(o.Actor) {
		return fmt.Errorf("%w: got %q", ErrInvalidActor, o.Actor)
	}
	if strings.TrimSpace(o.Machine) == "" {
		return ErrMissingMachine
	}
	if strings.TrimSpace(o.Event) == "" {
		return ErrMissingEvent
	}
	return nil
}

// Normalize fills defaults and truncates oversized fields. Idempotent.
func (o *Outcome) Normalize() {
	if o.Kind == "" {
		o.Kind = KindAgentOutcome
	}
	if o.Timestamp.IsZero() {
		o.Timestamp = time.Now().UTC()
	} else {
		o.Timestamp = o.Timestamp.UTC()
	}
	if len(o.Detail) > MaxDetailChars {
		o.Detail = o.Detail[:MaxDetailChars]
	}
}

// MarshalJSON forces UTC timestamps and sorted Meta keys for stable golden
// output.
func (o Outcome) MarshalJSON() ([]byte, error) {
	type alias Outcome
	c := alias(o)
	if !c.Timestamp.IsZero() {
		c.Timestamp = c.Timestamp.UTC()
	}
	if len(c.Meta) > 0 {
		sorted := make(map[string]string, len(c.Meta))
		keys := make([]string, 0, len(c.Meta))
		for k := range c.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = c.Meta[k]
		}
		c.Meta = sorted
	}
	return json.Marshal(c)
}

// Mem0Text returns the human-readable summary stored in Mem0.
func (o Outcome) Mem0Text() string {
	parts := []string{
		fmt.Sprintf("[%s] %s on %s:", o.Actor, o.Event, o.Machine),
	}
	if o.Detail != "" {
		parts = append(parts, o.Detail)
	}
	if o.LatencyMs > 0 {
		parts = append(parts, fmt.Sprintf("(%dms)", o.LatencyMs))
	}
	return strings.Join(parts, " ")
}

// Mem0Metadata maps the Outcome to a string-keyed metadata bag suitable for
// Mem0 v1/v2 APIs (which require all values to be strings).
func (o Outcome) Mem0Metadata() map[string]string {
	m := map[string]string{
		"kind":    KindAgentOutcome,
		"actor":   o.Actor,
		"machine": o.Machine,
		"event":   o.Event,
	}
	if o.McpTool != "" {
		m["mcp_tool"] = o.McpTool
	}
	if o.LatencyMs > 0 {
		m["latency_ms"] = strconv.FormatInt(o.LatencyMs, 10)
	}
	if o.SkillHit != nil {
		m["skill_hit"] = strconv.FormatBool(*o.SkillHit)
	}
	if o.KPIDelta != 0 {
		m["kpi_delta"] = strconv.FormatFloat(o.KPIDelta, 'f', -1, 64)
	}
	if o.SessionID != "" {
		m["session_id"] = o.SessionID
	}
	if o.Sprint != "" {
		m["sprint"] = o.Sprint
	}
	if o.ExitCode != nil {
		m["exit_code"] = strconv.Itoa(*o.ExitCode)
	}
	if o.BytesIn > 0 {
		m["bytes_in"] = strconv.FormatInt(o.BytesIn, 10)
	}
	if o.BytesOut > 0 {
		m["bytes_out"] = strconv.FormatInt(o.BytesOut, 10)
	}
	if o.DurationMs > 0 {
		m["duration_ms"] = strconv.FormatInt(o.DurationMs, 10)
	}
	for k, v := range o.Meta {
		if _, exists := m[k]; exists {
			continue
		}
		if v == "" {
			continue
		}
		m[k] = v
	}
	return m
}
