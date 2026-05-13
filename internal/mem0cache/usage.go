package mem0cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageEvent represents a single NDJSON log entry.
type UsageEvent struct {
	Timestamp string                 `json:"ts"`
	Event     string                 `json:"event"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// UsageTracker appends usage events to an NDJSON file.
type UsageTracker struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// UsageConfig controls where usage logs are written.
type UsageConfig struct {
	LogPath string // defaults to ~/logs/runx/mem0-usage.ndjson
}

func defaultUsageLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, "logs", "runx")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create log dir: %w", err)
	}
	return filepath.Join(dir, "mem0-usage.ndjson"), nil
}

// NewUsageTracker opens the NDJSON log file for append.
func NewUsageTracker(cfg UsageConfig) (*UsageTracker, error) {
	logPath := cfg.LogPath
	if logPath == "" {
		var err error
		logPath, err = defaultUsageLogPath()
		if err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open usage log: %w", err)
	}

	return &UsageTracker{path: logPath, file: f}, nil
}

func (u *UsageTracker) write(event UsageEvent) {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().Format(time.RFC3339)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	data = append(data, '\n')

	u.mu.Lock()
	defer u.mu.Unlock()
	_, _ = u.file.Write(data)
}

// LogAdd records an add operation.
func (u *UsageTracker) LogAdd(dedupHit bool) {
	u.write(UsageEvent{
		Event: "mem0_add",
		Meta:  map[string]interface{}{"dedup_hit": dedupHit},
	})
}

// LogSearch records a search operation.
func (u *UsageTracker) LogSearch(cacheHit bool) {
	u.write(UsageEvent{
		Event: "mem0_search",
		Meta:  map[string]interface{}{"cache_hit": cacheHit},
	})
}

// LogFlush records a batch flush event.
func (u *UsageTracker) LogFlush(count int) {
	u.write(UsageEvent{
		Event: "mem0_batch_flush",
		Meta:  map[string]interface{}{"count": count},
	})
}

// LogRateLimit records a rate-limited event.
func (u *UsageTracker) LogRateLimit(op string) {
	u.write(UsageEvent{
		Event: "mem0_rate_limited",
		Meta:  map[string]interface{}{"op": op},
	})
}

// Close flushes and closes the log file.
func (u *UsageTracker) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.file.Close()
}

// Path returns the log file path.
func (u *UsageTracker) Path() string {
	return u.path
}
