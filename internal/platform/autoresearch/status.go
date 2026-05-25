package autoresearch

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// ResearchStatus summarizes the state of the autoresearch loop.
type ResearchStatus struct {
	TotalIterations int            `json:"total_iterations"`
	KeepCount       int            `json:"keep_count"`
	DiscardCount    int            `json:"discard_count"`
	LastDecision    Decision       `json:"last_decision,omitempty"`
	LastMetric      float64        `json:"last_metric,omitempty"`
	LastDelta       float64        `json:"last_delta,omitempty"`
	LastRun         time.Time      `json:"last_run,omitempty"`
	LogEntries      int            `json:"log_entries"`
	LogPath         string         `json:"log_path,omitempty"`
	History         []LoopState    `json:"history,omitempty"`
}

// BuildStatus constructs a ResearchStatus from run history and the log file.
func BuildStatus(history []LoopState, logPath string) ResearchStatus {
	s := ResearchStatus{
		TotalIterations: len(history),
		LogPath:         logPath,
		History:         history,
	}
	for _, h := range history {
		switch h.Decision {
		case DecisionKeep:
			s.KeepCount++
		case DecisionDiscard:
			s.DiscardCount++
		}
	}
	if len(history) > 0 {
		last := history[len(history)-1]
		s.LastDecision = last.Decision
		s.LastMetric = last.Metric
		s.LastDelta = last.Delta
		s.LastRun = last.Timestamp
	}
	s.LogEntries = countLogLines(logPath)
	return s
}

// LoadStatusFromLog reconstructs status purely from the NDJSON log file,
// without needing an active runner.
func LoadStatusFromLog(logPath string) (ResearchStatus, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return ResearchStatus{LogPath: logPath}, err
	}
	defer f.Close()

	var (
		lastIter    int
		keepCount   int
		discardCount int
		lastDecision Decision
		lastMetric  float64
		lastDelta   float64
		lastRun     time.Time
		lineCount   int
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		lineCount++
		var rec map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}

		if iter, ok := rec["iteration"].(float64); ok {
			if int(iter) > lastIter {
				lastIter = int(iter)
			}
		}

		if d, ok := rec["decision"].(string); ok {
			switch Decision(d) {
			case DecisionKeep:
				lastDecision = DecisionKeep
				keepCount++
			case DecisionDiscard:
				lastDecision = DecisionDiscard
				discardCount++
			}
		}

		if m, ok := rec["metric"].(float64); ok {
			lastMetric = m
		}
		if d, ok := rec["delta"].(float64); ok {
			lastDelta = d
		}
		if ts, ok := rec["ts"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				lastRun = t
			}
		}
	}

	return ResearchStatus{
		TotalIterations: lastIter,
		KeepCount:       keepCount,
		DiscardCount:    discardCount,
		LastDecision:    lastDecision,
		LastMetric:      lastMetric,
		LastDelta:       lastDelta,
		LastRun:         lastRun,
		LogEntries:      lineCount,
		LogPath:         logPath,
	}, nil
}

func countLogLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count
}
