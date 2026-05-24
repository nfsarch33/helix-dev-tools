package helixone2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HelixonClient provides HTTP access to Helixon serve-mode API.
type HelixonClient struct {
	BaseURL string
	Client  *http.Client
}

// NewHelixonClient creates a client for the Helixon serve-mode endpoint.
func NewHelixonClient(baseURL string) *HelixonClient {
	return &HelixonClient{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// TaskRequest is the payload for submitting a task to Helixon.
type TaskRequest struct {
	Prompt string `json:"prompt"`
	UserID string `json:"user_id,omitempty"`
}

// TaskResponse is the Helixon response after processing a task.
type TaskResponse struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// HealthCheck verifies the Helixon service is running.
func (c *HelixonClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unhealthy: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

// SubmitTask sends a task prompt to Helixon and returns the response.
func (c *HelixonClient) SubmitTask(ctx context.Context, task TaskRequest) (*TaskResponse, error) {
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/task", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit task: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("task failed %d: %s", resp.StatusCode, string(respBody))
	}

	var taskResp TaskResponse
	if err := json.Unmarshal(respBody, &taskResp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &taskResp, nil
}

// EngramVerifier checks that a memory was stored in Engram.
type EngramVerifier struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// NewEngramVerifier creates a verifier for Engram memories.
func NewEngramVerifier(baseURL, apiKey string) *EngramVerifier {
	return &EngramVerifier{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// SearchMemory searches Engram for a query string.
func (v *EngramVerifier) SearchMemory(ctx context.Context, query, userID string) (bool, error) {
	payload := map[string]interface{}{
		"query":   query,
		"user_id": userID,
		"limit":   5,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.BaseURL+"/v1/memories/search/", bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if v.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+v.APIKey)
	}

	resp, err := v.Client.Do(req)
	if err != nil {
		return false, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("search error %d: %s", resp.StatusCode, string(respBody))
	}

	var results []interface{}
	if err := json.Unmarshal(respBody, &results); err != nil {
		return false, nil
	}
	return len(results) > 0, nil
}
