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

// GitLabClient provides access to the GitLab CI/CD API.
type GitLabClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewGitLabClient creates a client for the GitLab API.
func NewGitLabClient(baseURL, token string) *GitLabClient {
	return &GitLabClient{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Project represents a GitLab project.
type Project struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	WebURL   string `json:"web_url"`
	PathNS   string `json:"path_with_namespace"`
}

// Pipeline represents a GitLab CI pipeline run.
type Pipeline struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	WebURL    string `json:"web_url"`
	CreatedAt string `json:"created_at"`
}

// PipelineStatus constants.
const (
	StatusPending  = "pending"
	StatusRunning  = "running"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

func (c *GitLabClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	url := c.BaseURL + "/api/v4" + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read body: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

// GetProject fetches project by ID.
func (c *GitLabClient) GetProject(ctx context.Context, projectID int) (*Project, error) {
	body, status, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%d", projectID), nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("gitlab api %d: %s", status, string(body))
	}
	var p Project
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &p, nil
}

// TriggerPipeline triggers a new pipeline on the given ref.
func (c *GitLabClient) TriggerPipeline(ctx context.Context, projectID int, ref string) (*Pipeline, error) {
	payload := map[string]string{"ref": ref}
	body, status, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/projects/%d/pipeline", projectID), payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("trigger pipeline %d: %s", status, string(body))
	}
	var p Pipeline
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &p, nil
}

// GetPipelineStatus fetches the current status of a pipeline.
func (c *GitLabClient) GetPipelineStatus(ctx context.Context, projectID, pipelineID int) (*Pipeline, error) {
	body, status, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/projects/%d/pipelines/%d", projectID, pipelineID), nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("get pipeline %d: %s", status, string(body))
	}
	var p Pipeline
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &p, nil
}

// ListPipelines lists recent pipelines for a project.
func (c *GitLabClient) ListPipelines(ctx context.Context, projectID int, limit int) ([]Pipeline, error) {
	path := fmt.Sprintf("/projects/%d/pipelines?per_page=%d", projectID, limit)
	body, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("list pipelines %d: %s", status, string(body))
	}
	var pipelines []Pipeline
	if err := json.Unmarshal(body, &pipelines); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return pipelines, nil
}
