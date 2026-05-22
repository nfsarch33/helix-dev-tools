// T-8800-B23 tests: guard-mcp must append a structured NDJSON event to the
// agentrace sink for every observed MCP call, including allow/deny/warn
// outcomes, and must not fail the call when the sink is missing.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
)

func newAgentraceTempHandler(t *testing.T) (*guardMcpHandler, string, string) {
	t.Helper()
	tmp := t.TempDir()
	metrics := filepath.Join(tmp, "metrics.jsonl")
	agentrace := filepath.Join(tmp, "agentrace.ndjson")
	return &guardMcpHandler{
		log:           logger.New(filepath.Join(tmp, "test.log")),
		metricsPath:   metrics,
		agentracePath: agentrace,
	}, metrics, agentrace
}

func readAgentraceRows(t *testing.T, path string) []guardMcpAgentraceEvent {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open agentrace: %v", err)
	}
	defer f.Close()
	var rows []guardMcpAgentraceEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev guardMcpAgentraceEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("unmarshal row %q: %v", sc.Text(), err)
		}
		rows = append(rows, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return rows
}

func TestGuardMcp_EmitsAgentraceOnAllow(t *testing.T) {
	h, _, ndjson := newAgentraceTempHandler(t)
	in := &hookio.Input{ToolName: "search", ToolInput: `{"q":"x"}`}
	if _, err := h.Handle(context.Background(), in); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	rows := readAgentraceRows(t, ndjson)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.Source != "guard-mcp" || r.Tool != "search" || r.Action != "allow" {
		t.Fatalf("unexpected event: %+v", r)
	}
	if r.Timestamp == "" {
		t.Fatalf("timestamp not set")
	}
	if r.BytesIn == 0 {
		t.Fatalf("bytes_in not recorded")
	}
}

func TestGuardMcp_EmitsAgentraceWithMemoryMeta(t *testing.T) {
	h, _, ndjson := newAgentraceTempHandler(t)
	in := &hookio.Input{ToolName: "search_memories", ToolInput: `{"q":"r"}`}
	if _, err := h.Handle(context.Background(), in); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	rows := readAgentraceRows(t, ndjson)
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Memory != "mem0" || rows[0].MemoryOp != "search" {
		t.Fatalf("memory meta missing: %+v", rows[0])
	}
}

func TestGuardMcp_AgentraceSinkOptional(t *testing.T) {
	tmp := t.TempDir()
	h := &guardMcpHandler{
		log:           logger.New(filepath.Join(tmp, "x.log")),
		metricsPath:   filepath.Join(tmp, "m.jsonl"),
		agentracePath: "",
	}
	in := &hookio.Input{ToolName: "search", ToolInput: "x"}
	resp, err := h.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if resp.Permission != "allow" {
		t.Fatalf("permission = %q", resp.Permission)
	}
}

func TestGuardMcp_AgentraceConcurrentSafe(t *testing.T) {
	h, _, ndjson := newAgentraceTempHandler(t)
	const N = 25
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := &hookio.Input{
				ToolName:  "search",
				ToolInput: strings.Repeat("x", i+1),
			}
			if _, err := h.Handle(context.Background(), in); err != nil {
				t.Errorf("Handle: %v", err)
			}
		}(i)
	}
	wg.Wait()
	rows := readAgentraceRows(t, ndjson)
	if len(rows) != N {
		t.Fatalf("rows = %d, want %d", len(rows), N)
	}
}
