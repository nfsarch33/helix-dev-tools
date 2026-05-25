package evalharness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEmitEvoSpineEvent_Pass(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "agentrace.ndjson")

	verdict := GateVerdict{Pass: true, PassRate: 0.95, FailCount: 2, TotalCount: 40}
	event, err := emitEvoSpineEventTo(logPath, verdict, "v11200")
	if err != nil {
		t.Fatalf("emit error: %v", err)
	}

	if event.Type != "eval_gate_result" {
		t.Errorf("expected type eval_gate_result, got %s", event.Type)
	}
	if event.Verdict != "pass" {
		t.Errorf("expected verdict pass, got %s", event.Verdict)
	}
	if event.SprintID != "v11200" {
		t.Errorf("expected sprint v11200, got %s", event.SprintID)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var parsed EvoSpineEvent
	if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil {
		t.Fatalf("parse NDJSON: %v", err)
	}
	if parsed.Source != "evalharness" {
		t.Errorf("expected source evalharness, got %s", parsed.Source)
	}
}

func TestEmitEvoSpineEvent_Fail(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "agentrace.ndjson")

	verdict := GateVerdict{Pass: false, PassRate: 0.60, FailCount: 10, TotalCount: 25}
	event, err := emitEvoSpineEventTo(logPath, verdict, "v11200")
	if err != nil {
		t.Fatalf("emit error: %v", err)
	}
	if event.Verdict != "fail" {
		t.Errorf("expected verdict fail, got %s", event.Verdict)
	}
}

func TestEmitSprintReport_WritesNDJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "agentrace.ndjson")

	report := SprintReport{
		SprintID:   "v11200",
		EventCount: 5,
		Verdict:    GateVerdict{Pass: true, PassRate: 0.90, FailCount: 1, TotalCount: 10},
	}

	event, err := emitSprintReportTo(logPath, report)
	if err != nil {
		t.Fatalf("emit error: %v", err)
	}
	if event.Type != "eval_sprint_report" {
		t.Errorf("expected type eval_sprint_report, got %s", event.Type)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty log file")
	}
}

func emitEvoSpineEventTo(path string, verdict GateVerdict, sprintID string) (EvoSpineEvent, error) {
	verdictStr := "pass"
	if !verdict.Pass {
		verdictStr = "fail"
	}
	event := EvoSpineEvent{
		Type:      "eval_gate_result",
		Source:    "evalharness",
		Timestamp: "2026-05-25T00:00:00Z",
		SprintID:  sprintID,
		Verdict:   verdictStr,
		PassRate:  verdict.PassRate,
		FailCount: verdict.FailCount,
		Payload:   verdict,
	}
	return event, appendNDJSON(path, event)
}

func emitSprintReportTo(path string, report SprintReport) (EvoSpineEvent, error) {
	verdictStr := "pass"
	if !report.Verdict.Pass {
		verdictStr = "fail"
	}
	event := EvoSpineEvent{
		Type:      "eval_sprint_report",
		Source:    "evalharness",
		Timestamp: "2026-05-25T00:00:00Z",
		SprintID:  report.SprintID,
		Verdict:   verdictStr,
		PassRate:  report.Verdict.PassRate,
		FailCount: report.Verdict.FailCount,
		Payload:   report,
	}
	return event, appendNDJSON(path, event)
}
