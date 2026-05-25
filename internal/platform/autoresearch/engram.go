package autoresearch

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

const (
	engramAppID      = "autoresearch"
	defaultEngramURL = "http://localhost:8281"
	engramTimeout    = 15 * time.Second
)

// EngramClient stores and retrieves research findings from Engram's
// mem0-compat API. All memories use app_id "autoresearch".
type EngramClient struct {
	BaseURL string
	UserID  string
	APIKey  string
	HTTP    *http.Client
}

// EngramMemory is a single memory record returned by Engram search/list.
type EngramMemory struct {
	ID       string                 `json:"id"`
	Memory   string                 `json:"memory"`
	Score    float64                `json:"score,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewEngramClient creates a client for research memory persistence.
// It reads ENGRAM_URL, ENGRAM_API_KEY, and ENGRAM_USER_ID from the
// environment, falling back to defaults suitable for the local tunnel.
func NewEngramClient() *EngramClient {
	baseURL := strings.TrimSpace(os.Getenv("ENGRAM_URL"))
	if baseURL == "" {
		baseURL = defaultEngramURL
	}
	return &EngramClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		UserID:  envOrDefault("ENGRAM_USER_ID", "autoresearch-agent"),
		APIKey:  strings.TrimSpace(os.Getenv("ENGRAM_API_KEY")),
		HTTP:    &http.Client{Timeout: engramTimeout},
	}
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v != "" {
		return v
	}
	return fallback
}

type engramAddPayload struct {
	UserID   string            `json:"user_id"`
	AppID    string            `json:"app_id"`
	Text     string            `json:"text"`
	Messages []engramMsg       `json:"messages"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Infer    bool              `json:"infer"`
}

type engramMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type engramSearchPayload struct {
	Query   string      `json:"query"`
	Filters interface{} `json:"filters,omitempty"`
	Limit   int         `json:"limit,omitempty"`
}

// StoreResearch persists a research finding to Engram.
func (c *EngramClient) StoreResearch(ctx context.Context, text string, meta map[string]string) error {
	payload := engramAddPayload{
		UserID:   c.UserID,
		AppID:    engramAppID,
		Text:     text,
		Messages: []engramMsg{{Role: "user", Content: text}},
		Metadata: meta,
		Infer:    false,
	}
	_, err := c.doJSON(ctx, http.MethodPost, "/v1/memories/", payload)
	return err
}

// SearchResearch queries Engram for past research findings matching the query.
func (c *EngramClient) SearchResearch(ctx context.Context, query string, limit int) ([]EngramMemory, error) {
	if limit <= 0 {
		limit = 10
	}
	payload := engramSearchPayload{
		Query: query,
		Filters: map[string]interface{}{
			"AND": []map[string]string{
				{"app_id": engramAppID},
				{"user_id": c.UserID},
			},
		},
		Limit: limit,
	}
	body, err := c.doJSON(ctx, http.MethodPost, "/v1/memories/search/", payload)
	if err != nil {
		return nil, err
	}
	var results []EngramMemory
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("parse search results: %w", err)
	}
	return results, nil
}

func (c *EngramClient) doJSON(ctx context.Context, method, path string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Token "+c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("engram request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("engram %s %s: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}
