// T-8800-B23: agentrace NDJSON emission for guard-mcp.
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// guardMcpAgentraceEvent is the structured row emitted to the agentrace NDJSON
// sink for each MCP call observed by guard-mcp.
type guardMcpAgentraceEvent struct {
	Timestamp string `json:"ts"`
	Source    string `json:"source"`
	Tool      string `json:"tool"`
	Server    string `json:"server,omitempty"`
	Action    string `json:"action"`
	LatencyMs int64  `json:"latency_ms"`
	BytesIn   int64  `json:"bytes_in"`
	Memory    string `json:"memory_layer,omitempty"`
	MemoryOp  string `json:"memory_op,omitempty"`
}

// agentraceMu serialises writes across concurrent guard-mcp invocations
// in the same process (tests in particular). Cross-process safety relies
// on append-mode semantics of os.OpenFile + the kernel's atomic write
// guarantee for small NDJSON rows.
var agentraceMu sync.Mutex

// defaultAgentracePath returns the canonical NDJSON sink for guard-mcp.
// Honours the AGENTRACE_LOG_PATH env override; otherwise falls back to
// ~/.cursor/hooks/agentrace.ndjson which the EvoSpine reducer already
// scans.
func defaultAgentracePath() string {
	if p := os.Getenv("AGENTRACE_LOG_PATH"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cursor", "hooks", "agentrace.ndjson")
}

// appendAgentraceEvent appends a single NDJSON row to path. Errors during
// directory creation or write are swallowed so guard-mcp never fails the
// MCP call because of telemetry trouble.
func appendAgentraceEvent(path string, ev guardMcpAgentraceEvent) error {
	if path == "" {
		return nil
	}
	agentraceMu.Lock()
	defer agentraceMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(ev)
}
