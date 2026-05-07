// Package mem0 provides a read-hit-rate canary for the Mem0 OSS endpoint.
// The canary drives a configurable set of search queries through the OSS
// self-hosted API and reports the fraction that returned non-empty results.
package mem0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Canary drives search queries against a Mem0 OSS endpoint and tracks the
// fraction of queries that return at least one result (hit rate).
type Canary struct {
	OSSEndpoint string
	APIKey      string
	Queries     []string
	Timeout     time.Duration
}

// Result holds the outcome of a canary run.
type Result struct {
	Total    int
	Hits     int
	Errors   int
	Duration time.Duration
}

// HitRate returns the fraction of queries that returned results.
func (r Result) HitRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.Hits) / float64(r.Total)
}

// Run executes all configured queries against the OSS endpoint sequentially
// and returns the aggregate hit-rate result.
func (c *Canary) Run(ctx context.Context) (Result, error) {
	if len(c.Queries) == 0 {
		return Result{}, errors.New("canary: no queries configured")
	}

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	start := time.Now()
	var res Result
	res.Total = len(c.Queries)

	for _, q := range c.Queries {
		hit, err := c.search(ctx, client, q)
		if err != nil {
			res.Errors++
			continue
		}
		if hit {
			res.Hits++
		}
	}

	res.Duration = time.Since(start)
	return res, nil
}

type searchResponse struct {
	Results []json.RawMessage `json:"results"`
}

func (c *Canary) search(ctx context.Context, client *http.Client, query string) (bool, error) {
	u, err := url.Parse(c.OSSEndpoint)
	if err != nil {
		return false, fmt.Errorf("parse endpoint: %w", err)
	}
	u.Path = "/v1/memories/"
	params := u.Query()
	params.Set("query", query)
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("search %q: %w", query, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("search %q: status %d", query, resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return false, fmt.Errorf("decode %q: %w", query, err)
	}

	return len(sr.Results) > 0, nil
}
