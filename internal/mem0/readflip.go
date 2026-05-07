package mem0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// SearchResult is a single item returned by a Searcher.
type SearchResult struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

// Searcher abstracts a Mem0 search endpoint. Both OSS and managed
// backends implement this interface.
type Searcher interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

// ReadFlipReport summarises one read-flip canary run.
type ReadFlipReport struct {
	TotalQueries int
	FlippedToOSS int
	AvgOverlap   float64
	Duration     time.Duration
}

// OverlapLogEntry is a single NDJSON line written per query.
type OverlapLogEntry struct {
	Timestamp    time.Time `json:"ts"`
	Query        string    `json:"query"`
	Overlap      float64   `json:"overlap"`
	OSSTop3      []string  `json:"oss_top3"`
	ManagedTop3  []string  `json:"managed_top3"`
	FlippedToOSS bool      `json:"flipped"`
}

// ReadFlip routes a configurable percentage of reads to the OSS
// backend first while keeping managed as a fallback. For each flipped
// query it records the top-3 ID overlap between OSS and managed in an
// NDJSON log file.
type ReadFlip struct {
	OSS     Searcher
	Managed Searcher

	// FlipPct is 0-100: the percentage of queries sent to OSS first.
	FlipPct int

	LogPath string
	Queries []string
	Timeout time.Duration
}

// Run executes the read-flip canary across all configured queries.
func (rf *ReadFlip) Run(ctx context.Context) (ReadFlipReport, error) {
	if len(rf.Queries) == 0 {
		return ReadFlipReport{}, errors.New("readflip: no queries configured")
	}

	if err := os.MkdirAll(filepath.Dir(rf.LogPath), 0o755); err != nil {
		return ReadFlipReport{}, err
	}
	logFile, err := os.OpenFile(rf.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return ReadFlipReport{}, err
	}
	defer logFile.Close()

	start := time.Now()
	var report ReadFlipReport
	report.TotalQueries = len(rf.Queries)

	var totalOverlap float64
	var overlapCount int

	for _, q := range rf.Queries {
		flipped := rf.shouldFlip()
		entry := OverlapLogEntry{
			Timestamp:    time.Now().UTC(),
			Query:        q,
			FlippedToOSS: flipped,
		}

		if !flipped {
			line, _ := json.Marshal(entry)
			_, _ = logFile.Write(append(line, '\n'))
			continue
		}

		report.FlippedToOSS++

		ossResults, ossErr := rf.OSS.Search(ctx, q)
		managedResults, managedErr := rf.Managed.Search(ctx, q)

		if ossErr != nil || managedErr != nil {
			line, _ := json.Marshal(entry)
			_, _ = logFile.Write(append(line, '\n'))
			continue
		}

		ossTop3 := topNIDs(ossResults, 3)
		managedTop3 := topNIDs(managedResults, 3)
		overlap := computeOverlap(ossTop3, managedTop3)

		entry.Overlap = overlap
		entry.OSSTop3 = ossTop3
		entry.ManagedTop3 = managedTop3

		totalOverlap += overlap
		overlapCount++

		line, _ := json.Marshal(entry)
		_, _ = logFile.Write(append(line, '\n'))
	}

	if overlapCount > 0 {
		report.AvgOverlap = totalOverlap / float64(overlapCount)
	}
	report.Duration = time.Since(start)
	return report, nil
}

func (rf *ReadFlip) shouldFlip() bool {
	if rf.FlipPct <= 0 {
		return false
	}
	if rf.FlipPct >= 100 {
		return true
	}
	return rand.IntN(100) < rf.FlipPct
}

func topNIDs(results []SearchResult, n int) []string {
	ids := make([]string, 0, n)
	for i := 0; i < n && i < len(results); i++ {
		ids = append(ids, results[i].ID)
	}
	return ids
}

func computeOverlap(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	denom := max(len(a), len(b))
	if denom == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(b))
	for _, id := range b {
		set[id] = struct{}{}
	}
	var count int
	for _, id := range a {
		if _, ok := set[id]; ok {
			count++
		}
	}
	return float64(count) / float64(denom)
}

// HTTPSearcher implements Searcher against a Mem0 v1 /memories/ GET
// endpoint. Used by the CLI canary to hit both OSS and managed.
type HTTPSearcher struct {
	Endpoint string
	APIKey   string
	Timeout  time.Duration
}

// Search queries the Mem0 search endpoint and returns results.
func (s *HTTPSearcher) Search(ctx context.Context, query string) ([]SearchResult, error) {
	u, err := url.Parse(s.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	u.Path = "/v1/memories/"
	params := u.Query()
	params.Set("query", query)
	u.RawQuery = params.Encode()

	client := &http.Client{Timeout: s.Timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if s.APIKey != "" {
		req.Header.Set("X-API-Key", s.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search %q: status %d", query, resp.StatusCode)
	}

	var sr struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	return sr.Results, nil
}
