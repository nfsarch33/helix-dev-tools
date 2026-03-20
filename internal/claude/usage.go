package claude

import "time"

// Usage captures resource consumption from a single Claude CLI invocation.
type Usage struct {
	Timestamp    time.Time `json:"ts"`
	Model        string    `json:"model,omitempty"`
	PromptBytes  int       `json:"prompt_bytes"`
	OutputBytes  int       `json:"output_bytes"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	CacheRead    int       `json:"cache_read,omitempty"`
	CacheWrite   int       `json:"cache_write,omitempty"`
	Cost         float64   `json:"cost,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
	ExitCode     int       `json:"exit_code"`
	Backend      string    `json:"backend"`
	Prompt       string    `json:"prompt,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// HasTokenCounts returns true when the backend reported actual token metrics.
func (u Usage) HasTokenCounts() bool {
	return u.InputTokens > 0 || u.OutputTokens > 0
}
