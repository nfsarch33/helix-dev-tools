package mcptrace

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TraceEvent is the NDJSON-serialized record of an MCP tool call.
type TraceEvent struct {
	EventType string                 `json:"event_type"`
	ToolName  string                 `json:"tool_name"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Duration  string                 `json:"duration"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Timestamp string                 `json:"ts"`
}

// Config controls the agentrace middleware behavior.
type Config struct {
	Enabled bool
	LogPath string
}

// ConfigFromEnv builds configuration from environment variables.
func ConfigFromEnv() Config {
	enabled := strings.EqualFold(os.Getenv("AGENTRACE_ENABLED"), "true")
	logPath := os.Getenv("AGENTRACE_LOG_PATH")
	if logPath == "" && enabled {
		home, _ := os.UserHomeDir()
		logPath = filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")
	}
	return Config{Enabled: enabled, LogPath: logPath}
}

// Middleware logs MCP tool calls to an NDJSON file for EvoSpine consumption.
type Middleware struct {
	config Config
	mu     sync.Mutex
	writer io.Writer
	file   *os.File
}

// NewMiddleware creates a middleware instance.
func NewMiddleware(cfg Config) *Middleware {
	return &Middleware{config: cfg}
}

// IsEnabled returns whether tracing is active.
func (m *Middleware) IsEnabled() bool {
	return m.config.Enabled
}

// SetWriter overrides the output destination (for testing).
func (m *Middleware) SetWriter(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writer = w
}

// Open initializes the file writer.
func (m *Middleware) Open() error {
	if !m.config.Enabled || m.config.LogPath == "" {
		return nil
	}
	dir := filepath.Dir(m.config.LogPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mcptrace: mkdir %s: %w", dir, err)
	}
	f, err := os.OpenFile(m.config.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("mcptrace: open %s: %w", m.config.LogPath, err)
	}
	m.mu.Lock()
	m.file = f
	m.writer = f
	m.mu.Unlock()
	return nil
}

// Close flushes and closes the file.
func (m *Middleware) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file != nil {
		m.file.Close()
		m.file = nil
	}
}

// RecordCall logs a single MCP tool invocation.
func (m *Middleware) RecordCall(toolName string, args map[string]interface{}, duration time.Duration, err error) {
	if !m.config.Enabled {
		return
	}
	m.mu.Lock()
	w := m.writer
	m.mu.Unlock()

	if w == nil {
		return
	}

	event := TraceEvent{
		EventType: "mcp_tool_call",
		ToolName:  toolName,
		Args:      sanitizeArgs(args),
		Duration:  duration.Round(time.Millisecond).String(),
		Success:   err == nil,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if err != nil {
		event.Error = err.Error()
	}

	data, _ := json.Marshal(event)
	data = append(data, '\n')

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writer != nil {
		_, _ = m.writer.Write(data)
	}
}

func sanitizeArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}
	safe := make(map[string]interface{}, len(args))
	for k, v := range args {
		if s, ok := v.(string); ok && len(s) > 200 {
			safe[k] = s[:200] + "...[truncated]"
		} else {
			safe[k] = v
		}
	}
	return safe
}
