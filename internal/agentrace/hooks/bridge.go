package hooks

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/agentrace/reducer"
)

const (
	defaultEventsFile = "events.jsonl"
	filePerms         = 0600
	dirPerms          = 0700
	defaultArchives   = 5
)

var migrateOnce sync.Once

type EventInput struct {
	EventName     string
	Timestamp     int64
	SessionID     string
	AgentID       string
	ParentAgentID string
	ToolCallID    string
	ToolName      string
	Prompt        string
	Output        string
	Error         string
	CostUSD       float64
	InputTokens   int
	OutputTokens  int
	Iteration     int
	ExitCode      int
	Payload       map[string]any
}

// MigrateStateDir moves ~/.tarsa to ~/.agentrace on first call if the
// legacy directory exists and the new one does not. A symlink is left
// at the old path for backward compatibility during one deprecation
// window.
func MigrateStateDir() {
	migrateOnce.Do(func() {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		oldDir := filepath.Join(home, ".tarsa")
		newDir := filepath.Join(home, ".agentrace")

		oldInfo, oldErr := os.Lstat(oldDir)
		if oldErr != nil || !oldInfo.IsDir() {
			return
		}
		if oldInfo.Mode()&os.ModeSymlink != 0 {
			return
		}
		if _, newErr := os.Stat(newDir); newErr == nil {
			return
		}

		if err := os.Rename(oldDir, newDir); err != nil {
			log.Printf("agentrace: migration failed (rename %s -> %s): %v", oldDir, newDir, err)
			return
		}
		if err := os.Symlink(newDir, oldDir); err != nil {
			log.Printf("agentrace: symlink %s -> %s failed: %v", oldDir, newDir, err)
		}
		log.Printf("agentrace: ~/.tarsa is deprecated; state moved to ~/.agentrace")
	})
}

// DefaultEventsPath returns ~/.agentrace/events.jsonl. On first call
// it migrates ~/.tarsa to ~/.agentrace if the legacy directory exists.
func DefaultEventsPath() string {
	MigrateStateDir()
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".agentrace", defaultEventsFile)
	}
	return filepath.Join(home, ".agentrace", defaultEventsFile)
}

// AppendEvent validates raw as JSON then atomically appends one JSONL
// line to the file at path. The file and parent directories are created
// if they do not exist.
func AppendEvent(path string, raw []byte) error {
	if !json.Valid(raw) {
		return fmt.Errorf("invalid JSON payload")
	}

	compact, err := compactJSON(raw)
	if err != nil {
		return fmt.Errorf("compact JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), dirPerms); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, filePerms)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	defer func() { _ = f.Close() }()
	line := append(compact, '\n')
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return nil
}

func BuildEvent(input EventInput) ([]byte, error) {
	eventType, err := normalizeEventType(input.EventName, input.ExitCode, input.Error)
	if err != nil {
		return nil, err
	}

	payload := clonePayload(input.Payload)
	if input.ExitCode != 0 {
		if payload == nil {
			payload = map[string]any{}
		}
		payload["exit_code"] = input.ExitCode
	}

	event := reducer.Event{
		Type:          eventType,
		Timestamp:     input.Timestamp,
		SessionID:     reducer.SessionID(strings.TrimSpace(input.SessionID)),
		AgentID:       reducer.AgentID(strings.TrimSpace(input.AgentID)),
		ParentAgentID: reducer.AgentID(strings.TrimSpace(input.ParentAgentID)),
		ToolCallID:    reducer.ToolCallID(strings.TrimSpace(input.ToolCallID)),
		ToolName:      strings.TrimSpace(input.ToolName),
		Prompt:        input.Prompt,
		Output:        input.Output,
		Error:         input.Error,
		CostUSD:       input.CostUSD,
		InputTokens:   input.InputTokens,
		OutputTokens:  input.OutputTokens,
		Iteration:     input.Iteration,
		Payload:       payload,
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	return json.Marshal(event)
}

type RotationOptions struct {
	MaxBytes    int64
	MaxArchives int
}

type RotationResult struct {
	Rotated     bool
	Path        string
	ArchivePath string
	Bytes       int64
}

func RotateEvents(path string, opts RotationOptions) (RotationResult, error) {
	result := RotationResult{Path: path}
	if opts.MaxBytes <= 0 {
		return result, fmt.Errorf("max bytes must be positive")
	}
	if opts.MaxArchives <= 0 {
		opts.MaxArchives = defaultArchives
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("stat events file: %w", err)
	}
	result.Bytes = info.Size()
	if info.Size() <= opts.MaxBytes {
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), dirPerms); err != nil {
		return result, fmt.Errorf("create directory: %w", err)
	}
	archive := fmt.Sprintf("%s.%s", path, time.Now().UTC().Format("20060102T150405.000000000Z"))
	if err := os.Rename(path, archive); err != nil {
		return result, fmt.Errorf("rotate events file: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, filePerms)
	if err != nil {
		return result, fmt.Errorf("create active events file: %w", err)
	}
	if err := f.Close(); err != nil {
		return result, fmt.Errorf("close active events file: %w", err)
	}
	result.Rotated = true
	result.ArchivePath = archive
	if err := pruneArchives(path, opts.MaxArchives); err != nil {
		return result, err
	}
	return result, nil
}

func pruneArchives(path string, maxArchives int) error {
	matches, err := filepath.Glob(path + ".*")
	if err != nil {
		return fmt.Errorf("glob event archives: %w", err)
	}
	if len(matches) <= maxArchives {
		return nil
	}
	sort.Strings(matches)
	for _, old := range matches[:len(matches)-maxArchives] {
		if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("prune event archive: %w", err)
		}
	}
	return nil
}

func compactJSON(raw []byte) ([]byte, error) {
	var v json.RawMessage
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func normalizeEventType(raw string, exitCode int, errText string) (reducer.EventType, error) {
	key := canonicalEventKey(raw)
	switch key {
	case "userpromptsubmit", "sessionstart":
		return reducer.EventUserPromptSubmit, nil
	case "stop", "sessionend":
		return reducer.EventStop, nil
	case "pretooluse", "shellstart":
		return reducer.EventPreToolUse, nil
	case "posttooluse":
		return reducer.EventPostToolUse, nil
	case "posttoolusefailure":
		return reducer.EventPostToolUseFailure, nil
	case "shellend":
		if exitCode != 0 || strings.TrimSpace(errText) != "" {
			return reducer.EventPostToolUseFailure, nil
		}
		return reducer.EventPostToolUse, nil
	case "subagentstart":
		return reducer.EventSubagentStart, nil
	case "subagentstop":
		return reducer.EventSubagentStop, nil
	default:
		return "", fmt.Errorf("unsupported agentrace event %q", raw)
	}
}

func canonicalEventKey(raw string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(raw)))
}

func clonePayload(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
