package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultUsageDir returns ~/.cursor/claude-usage/.
func DefaultUsageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cursor", "claude-usage")
}

// UsageFilePath returns the JSONL file path for the given date.
func UsageFilePath(dir string, t time.Time) string {
	return filepath.Join(dir, t.Format("2006-01-02")+".jsonl")
}

// AppendUsage writes a Usage record as a JSONL line with atomic append.
func AppendUsage(dir string, u Usage) error {
	if u.Timestamp.IsZero() {
		u.Timestamp = time.Now().UTC()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating usage dir: %w", err)
	}
	path := UsageFilePath(dir, u.Timestamp)
	data, err := json.Marshal(u)
	if err != nil {
		return fmt.Errorf("marshalling usage: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening usage file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing usage: %w", err)
	}
	return nil
}

// ReadUsage reads all Usage records from a JSONL file.
func ReadUsage(path string) ([]Usage, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening usage file: %w", err)
	}
	defer f.Close()

	var records []Usage
	dec := json.NewDecoder(f)
	for dec.More() {
		var u Usage
		if err := dec.Decode(&u); err != nil {
			continue
		}
		records = append(records, u)
	}
	return records, nil
}
