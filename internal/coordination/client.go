package coordination

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// ClientMetrics holds atomic counters for coordination operations.
type ClientMetrics struct {
	SignalsAdded    atomic.Int64
	SignalsListed   atomic.Int64
	SignalsDeleted  atomic.Int64
	SignalsSearched atomic.Int64
	CleanupRuns     atomic.Int64
	CleanupDeleted  atomic.Int64
	Errors          atomic.Int64
	Retries         atomic.Int64
}

// Snapshot returns a point-in-time copy of all counters.
func (m *ClientMetrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"ironclaw_coordination_signals_added_total":    m.SignalsAdded.Load(),
		"ironclaw_coordination_signals_listed_total":   m.SignalsListed.Load(),
		"ironclaw_coordination_signals_deleted_total":  m.SignalsDeleted.Load(),
		"ironclaw_coordination_signals_searched_total": m.SignalsSearched.Load(),
		"ironclaw_coordination_cleanup_runs_total":     m.CleanupRuns.Load(),
		"ironclaw_coordination_cleanup_deleted_total":  m.CleanupDeleted.Load(),
		"ironclaw_coordination_errors_total":           m.Errors.Load(),
		"ironclaw_coordination_retries_total":          m.Retries.Load(),
	}
}

// Client interacts with the Mem0 API for coordination signals.
type Client struct {
	APIKey         string
	UserID         string
	BaseURL        string
	HTTP           *http.Client
	Log            *slog.Logger
	Stats          *ClientMetrics
	MaxRetries     int
	RetryBaseDelay time.Duration
}

// NewClient creates a coordination client. If baseURL is empty, defaults to
// the Mem0 managed API.
func NewClient(apiKey, userID, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.mem0.ai"
	}
	return &Client{
		APIKey:         apiKey,
		UserID:         userID,
		BaseURL:        strings.TrimRight(baseURL, "/"),
		HTTP:           &http.Client{Timeout: 30 * time.Second},
		Log:            slog.Default(),
		Stats:          &ClientMetrics{},
		MaxRetries:     3,
		RetryBaseDelay: 500 * time.Millisecond,
	}
}

// WithLogger returns the client with the given slog logger.
func (c *Client) WithLogger(l *slog.Logger) *Client {
	c.Log = l
	return c
}

type mem0Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mem0AddPayload struct {
	UserID   string            `json:"user_id"`
	AppID    string            `json:"app_id"`
	Text     string            `json:"text"`
	Messages []mem0Message     `json:"messages"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Infer    bool              `json:"infer"`
}

type mem0SearchPayload struct {
	Query   string      `json:"query"`
	Filters interface{} `json:"filters,omitempty"`
	Limit   int         `json:"limit,omitempty"`
}

type mem0ListPayload struct {
	Filters interface{} `json:"filters,omitempty"`
	Page    int         `json:"page"`
	Size    int         `json:"page_size"`
}

type mem0SearchResult struct {
	ID       string            `json:"id"`
	Memory   string            `json:"memory"`
	Score    float64           `json:"score"`
	Metadata map[string]string `json:"metadata"`
	UserID   string            `json:"user_id"`
	AppID    string            `json:"app_id"`
}

type mem0ListResult struct {
	Results []mem0SearchResult `json:"results"`
	Total   int                `json:"total"`
}

// AddSignal writes a coordination signal to Mem0.
func (c *Client) AddSignal(ctx context.Context, s Signal) error {
	text := s.Mem0Text()
	payload := mem0AddPayload{
		UserID:   c.UserID,
		AppID:    AppID,
		Text:     text,
		Messages: []mem0Message{{Role: "user", Content: text}},
		Metadata: s.Mem0Metadata(),
		Infer:    false,
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/memories/", payload); err != nil {
		c.Stats.Errors.Add(1)
		c.Log.Warn("coordination signal add failed", "type", s.Type, "machine", s.Machine, "error", err)
		return err
	}
	c.Stats.SignalsAdded.Add(1)
	c.Log.Info("coordination signal added", "type", s.Type, "machine", s.Machine, "target_for", s.TargetFor)
	return nil
}

// SearchSignals queries Mem0 for coordination signals matching the query.
func (c *Client) SearchSignals(ctx context.Context, query string, limit int) ([]Signal, error) {
	if limit <= 0 {
		limit = 20
	}
	payload := mem0SearchPayload{
		Query: query,
		Filters: map[string]interface{}{
			"AND": []map[string]string{
				{"app_id": AppID},
				{"user_id": c.UserID},
			},
		},
		Limit: limit,
	}

	body, err := c.doJSONResponse(ctx, http.MethodPost, "/v1/memories/search/", payload)
	if err != nil {
		c.Stats.Errors.Add(1)
		c.Log.Warn("coordination signal search failed", "query", query, "error", err)
		return nil, err
	}
	signals, err := parseSearchResults(body)
	if err != nil {
		c.Stats.Errors.Add(1)
		return nil, err
	}
	c.Stats.SignalsSearched.Add(1)
	c.Log.Debug("coordination signals searched", "query", query, "results", len(signals))
	return signals, nil
}

// ListSignals retrieves all coordination signals (paginated).
func (c *Client) ListSignals(ctx context.Context) ([]Signal, error) {
	var all []Signal
	for page := 1; ; page++ {
		payload := mem0ListPayload{
			Filters: map[string]interface{}{
				"AND": []map[string]string{
					{"app_id": AppID},
					{"user_id": c.UserID},
				},
			},
			Page: page,
			Size: 100,
		}

		body, err := c.doJSONResponse(ctx, http.MethodPost, "/v2/memories/", payload)
		if err != nil {
			c.Stats.Errors.Add(1)
			c.Log.Warn("coordination signal list failed", "page", page, "error", err)
			return nil, err
		}

		signals, total, err := parseListResults(body)
		if err != nil {
			c.Stats.Errors.Add(1)
			return nil, err
		}
		all = append(all, signals...)
		if len(signals) == 0 || len(all) >= total {
			break
		}
	}
	c.Stats.SignalsListed.Add(1)
	c.Log.Debug("coordination signals listed", "count", len(all))
	return all, nil
}

// DeleteSignal removes a single coordination signal by Mem0 memory ID.
func (c *Client) DeleteSignal(ctx context.Context, memoryID string) error {
	if err := c.doJSON(ctx, http.MethodDelete, "/v1/memories/"+memoryID+"/", nil); err != nil {
		c.Stats.Errors.Add(1)
		c.Log.Warn("coordination signal delete failed", "memory_id", memoryID, "error", err)
		return err
	}
	c.Stats.SignalsDeleted.Add(1)
	c.Log.Info("coordination signal deleted", "memory_id", memoryID)
	return nil
}

// CleanStaleSignals deletes signals older than maxAge. Returns the count deleted.
func (c *Client) CleanStaleSignals(ctx context.Context, maxAge time.Duration) (int, error) {
	c.Stats.CleanupRuns.Add(1)
	signals, err := c.ListSignals(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing signals for cleanup: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0
	for _, s := range signals {
		if s.ID == "" {
			continue
		}
		if s.Type == SignalCompleted || (!s.CreatedAt.IsZero() && s.CreatedAt.Before(cutoff)) {
			if err := c.DeleteSignal(ctx, s.ID); err != nil {
				return deleted, fmt.Errorf("deleting signal %s: %w", s.ID, err)
			}
			deleted++
		}
	}
	c.Stats.CleanupDeleted.Add(int64(deleted))
	c.Log.Info("coordination cleanup complete", "deleted", deleted, "total_signals", len(signals), "max_age", maxAge.String())
	return deleted, nil
}

// FilterForMachine returns only signals targeted at the given machine.
func FilterForMachine(signals []Signal, machine string) []Signal {
	var filtered []Signal
	for _, s := range signals {
		if s.TargetFor == "" || strings.EqualFold(s.TargetFor, machine) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// FilterPendingTasks returns task-dispatch signals targeting a specific machine.
func FilterPendingTasks(signals []Signal, machine string) []Signal {
	var tasks []Signal
	for _, s := range signals {
		if s.Type == SignalTaskDispatch && strings.EqualFold(s.TargetFor, machine) {
			tasks = append(tasks, s)
		}
	}
	return tasks
}

// isRetryable returns true for status codes that indicate a transient server
// error worth retrying (5xx and 429 Too Many Requests). 4xx errors (except 429)
// are client errors and must NOT be retried.
func isRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

// doHTTP executes an HTTP request with exponential backoff retry for transient
// errors. It returns the response body on success or the last error after all
// retries are exhausted.
func (c *Client) doHTTP(ctx context.Context, method, path string, payload []byte) ([]byte, error) {
	maxAttempts := 1 + c.MaxRetries
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := c.RetryBaseDelay * (1 << (attempt - 1))
			jitter := time.Duration(float64(delay) * 0.1 * rand.Float64())
			delay += jitter
			c.Stats.Retries.Add(1)
			c.Log.Warn("retrying", "attempt", attempt, "status", lastErr, "delay", delay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var bodyReader io.Reader
		if payload != nil {
			bodyReader = bytes.NewReader(payload)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("Authorization", "Token "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("sending request: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 300 {
			return body, nil
		}

		lastErr = fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		if !isRetryable(resp.StatusCode) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload interface{}) error {
	var data []byte
	if payload != nil {
		var err error
		data, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encoding payload: %w", err)
		}
	}
	_, err := c.doHTTP(ctx, method, path, data)
	return err
}

func (c *Client) doJSONResponse(ctx context.Context, method, path string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding payload: %w", err)
	}
	return c.doHTTP(ctx, method, path, data)
}

func parseSearchResults(data []byte) ([]Signal, error) {
	var results []mem0SearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}
	signals := make([]Signal, 0, len(results))
	for _, r := range results {
		signals = append(signals, signalFromMem0(r))
	}
	return signals, nil
}

func parseListResults(data []byte) ([]Signal, int, error) {
	var wrapped mem0ListResult
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Results) > 0 {
		signals := make([]Signal, 0, len(wrapped.Results))
		for _, r := range wrapped.Results {
			signals = append(signals, signalFromMem0(r))
		}
		return signals, wrapped.Total, nil
	}

	var flat []mem0SearchResult
	if err := json.Unmarshal(data, &flat); err == nil {
		signals := make([]Signal, 0, len(flat))
		for _, r := range flat {
			signals = append(signals, signalFromMem0(r))
		}
		return signals, len(flat), nil
	}
	return nil, 0, fmt.Errorf("parsing list results")
}

func signalFromMem0(r mem0SearchResult) Signal {
	s := Signal{
		ID:      r.ID,
		Message: r.Memory,
		Machine: metaValue(r.Metadata, "machine"),
	}
	if t := metaValue(r.Metadata, "type"); t != "" {
		s.Type = SignalType(t)
	}
	s.TargetFor = metaValue(r.Metadata, "target_for")
	s.Priority = metaValue(r.Metadata, "priority")
	s.Sprint = metaValue(r.Metadata, "sprint")
	return s
}

func metaValue(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}

// ResolveCredentials reads Mem0 API key and user ID from env or MCP config.
func ResolveCredentials(mcpConfigPath string) (apiKey, userID string, err error) {
	apiKey = strings.TrimSpace(os.Getenv("MEM0_API_KEY"))
	userID = strings.TrimSpace(os.Getenv("MEM0_DEFAULT_USER_ID"))
	if apiKey != "" && userID != "" {
		return apiKey, userID, nil
	}

	data, readErr := os.ReadFile(mcpConfigPath)
	if readErr != nil {
		return "", "", fmt.Errorf("reading mcp config: %w", readErr)
	}

	var cfg struct {
		MCPServers map[string]struct {
			Env map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", "", fmt.Errorf("parsing mcp config: %w", err)
	}

	mem0, ok := cfg.MCPServers["mem0"]
	if !ok {
		return "", "", fmt.Errorf("mem0 server not found in mcp config")
	}
	if apiKey == "" {
		apiKey = resolveEnvValue(mem0.Env["MEM0_API_KEY"])
	}
	if userID == "" {
		userID = resolveEnvValue(mem0.Env["MEM0_DEFAULT_USER_ID"])
	}
	if apiKey == "" {
		return "", "", fmt.Errorf("MEM0_API_KEY not found")
	}
	if userID == "" {
		return "", "", fmt.Errorf("MEM0_DEFAULT_USER_ID not found")
	}
	return apiKey, userID, nil
}

func resolveEnvValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "$") && len(raw) > 1 {
		return strings.TrimSpace(os.Getenv(strings.TrimPrefix(raw, "$")))
	}
	return raw
}
