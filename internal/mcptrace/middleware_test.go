package mcptrace_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/mcptrace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_Disabled(t *testing.T) {
	m := mcptrace.NewMiddleware(mcptrace.Config{Enabled: false})
	assert.False(t, m.IsEnabled())
}

func TestMiddleware_Enabled(t *testing.T) {
	m := mcptrace.NewMiddleware(mcptrace.Config{Enabled: true, LogPath: "/tmp/test-agentrace.ndjson"})
	assert.True(t, m.IsEnabled())
}

func TestMiddleware_RecordCall(t *testing.T) {
	var buf bytes.Buffer
	m := mcptrace.NewMiddleware(mcptrace.Config{Enabled: true})
	m.SetWriter(&buf)

	m.RecordCall("ticket_create", map[string]interface{}{
		"id":    "T-001",
		"title": "test ticket",
	}, 150*time.Millisecond, nil)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.Len(t, lines, 1)

	var event mcptrace.TraceEvent
	err := json.Unmarshal(lines[0], &event)
	require.NoError(t, err)
	assert.Equal(t, "mcp_tool_call", event.EventType)
	assert.Equal(t, "ticket_create", event.ToolName)
	assert.Equal(t, "150ms", event.Duration)
	assert.True(t, event.Success)
	assert.Equal(t, "T-001", event.Args["id"])
}

func TestMiddleware_RecordError(t *testing.T) {
	var buf bytes.Buffer
	m := mcptrace.NewMiddleware(mcptrace.Config{Enabled: true})
	m.SetWriter(&buf)

	m.RecordCall("mem0_search", map[string]interface{}{
		"query": "test",
	}, 30*time.Second, assert.AnError)

	var event mcptrace.TraceEvent
	err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &event)
	require.NoError(t, err)
	assert.False(t, event.Success)
	assert.Contains(t, event.Error, "assert.AnError")
}

func TestMiddleware_FileOutput(t *testing.T) {
	tmpFile := t.TempDir() + "/agentrace-test.ndjson"
	m := mcptrace.NewMiddleware(mcptrace.Config{
		Enabled: true,
		LogPath: tmpFile,
	})
	require.NoError(t, m.Open())
	defer m.Close()

	m.RecordCall("sprint_create", map[string]interface{}{"id": "v6300"}, 50*time.Millisecond, nil)
	m.RecordCall("ticket_list", map[string]interface{}{"sprint_id": "v6300"}, 30*time.Millisecond, nil)

	m.Close()

	data, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	assert.Len(t, lines, 2)
}

func TestConfig_FromEnv(t *testing.T) {
	t.Setenv("AGENTRACE_ENABLED", "true")
	t.Setenv("AGENTRACE_LOG_PATH", "/tmp/custom-trace.ndjson")

	cfg := mcptrace.ConfigFromEnv()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "/tmp/custom-trace.ndjson", cfg.LogPath)
}

func TestConfig_FromEnvDisabled(t *testing.T) {
	t.Setenv("AGENTRACE_ENABLED", "false")
	cfg := mcptrace.ConfigFromEnv()
	assert.False(t, cfg.Enabled)
}

func TestConfig_DefaultPath(t *testing.T) {
	t.Setenv("AGENTRACE_ENABLED", "true")
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := mcptrace.ConfigFromEnv()
	assert.Contains(t, cfg.LogPath, "agentrace-mcp.ndjson")
}

func TestTraceEvent_Serialization(t *testing.T) {
	event := mcptrace.TraceEvent{
		EventType: "mcp_tool_call",
		ToolName:  "task_claim",
		Args:      map[string]interface{}{"ticket_id": "T-005"},
		Duration:  "45ms",
		Success:   true,
		Timestamp: "2026-05-19T10:00:00+10:00",
	}
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"tool_name":"task_claim"`)
}
