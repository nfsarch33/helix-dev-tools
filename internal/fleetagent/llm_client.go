package fleetagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPLLMClient implements LLMClient via OpenAI-compatible HTTP API.
type HTTPLLMClient struct {
	BaseURL    string
	Model      string
	APIKey     string
	HTTPClient *http.Client
}

// NewHTTPLLMClient creates a client for the llm-cluster-router.
func NewHTTPLLMClient(baseURL, model, apiKey string) *HTTPLLMClient {
	return &HTTPLLMClient{
		BaseURL: baseURL,
		Model:   model,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *chatError   `json:"error,omitempty"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (c *HTTPLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	req := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	url := c.BaseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm api error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("llm error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// EngramReporter implements Reporter by posting to Engram memory.
type EngramReporter struct {
	BaseURL string
	UserID  string
	AppID   string
	APIKey  string
	Client  *http.Client
}

// NewEngramReporter creates a reporter that stores results in Engram.
func NewEngramReporter(baseURL, userID, appID, apiKey string) *EngramReporter {
	return &EngramReporter{
		BaseURL: baseURL,
		UserID:  userID,
		AppID:   appID,
		APIKey:  apiKey,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type engramAddRequest struct {
	Messages []engramMessage `json:"messages"`
	UserID   string          `json:"user_id"`
	AppID    string          `json:"app_id,omitempty"`
}

type engramMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (r *EngramReporter) Report(ctx context.Context, result ExecutionResult) error {
	content := fmt.Sprintf("Fleet agent completed ticket %s: success=%t duration=%s",
		result.TicketID, result.Success, result.Duration.Round(time.Second))
	if result.Error != "" {
		content += " error=" + result.Error
	}

	req := engramAddRequest{
		Messages: []engramMessage{{Role: "user", Content: content}},
		UserID:   r.UserID,
		AppID:    r.AppID,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := r.BaseURL + "/v1/memories/"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if r.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.APIKey)
	}

	resp, err := r.Client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("engram error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
