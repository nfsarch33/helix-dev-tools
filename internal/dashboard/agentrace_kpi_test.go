package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeAgentraceKPI_Empty(t *testing.T) {
	summary := ComputeAgentraceKPI(nil)
	assert.Equal(t, 0, summary.TotalEvents)
	assert.NotNil(t, summary.TopTools)
	assert.NotNil(t, summary.HourlyTrend)
	assert.Equal(t, 0.0, summary.ErrorRate)
}

func TestComputeAgentraceKPI_BasicCounts(t *testing.T) {
	events := []AgentraceKPIEvent{
		{Timestamp: "2026-05-25T10:00:00Z", EventType: "tool_call", Tool: "memory.search", Success: true, DurationMS: 50},
		{Timestamp: "2026-05-25T10:01:00Z", EventType: "tool_call", Tool: "memory.search", Success: true, DurationMS: 30},
		{Timestamp: "2026-05-25T10:02:00Z", EventType: "tool_call", Tool: "sprintboard.list", Success: true, DurationMS: 100},
		{Timestamp: "2026-05-25T11:00:00Z", EventType: "tool_call", Tool: "memory.search", Success: false, ErrorMessage: "timeout", DurationMS: 5000},
		{Timestamp: "2026-05-25T11:01:00Z", EventType: "lifecycle", Tool: "", Success: true},
	}

	summary := ComputeAgentraceKPI(events)
	assert.Equal(t, 5, summary.TotalEvents)
	assert.Equal(t, 4, summary.EventsByType["tool_call"])
	assert.Equal(t, 1, summary.EventsByType["lifecycle"])
	assert.Equal(t, 1, summary.ErrorCount)
	assert.Equal(t, 4, summary.SuccessCount)
	assert.InDelta(t, 0.2, summary.ErrorRate, 0.001)

	assert.Len(t, summary.TopTools, 2)
	assert.Equal(t, "memory.search", summary.TopTools[0].Tool)
	assert.Equal(t, 3, summary.TopTools[0].Count)
	assert.Equal(t, "sprintboard.list", summary.TopTools[1].Tool)
	assert.Equal(t, 1, summary.TopTools[1].Count)

	assert.Len(t, summary.HourlyTrend, 2)
	assert.Equal(t, "2026-05-25T10:00:00Z", summary.HourlyTrend[0].Hour)
	assert.Equal(t, 3, summary.HourlyTrend[0].Count)
	assert.Equal(t, "2026-05-25T11:00:00Z", summary.HourlyTrend[1].Hour)
	assert.Equal(t, 2, summary.HourlyTrend[1].Count)

	assert.NotNil(t, summary.TimeRange)
	assert.Equal(t, "2026-05-25T10:00:00Z", summary.TimeRange.Earliest)
	assert.Equal(t, "2026-05-25T11:01:00Z", summary.TimeRange.Latest)

	assert.InDelta(t, 1295.0, summary.AvgDurationMS, 1.0)
}

func TestComputeAgentraceKPI_EventFieldFallback(t *testing.T) {
	events := []AgentraceKPIEvent{
		{Timestamp: "2026-05-25T10:00:00Z", Event: "semble_search", Tool: "semble", Success: true},
		{Timestamp: "2026-05-25T10:00:01Z", Event: "grep_fallback", Tool: "grep", Success: true},
	}

	summary := ComputeAgentraceKPI(events)
	assert.Equal(t, 1, summary.EventsByType["semble_search"])
	assert.Equal(t, 1, summary.EventsByType["grep_fallback"])
}

func TestComputeAgentraceKPI_TopToolsLimit(t *testing.T) {
	var events []AgentraceKPIEvent
	for i := 0; i < 30; i++ {
		events = append(events, AgentraceKPIEvent{
			Timestamp: "2026-05-25T10:00:00Z",
			EventType: "tool_call",
			Tool:      "tool_" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Success:   true,
		})
	}

	summary := ComputeAgentraceKPI(events)
	assert.LessOrEqual(t, len(summary.TopTools), 20)
}

func TestParseAgentraceNDJSON(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "agentrace.ndjson")

	lines := `{"ts":"2026-05-25T10:00:00Z","event_type":"tool_call","tool":"mem.search","success":true,"duration_ms":42}
{"ts":"2026-05-25T10:01:00Z","event_type":"tool_call","tool":"mem.search","success":false,"error_message":"timeout"}
invalid-json-line-should-be-skipped
{"ts":"2026-05-25T10:02:00Z","event_type":"lifecycle","success":true}
`
	require.NoError(t, os.WriteFile(logFile, []byte(lines), 0o644))

	events, err := ParseAgentraceNDJSON(logFile)
	require.NoError(t, err)
	assert.Len(t, events, 3)
	assert.Equal(t, "tool_call", events[0].EventType)
	assert.Equal(t, "mem.search", events[0].Tool)
	assert.True(t, events[0].Success)
	assert.Equal(t, int64(42), events[0].DurationMS)
	assert.False(t, events[1].Success)
	assert.Equal(t, "timeout", events[1].ErrorMessage)
}

func TestParseAgentraceNDJSON_FileNotFound(t *testing.T) {
	_, err := ParseAgentraceNDJSON("/nonexistent/path/agentrace.ndjson")
	assert.Error(t, err)
}

func TestHandleAgentraceKPI_Integration(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "agentrace.ndjson")

	lines := `{"ts":"2026-05-25T10:00:00Z","event_type":"tool_call","tool":"alpha","success":true,"duration_ms":10}
{"ts":"2026-05-25T10:01:00Z","event_type":"tool_call","tool":"beta","success":true,"duration_ms":20}
{"ts":"2026-05-25T10:02:00Z","event_type":"tool_call","tool":"alpha","success":false,"error_message":"fail","duration_ms":5}
`
	require.NoError(t, os.WriteFile(logFile, []byte(lines), 0o644))

	srv := newTestServer(t)
	srv.AgentraceLogPath = logFile
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/agentrace/kpi", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var summary AgentraceKPISummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summary))
	assert.Equal(t, 3, summary.TotalEvents)
	assert.Equal(t, 3, summary.EventsByType["tool_call"])
	assert.Equal(t, 1, summary.ErrorCount)
	assert.Equal(t, 2, summary.SuccessCount)
	assert.InDelta(t, 1.0/3.0, summary.ErrorRate, 0.001)
	assert.Len(t, summary.TopTools, 2)
	assert.Equal(t, "alpha", summary.TopTools[0].Tool)
	assert.Equal(t, 2, summary.TopTools[0].Count)
}

func TestHandleAgentraceKPI_MissingFile(t *testing.T) {
	srv := newTestServer(t)
	srv.AgentraceLogPath = "/nonexistent/agentrace.ndjson"
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/agentrace/kpi", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary AgentraceKPISummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summary))
	assert.Equal(t, 0, summary.TotalEvents)
	assert.NotNil(t, summary.TopTools)
	assert.NotNil(t, summary.HourlyTrend)
}

func TestHandleAgentraceKPI_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(logFile, []byte(""), 0o644))

	srv := newTestServer(t)
	srv.AgentraceLogPath = logFile
	handler := srv.Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/agentrace/kpi", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	var summary AgentraceKPISummary
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &summary))
	assert.Equal(t, 0, summary.TotalEvents)
}

func TestComputeAgentraceKPI_AllErrors(t *testing.T) {
	events := []AgentraceKPIEvent{
		{Timestamp: "2026-05-25T10:00:00Z", EventType: "tool_call", Tool: "x", Success: false, ErrorMessage: "e1"},
		{Timestamp: "2026-05-25T10:00:01Z", EventType: "tool_call", Tool: "y", Success: false, ErrorMessage: "e2"},
	}

	summary := ComputeAgentraceKPI(events)
	assert.Equal(t, 2, summary.ErrorCount)
	assert.Equal(t, 0, summary.SuccessCount)
	assert.Equal(t, 1.0, summary.ErrorRate)
}
