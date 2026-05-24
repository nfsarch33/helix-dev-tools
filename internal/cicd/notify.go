package cicd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Notifier sends pipeline failure alerts to Engram memory.
type Notifier struct {
	EngramURL string
	UserID    string
	AppID     string
	APIKey    string
	Client    *http.Client
}

// NewNotifier creates a pipeline failure notifier.
func NewNotifier(engramURL, userID, appID, apiKey string) *Notifier {
	return &Notifier{
		EngramURL: engramURL,
		UserID:    userID,
		AppID:     appID,
		APIKey:    apiKey,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NotifyFailure stores a pipeline failure event in Engram.
func (n *Notifier) NotifyFailure(ctx context.Context, project string, pipeline Pipeline) error {
	content := fmt.Sprintf("CI FAILURE: project=%s pipeline=#%d ref=%s status=%s url=%s",
		project, pipeline.ID, pipeline.Ref, pipeline.Status, pipeline.WebURL)

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
		"user_id": n.UserID,
		"app_id":  n.AppID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := n.EngramURL + "/v1/memories/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if n.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+n.APIKey)
	}

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("engram %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// NotifySuccess stores a pipeline success event (optional, for tracking).
func (n *Notifier) NotifySuccess(ctx context.Context, project string, pipeline Pipeline) error {
	content := fmt.Sprintf("CI PASS: project=%s pipeline=#%d ref=%s",
		project, pipeline.ID, pipeline.Ref)

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
		"user_id": n.UserID,
		"app_id":  n.AppID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := n.EngramURL + "/v1/memories/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if n.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+n.APIKey)
	}

	resp, err := n.Client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}
