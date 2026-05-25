package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EvoSpineEvent is a structured event consumable by the ORHEP cycle.
type EvoSpineEvent struct {
	Type      string      `json:"type"`
	Source    string      `json:"source"`
	Timestamp string      `json:"ts"`
	SprintID  string      `json:"sprint_id,omitempty"`
	Verdict   string      `json:"verdict"`
	PassRate  float64     `json:"pass_rate"`
	FailCount int         `json:"fail_count"`
	Payload   interface{} `json:"payload,omitempty"`
}

// EmitEvoSpineEvent writes a structured eval event to the agentrace NDJSON log
// so the ORHEP cycle can consume it for Observe and Evolve stages.
func EmitEvoSpineEvent(verdict GateVerdict, sprintID string) (EvoSpineEvent, error) {
	verdictStr := "pass"
	if !verdict.Pass {
		verdictStr = "fail"
	}
	event := EvoSpineEvent{
		Type:      "eval_gate_result",
		Source:    "evalharness",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		SprintID:  sprintID,
		Verdict:   verdictStr,
		PassRate:  verdict.PassRate,
		FailCount: verdict.FailCount,
		Payload:   verdict,
	}

	logPath := defaultAgentraceLogPath()
	if err := appendNDJSON(logPath, event); err != nil {
		return event, fmt.Errorf("emit evospine event: %w", err)
	}
	return event, nil
}

// EmitSprintReport writes a sprint report event for ORHEP consumption.
func EmitSprintReport(report SprintReport) (EvoSpineEvent, error) {
	verdictStr := "pass"
	if !report.Verdict.Pass {
		verdictStr = "fail"
	}
	event := EvoSpineEvent{
		Type:      "eval_sprint_report",
		Source:    "evalharness",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		SprintID:  report.SprintID,
		Verdict:   verdictStr,
		PassRate:  report.Verdict.PassRate,
		FailCount: report.Verdict.FailCount,
		Payload:   report,
	}

	logPath := defaultAgentraceLogPath()
	if err := appendNDJSON(logPath, event); err != nil {
		return event, fmt.Errorf("emit sprint report event: %w", err)
	}
	return event, nil
}

func defaultAgentraceLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")
}

func appendNDJSON(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}
