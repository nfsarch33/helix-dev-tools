package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/offload-telemetry/envelope"
)

// TestTierARecord_AppendsRedactedJSONL drives the
// `cursor-tools tier-a metric record` surface. The recorder must:
//   - validate enum-like fields (tier, decision, route)
//   - reject free-form payload labels at record time
//   - append one canonical NDJSON record per call to the configured path
//   - never include any field other than the redacted whitelist
func TestTierARecord_AppendsRedactedJSONL(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "tier-a-metrics.jsonl")

	args := tierARecordArgs{
		Tier:               "a",
		Decision:           "offloaded",
		Route:              "claude_code_subagent",
		Model:              "us.anthropic.claude-opus-4-7",
		LatencyMS:          1234,
		TokensPerSecond:    44.5,
		TimeToFirstTokenMS: 250,
		CostUSD:            0.0123,
		StatusCode:         200,
		ParentTaskID:       "task-123",
		Sender:             "cursor-ide",
	}
	if err := runTierARecord(path, args); err != nil {
		t.Fatalf("runTierARecord: %v", err)
	}
	if err := runTierARecord(path, tierARecordArgs{
		Tier:      "a",
		Decision:  "kept_local",
		Route:     "router_qwen36_27b",
		LatencyMS: 28,
		Sender:    "router",
	}); err != nil {
		t.Fatalf("runTierARecord: %v", err)
	}

	got := readJSONL(t, path)
	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	checkAllowed := map[string]bool{
		"recorded_at":            true,
		"schema_version":         true,
		"tier":                   true,
		"decision":               true,
		"route":                  true,
		"model":                  true,
		"latency_ms":             true,
		"tokens_per_second":      true,
		"time_to_first_token_ms": true,
		"cost_usd":               true,
		"status_code":            true,
		"parent_task_id":         true,
		"sender":                 true,
	}
	for i, rec := range got {
		for k := range rec {
			if !checkAllowed[k] {
				t.Fatalf("record %d has forbidden field %q (redaction violation): %#v", i, k, rec)
			}
		}
		if rec["tier"] == "" || rec["decision"] == "" || rec["route"] == "" {
			t.Fatalf("record %d missing required field: %#v", i, rec)
		}
		if _, err := time.Parse(time.RFC3339Nano, rec["recorded_at"].(string)); err != nil {
			t.Fatalf("record %d: invalid recorded_at: %v", i, err)
		}
	}
	if got[0]["tier"] != "a" || got[0]["decision"] != "offloaded" || got[0]["route"] != "claude_code_subagent" {
		t.Fatalf("record 0 mismatched: %#v", got[0])
	}
	if v := got[0]["latency_ms"].(float64); v != 1234 {
		t.Fatalf("record 0 latency: got %v want 1234", v)
	}
	if got[0]["schema_version"] != "offload.telemetry.v1" {
		t.Fatalf("schema_version = %v", got[0]["schema_version"])
	}
	if got[0]["parent_task_id"] != "task-123" {
		t.Fatalf("parent_task_id = %v", got[0]["parent_task_id"])
	}
}

func TestTierARecord_UsesSharedOffloadEnvelopeSchema(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "tier-a-metrics.jsonl")

	args := tierARecordArgs{
		Tier:               "a",
		Decision:           "offloaded",
		Route:              "claude_code_subagent",
		Model:              "us.anthropic.claude-opus-4-7",
		LatencyMS:          1234,
		TokensPerSecond:    44.5,
		TimeToFirstTokenMS: 250,
		CostUSD:            0.0123,
		StatusCode:         200,
		ParentTaskID:       "task-123",
		Sender:             "cursor-ide",
	}
	if err := runTierARecord(path, args); err != nil {
		t.Fatalf("runTierARecord: %v", err)
	}

	got := readJSONL(t, path)[0]
	expected := envelope.NewEvent(envelope.Input{
		RecordedAt:          got["recorded_at"].(string),
		Tier:                args.Tier,
		Decision:            args.Decision,
		Route:               args.Route,
		Model:               args.Model,
		LatencyMS:           args.LatencyMS,
		TokensPerSecond:     args.TokensPerSecond,
		TimeToFirstTokenMS:  args.TimeToFirstTokenMS,
		CostUSD:             args.CostUSD,
		StatusCode:          args.StatusCode,
		ParentTaskID:        args.ParentTaskID,
		Sender:              args.Sender,
		Prompt:              "must-not-appear",
		Body:                "must-not-appear",
		Secret:              "must-not-appear",
		ProviderToken:       "must-not-appear",
		AuthorizationHeader: "must-not-appear",
	})
	encoded, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("marshal expected envelope: %v", err)
	}
	var want map[string]interface{}
	if err := json.Unmarshal(encoded, &want); err != nil {
		t.Fatalf("unmarshal expected envelope: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("field count mismatch got=%d want=%d got=%#v want=%#v", len(got), len(want), got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("field %s mismatch got=%#v want=%#v full=%#v", k, got[k], v, got)
		}
	}
}

// TestTierARecord_RejectsBadInputs covers boundary validation. The CLI
// surface should fail loud rather than silently dropping records, since
// the metrics drive promotion decisions.
func TestTierARecord_RejectsBadInputs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "tier-a-metrics.jsonl")

	cases := []struct {
		name string
		args tierARecordArgs
	}{
		{"empty tier", tierARecordArgs{Tier: "", Decision: "offloaded", Route: "x", LatencyMS: 0}},
		{"unknown tier", tierARecordArgs{Tier: "z", Decision: "offloaded", Route: "x", LatencyMS: 0}},
		{"empty decision", tierARecordArgs{Tier: "a", Decision: "", Route: "x", LatencyMS: 0}},
		{"unknown decision", tierARecordArgs{Tier: "a", Decision: "yolo", Route: "x", LatencyMS: 0}},
		{"empty route", tierARecordArgs{Tier: "a", Decision: "offloaded", Route: "", LatencyMS: 0}},
		{"route too long", tierARecordArgs{Tier: "a", Decision: "offloaded", Route: strings.Repeat("x", 257), LatencyMS: 0}},
		{"route with spaces", tierARecordArgs{Tier: "a", Decision: "offloaded", Route: "claude code", LatencyMS: 0}},
		{"negative latency", tierARecordArgs{Tier: "a", Decision: "offloaded", Route: "x", LatencyMS: -5}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := runTierARecord(path, c.args); err == nil {
				t.Fatalf("expected validation error for %s, got nil", c.name)
			}
		})
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected JSONL not to be created on validation failures, got err=%v", err)
	}
}

// TestTierASummary_AggregatesByTierDecisionRoute proves the markdown
// summary command aggregates JSONL records by the canonical label
// tuple, with stable ordering for diffability.
func TestTierASummary_AggregatesByTierDecisionRoute(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "tier-a-metrics.jsonl")

	for i := 0; i < 3; i++ {
		if err := runTierARecord(path, tierARecordArgs{
			Tier: "a", Decision: "offloaded", Route: "claude_code_subagent", LatencyMS: 1000 + int64(i)*100,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := runTierARecord(path, tierARecordArgs{
		Tier: "a", Decision: "kept_local", Route: "router_qwen36_27b", LatencyMS: 25,
	}); err != nil {
		t.Fatal(err)
	}
	if err := runTierARecord(path, tierARecordArgs{
		Tier: "a", Decision: "declined", Route: "codex_subagent", LatencyMS: 0,
	}); err != nil {
		t.Fatal(err)
	}

	out, err := tierASummary(path)
	if err != nil {
		t.Fatalf("tierASummary: %v", err)
	}
	for _, want := range []string{
		"# Tier-A Offload Telemetry Summary",
		"| tier | decision | route | count | p50_latency_ms |",
		"| a | declined | codex_subagent | 1 |",
		"| a | kept_local | router_qwen36_27b | 1 |",
		"| a | offloaded | claude_code_subagent | 3 |",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q\n--- full output:\n%s", want, out)
		}
	}
}

func readJSONL(t *testing.T, path string) []map[string]interface{} {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	out := []map[string]interface{}{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid jsonl line %q: %v", line, err)
		}
		out = append(out, m)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return out
}
