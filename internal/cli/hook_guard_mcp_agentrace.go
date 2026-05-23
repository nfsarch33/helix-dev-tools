// T-8800-B23: agentrace NDJSON emission for guard-mcp.
//
// The append path is delegated to internal/ndjson (T-8800-B19/B22) so every
// new NDJSON sink in this binary shares the same race-safe writer contract
// with helixon-ec/internal/ndjson and sprintboard-mcp.
package cli

import (
	"os"
	"path/filepath"

	"github.com/nfsarch33/helix-dev-tools/internal/ndjson"
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
// directory creation or write are swallowed by the caller so guard-mcp never
// fails the MCP call because of telemetry trouble.
func appendAgentraceEvent(path string, ev guardMcpAgentraceEvent) error {
	return ndjson.AppendOne(path, ev)
}
