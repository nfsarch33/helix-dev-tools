package agentstatus

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AgentTraceSource reads agent status from agentrace NDJSON transcripts.
type AgentTraceSource struct {
	transcriptsDir string
	maxAge         time.Duration
}

// NewAgentTraceSource creates a source that scans the given transcripts directory.
func NewAgentTraceSource(transcriptsDir string, maxAge time.Duration) *AgentTraceSource {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	return &AgentTraceSource{
		transcriptsDir: transcriptsDir,
		maxAge:         maxAge,
	}
}

func (s *AgentTraceSource) Name() string { return "agentrace" }

type transcriptEntry struct {
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"`
	ToolUse   bool   `json:"tool_use"`
}

// Read scans recent NDJSON transcripts and derives agent states.
func (s *AgentTraceSource) Read(ctx context.Context) ([]AgentStatus, error) {
	entries, err := os.ReadDir(s.transcriptsDir)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-s.maxAge)
	var statuses []AgentStatus

	for _, entry := range entries {
		if ctx.Err() != nil {
			return statuses, ctx.Err()
		}
		if !entry.IsDir() {
			continue
		}

		jsonlPath := filepath.Join(s.transcriptsDir, entry.Name(), entry.Name()+".jsonl")
		info, err := os.Stat(jsonlPath)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			continue
		}

		lastActivity, running := scanTranscript(jsonlPath)
		state := StateIdle
		if running {
			state = StateRunning
		}

		statuses = append(statuses, AgentStatus{
			Name:         entry.Name()[:8],
			State:        state,
			LastActivity: lastActivity,
			RunID:        entry.Name(),
			Metadata:     map[string]string{"source": "agentrace"},
		})
	}

	return statuses, nil
}

func scanTranscript(path string) (lastActivity time.Time, isRunning bool) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}
	defer f.Close()

	stat, _ := f.Stat()
	offset := stat.Size() - 8192
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return time.Time{}, false
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)

	var last transcriptEntry
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry transcriptEntry
		if json.Unmarshal([]byte(line), &entry) == nil {
			last = entry
		}
	}

	if last.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, last.Timestamp); err == nil {
			lastActivity = t
		}
	}
	if lastActivity.IsZero() {
		lastActivity = stat.ModTime()
	}

	isRunning = time.Since(lastActivity) < 5*time.Minute
	return lastActivity, isRunning
}
