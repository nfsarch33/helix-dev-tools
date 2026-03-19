package coordination

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	AppID = "cursor-coordination"
)

// SignalType classifies the purpose of a coordination signal.
type SignalType string

const (
	SignalActiveState  SignalType = "active-state"
	SignalTaskDispatch SignalType = "task-dispatch"
	SignalDecision     SignalType = "decision"
	SignalBlocker      SignalType = "blocker"
	SignalCompleted    SignalType = "completed"
)

// ValidSignalTypes enumerates accepted signal types.
var ValidSignalTypes = []SignalType{
	SignalActiveState,
	SignalTaskDispatch,
	SignalDecision,
	SignalBlocker,
	SignalCompleted,
}

// IsValidSignalType checks whether a string is a recognised signal type.
func IsValidSignalType(s string) bool {
	for _, valid := range ValidSignalTypes {
		if string(valid) == s {
			return true
		}
	}
	return false
}

// Signal represents a coordination message between Cursor instances.
type Signal struct {
	ID        string            `json:"id,omitempty"`
	Type      SignalType        `json:"type"`
	Machine   string            `json:"machine"`
	TargetFor string            `json:"target_for,omitempty"`
	Message   string            `json:"message"`
	Priority  string            `json:"priority,omitempty"`
	Sprint    string            `json:"sprint,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Mem0Text returns the natural-language text stored in Mem0 for this signal.
func (s Signal) Mem0Text() string {
	var parts []string

	switch s.Type {
	case SignalTaskDispatch:
		if s.TargetFor != "" {
			parts = append(parts, fmt.Sprintf("Task for %s:", s.TargetFor))
		} else {
			parts = append(parts, "Task:")
		}
	case SignalBlocker:
		parts = append(parts, "Blocker:")
	case SignalDecision:
		parts = append(parts, "Decision:")
	case SignalCompleted:
		parts = append(parts, "Completed:")
	case SignalActiveState:
		parts = append(parts, fmt.Sprintf("%s working on:", s.Machine))
	default:
		parts = append(parts, fmt.Sprintf("[%s]", s.Type))
	}

	parts = append(parts, s.Message)
	if s.Priority != "" && s.Priority != "normal" {
		parts = append(parts, fmt.Sprintf("Priority: %s.", s.Priority))
	}
	return strings.Join(parts, " ")
}

// Mem0Metadata returns the metadata map for storing in Mem0.
func (s Signal) Mem0Metadata() map[string]string {
	m := map[string]string{
		"type":    string(s.Type),
		"machine": s.Machine,
	}
	if s.TargetFor != "" {
		m["target_for"] = s.TargetFor
	}
	if s.Priority != "" {
		m["priority"] = s.Priority
	}
	if s.Sprint != "" {
		m["sprint"] = s.Sprint
	}
	for k, v := range s.Metadata {
		m[k] = v
	}
	return m
}

// LocalMachine returns the current machine's platform label.
func LocalMachine() string {
	switch {
	case runtime.GOOS == "darwin":
		return "macos"
	case os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "":
		return "wsl"
	default:
		return runtime.GOOS
	}
}

// RenderHandoffSection produces markdown from a list of signals, grouped by type.
func RenderHandoffSection(signals []Signal) string {
	if len(signals) == 0 {
		return ""
	}

	grouped := map[SignalType][]Signal{}
	for _, s := range signals {
		grouped[s.Type] = append(grouped[s.Type], s)
	}

	sectionOrder := []struct {
		sType   SignalType
		heading string
	}{
		{SignalActiveState, "## Task Summary"},
		{SignalDecision, "## Decisions Made"},
		{SignalBlocker, "## Blockers"},
		{SignalTaskDispatch, "## Delegated Tasks"},
		{SignalCompleted, "## Completed Items"},
	}

	var sb strings.Builder
	for _, sec := range sectionOrder {
		items, ok := grouped[sec.sType]
		if !ok || len(items) == 0 {
			continue
		}
		sb.WriteString(sec.heading + "\n\n")
		for i, item := range items {
			sb.WriteString(fmt.Sprintf("%d. %s", i+1, item.Message))
			if item.TargetFor != "" {
				sb.WriteString(fmt.Sprintf(" *(for %s)*", item.TargetFor))
			}
			if item.Priority != "" && item.Priority != "normal" {
				sb.WriteString(fmt.Sprintf(" [priority: %s]", item.Priority))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
