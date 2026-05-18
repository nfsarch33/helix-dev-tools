package autoresearch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SentruxMetric is a quality score event emitted after the Evaluate phase.
type SentruxMetric struct {
	Timestamp  string  `json:"ts"`
	AgentID    string  `json:"agent_id"`
	Iteration  int     `json:"iteration"`
	Score      float64 `json:"score"`
	BaseScore  float64 `json:"base_score"`
	Delta      float64 `json:"delta"`
	RepoPath   string  `json:"repo_path,omitempty"`
}

// SentruxPlugin records a quality score delta to the sentrux metrics NDJSON
// file so EvoSpine can consume it during ORHEP cycles.
// logPath defaults to ~/logs/runx/sentrux-autoresearch.ndjson.
func SentruxPlugin(agentID string, iter int, score, baseScore float64, repoPath, logPath string) error {
	if logPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("home dir: %w", err)
		}
		logPath = filepath.Join(home, "logs", "runx", "sentrux-autoresearch.ndjson")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	m := SentruxMetric{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		AgentID:   agentID,
		Iteration: iter,
		Score:     score,
		BaseScore: baseScore,
		Delta:     score - baseScore,
		RepoPath:  repoPath,
	}

	line, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}
