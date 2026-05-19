package mem0search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	BaseURL string
	APIKey  string
	AppID   string
	UserID  string
	Timeout time.Duration
}

type SearchResult struct {
	ID       string          `json:"id"`
	Memory   string          `json:"memory"`
	Score    float64         `json:"score"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type HealthStatus struct {
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency_ms"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

func DefaultConfig() Config {
	baseURL := os.Getenv("MEM0_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8888"
	}
	return Config{
		BaseURL: baseURL,
		AppID:   "cursor-global-kb",
		UserID:  "nfsarch33",
		Timeout: 30 * time.Second,
	}
}

func CheckHealth(cfg Config) HealthStatus {
	start := time.Now()
	client := &http.Client{Timeout: cfg.Timeout}

	resp, err := client.Get(cfg.BaseURL + "/healthz")
	latency := time.Since(start)

	if err != nil {
		return HealthStatus{
			Healthy:   false,
			Latency:   latency,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}
	defer resp.Body.Close()

	return HealthStatus{
		Healthy:   resp.StatusCode == http.StatusOK,
		Latency:   latency,
		Timestamp: time.Now(),
	}
}

func Search(cfg Config, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	body := fmt.Sprintf(`{"query":%q,"user_id":%q,"app_id":%q,"limit":%d}`,
		query, cfg.UserID, cfg.AppID, limit)

	client := &http.Client{Timeout: cfg.Timeout}
	req, err := http.NewRequest("POST", cfg.BaseURL+"/search", strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return results, nil
}

func DiagnoseSearchPipeline(cfg Config) map[string]string {
	diagnosis := make(map[string]string)

	health := CheckHealth(cfg)
	if !health.Healthy {
		diagnosis["tunnel"] = "DOWN: " + health.Error
		return diagnosis
	}
	diagnosis["tunnel"] = fmt.Sprintf("UP (%dms)", health.Latency.Milliseconds())

	_, err := Search(cfg, "test", 1)
	if err != nil {
		if strings.Contains(err.Error(), "Upstream provider error") {
			diagnosis["embedding"] = "BROKEN: upstream provider error (bridge unreachable or timeout)"
		} else {
			diagnosis["search"] = "ERROR: " + err.Error()
		}
	} else {
		diagnosis["search"] = "OK"
	}

	return diagnosis
}
