package dashboard

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"time"
)

// AgentraceKPIEvent is a superset struct for parsing NDJSON events from
// multiple agentrace producers (helixon tooldispatch, sembleproxy, hooks).
type AgentraceKPIEvent struct {
	Timestamp    string `json:"ts"`
	EventType    string `json:"event_type"`
	Event        string `json:"event"`
	Tool         string `json:"tool,omitempty"`
	Server       string `json:"server,omitempty"`
	AgentID      string `json:"agent_id,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	Query        string `json:"query,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
}

// AgentraceKPISummary is the JSON response for /api/agentrace/kpi.
type AgentraceKPISummary struct {
	TotalEvents   int              `json:"total_events"`
	EventsByType  map[string]int   `json:"events_by_type"`
	TopTools      []ToolInvocation `json:"top_tools"`
	ErrorRate     float64          `json:"error_rate"`
	ErrorCount    int              `json:"error_count"`
	SuccessCount  int              `json:"success_count"`
	HourlyTrend   []HourlyBucket   `json:"hourly_trend"`
	AvgDurationMS float64          `json:"avg_duration_ms,omitempty"`
	TimeRange     *TimeRange       `json:"time_range,omitempty"`
}

// ToolInvocation tracks invocation counts per tool.
type ToolInvocation struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

// HourlyBucket is one hour's event count.
type HourlyBucket struct {
	Hour  string `json:"hour"` // RFC3339 truncated to hour
	Count int    `json:"count"`
}

// TimeRange captures the span of events processed.
type TimeRange struct {
	Earliest string `json:"earliest"`
	Latest   string `json:"latest"`
}

// ComputeAgentraceKPI processes a slice of NDJSON events into a KPI summary.
func ComputeAgentraceKPI(events []AgentraceKPIEvent) AgentraceKPISummary {
	summary := AgentraceKPISummary{
		EventsByType: make(map[string]int),
	}
	if len(events) == 0 {
		summary.TopTools = []ToolInvocation{}
		summary.HourlyTrend = []HourlyBucket{}
		return summary
	}

	toolCounts := make(map[string]int)
	hourCounts := make(map[string]int)
	var totalDuration int64
	var durationCount int
	var earliest, latest string

	for _, ev := range events {
		summary.TotalEvents++

		eventType := ev.EventType
		if eventType == "" {
			eventType = ev.Event
		}
		if eventType == "" {
			eventType = "unknown"
		}
		summary.EventsByType[eventType]++

		if ev.Success {
			summary.SuccessCount++
		} else if ev.ErrorMessage != "" || eventType == "tool_call" {
			summary.ErrorCount++
		} else {
			summary.SuccessCount++
		}

		if ev.Tool != "" {
			toolCounts[ev.Tool]++
		}

		if ev.DurationMS > 0 {
			totalDuration += ev.DurationMS
			durationCount++
		}

		ts := ev.Timestamp
		if ts != "" {
			if earliest == "" || ts < earliest {
				earliest = ts
			}
			if latest == "" || ts > latest {
				latest = ts
			}
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				hourKey := t.UTC().Truncate(time.Hour).Format(time.RFC3339)
				hourCounts[hourKey]++
			} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
				hourKey := t.UTC().Truncate(time.Hour).Format(time.RFC3339)
				hourCounts[hourKey]++
			}
		}
	}

	if summary.TotalEvents > 0 {
		summary.ErrorRate = float64(summary.ErrorCount) / float64(summary.TotalEvents)
	}
	if durationCount > 0 {
		summary.AvgDurationMS = float64(totalDuration) / float64(durationCount)
	}

	if earliest != "" {
		summary.TimeRange = &TimeRange{Earliest: earliest, Latest: latest}
	}

	// Top tools sorted by count descending, limit to 20.
	type kv struct {
		k string
		v int
	}
	toolSlice := make([]kv, 0, len(toolCounts))
	for k, v := range toolCounts {
		toolSlice = append(toolSlice, kv{k, v})
	}
	sort.Slice(toolSlice, func(i, j int) bool { return toolSlice[i].v > toolSlice[j].v })
	limit := 20
	if len(toolSlice) < limit {
		limit = len(toolSlice)
	}
	summary.TopTools = make([]ToolInvocation, limit)
	for i := 0; i < limit; i++ {
		summary.TopTools[i] = ToolInvocation{Tool: toolSlice[i].k, Count: toolSlice[i].v}
	}

	// Hourly trend sorted chronologically.
	hourSlice := make([]HourlyBucket, 0, len(hourCounts))
	for h, c := range hourCounts {
		hourSlice = append(hourSlice, HourlyBucket{Hour: h, Count: c})
	}
	sort.Slice(hourSlice, func(i, j int) bool { return hourSlice[i].Hour < hourSlice[j].Hour })
	summary.HourlyTrend = hourSlice

	return summary
}

// ParseAgentraceNDJSON reads all events from an NDJSON file at path.
func ParseAgentraceNDJSON(path string) ([]AgentraceKPIEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []AgentraceKPIEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev AgentraceKPIEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

// handleAgentraceKPI serves GET /api/agentrace/kpi.
func (s *Server) handleAgentraceKPI(w http.ResponseWriter, r *http.Request) {
	logPath := s.agentraceLogPath()

	events, err := ParseAgentraceNDJSON(logPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AgentraceKPISummary{
			EventsByType: map[string]int{},
			TopTools:     []ToolInvocation{},
			HourlyTrend:  []HourlyBucket{},
		})
		return
	}

	summary := ComputeAgentraceKPI(events)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (s *Server) agentraceLogPath() string {
	if s.AgentraceLogPath != "" {
		return s.AgentraceLogPath
	}
	home, _ := os.UserHomeDir()
	return home + "/logs/runx/agentrace-mcp.ndjson"
}
