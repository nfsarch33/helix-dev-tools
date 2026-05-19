package embedhealth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProviderStatus struct {
	Name      string        `json:"name"`
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
	Dims      int           `json:"dims,omitempty"`
	CheckedAt time.Time     `json:"checked_at"`
}

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	ExpectDims int
	Timeout    time.Duration
}

func DefaultConfig() Config {
	return Config{
		Timeout:    10 * time.Second,
		ExpectDims: 1536,
		Model:      "text-embedding-3-small",
	}
}

type Checker struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Checker {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Checker{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Checker) Check(ctx context.Context) ProviderStatus {
	start := time.Now()
	status := ProviderStatus{
		Name:      c.cfg.Model,
		CheckedAt: start,
	}

	if c.cfg.BaseURL == "" {
		status.Error = "no base URL configured"
		return status
	}

	body := fmt.Sprintf(`{"input":"health check probe","model":"%s"}`, c.cfg.Model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.cfg.BaseURL, "/")+"/v1/embeddings",
		strings.NewReader(body))
	if err != nil {
		status.Error = err.Error()
		return status
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		status.Error = err.Error()
		status.Latency = time.Since(start)
		return status
	}
	defer resp.Body.Close()

	status.Latency = time.Since(start)

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		status.Error = fmt.Sprintf("status %d: %s", resp.StatusCode, string(respBody))
		return status
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		status.Error = "decode: " + err.Error()
		return status
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		status.Error = "empty embedding response"
		return status
	}

	status.Dims = len(result.Data[0].Embedding)
	if c.cfg.ExpectDims > 0 && status.Dims != c.cfg.ExpectDims {
		status.Error = fmt.Sprintf("dimension mismatch: got %d, want %d", status.Dims, c.cfg.ExpectDims)
		return status
	}

	status.Healthy = true
	return status
}

func (c *Checker) IsHealthy(ctx context.Context) bool {
	return c.Check(ctx).Healthy
}
