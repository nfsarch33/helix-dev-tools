package coordination

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client interacts with the Mem0 API for coordination signals.
type Client struct {
	APIKey  string
	UserID  string
	BaseURL string
	HTTP    *http.Client
}

// NewClient creates a coordination client. If baseURL is empty, defaults to
// the Mem0 managed API.
func NewClient(apiKey, userID, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.mem0.ai"
	}
	return &Client{
		APIKey:  apiKey,
		UserID:  userID,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

type mem0AddPayload struct {
	UserID   string            `json:"user_id"`
	AppID    string            `json:"app_id"`
	Text     string            `json:"text"`
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
	payload := mem0AddPayload{
		UserID:   c.UserID,
		AppID:    AppID,
		Text:     s.Mem0Text(),
		Metadata: s.Mem0Metadata(),
		Infer:    false,
	}
	return c.doJSON(ctx, http.MethodPost, "/v1/memories/", payload)
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
		return nil, err
	}
	return parseSearchResults(body)
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
			return nil, err
		}

		signals, total, err := parseListResults(body)
		if err != nil {
			return nil, err
		}
		all = append(all, signals...)
		if len(signals) == 0 || len(all) >= total {
			break
		}
	}
	return all, nil
}

// DeleteSignal removes a single coordination signal by Mem0 memory ID.
func (c *Client) DeleteSignal(ctx context.Context, memoryID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/v1/memories/"+memoryID+"/", nil)
}

// CleanStaleSignals deletes signals older than maxAge. Returns the count deleted.
func (c *Client) CleanStaleSignals(ctx context.Context, maxAge time.Duration) (int, error) {
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

func (c *Client) doJSON(ctx context.Context, method, path string, payload interface{}) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encoding payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Client) doJSONResponse(ctx context.Context, method, path string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(data))
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
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
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
