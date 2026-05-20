// Package search provides a token-indexed BM25 search over Agentrace
// NDJSON event files. The index is capped in memory and rebuilt from
// the tail of the events file on each query (suitable for <100k events).
package search

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// Event is a single Agentrace NDJSON line.
type Event struct {
	Timestamp string         `json:"ts"`
	EventType string         `json:"event"`
	Data      map[string]any `json:"-"`
	RawLine   string         `json:"-"`
	LineNum   int            `json:"-"`
}

// Result is a ranked search hit.
type Result struct {
	Event Event
	Score float64
}

// Search loads events from the NDJSON file, filters by time window,
// builds a BM25 index, and returns ranked results for the query.
func Search(path string, query string, since time.Duration, maxResults int) ([]Result, error) {
	events, err := loadEvents(path, since)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 || query == "" {
		return nil, nil
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil, nil
	}

	return bm25Rank(events, queryTokens, maxResults), nil
}

func loadEvents(path string, since time.Duration) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	cutoff := time.Now().Add(-since)
	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		ev := Event{
			Data:    raw,
			RawLine: line,
			LineNum: lineNum,
		}
		if ts, ok := raw["ts"].(string); ok {
			ev.Timestamp = ts
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				if t.Before(cutoff) {
					continue
				}
			}
		}
		if et, ok := raw["event"].(string); ok {
			ev.EventType = et
		}
		events = append(events, ev)
	}
	return events, scanner.Err()
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	words := strings.FieldsFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	unique := make(map[string]struct{})
	var result []string
	for _, w := range words {
		if len(w) < 2 {
			continue
		}
		if _, seen := unique[w]; !seen {
			unique[w] = struct{}{}
			result = append(result, w)
		}
	}
	return result
}

func eventTokens(ev Event) []string {
	var parts []string
	parts = append(parts, ev.EventType)
	for k, v := range ev.Data {
		parts = append(parts, k)
		if s, ok := v.(string); ok {
			parts = append(parts, s)
		}
	}
	return tokenize(strings.Join(parts, " "))
}

// bm25Rank implements Okapi BM25 ranking with k1=1.2, b=0.75.
func bm25Rank(events []Event, queryTokens []string, maxResults int) []Result {
	const k1 = 1.2
	const b = 0.75

	docs := make([][]string, len(events))
	totalLen := 0
	for i, ev := range events {
		docs[i] = eventTokens(ev)
		totalLen += len(docs[i])
	}
	avgDL := float64(totalLen) / float64(len(events))
	n := float64(len(events))

	df := map[string]int{}
	for _, doc := range docs {
		seen := map[string]struct{}{}
		for _, t := range doc {
			if _, ok := seen[t]; !ok {
				df[t]++
				seen[t] = struct{}{}
			}
		}
	}

	type scored struct {
		idx   int
		score float64
	}
	var results []scored

	for i, doc := range docs {
		tf := map[string]int{}
		for _, t := range doc {
			tf[t]++
		}

		score := 0.0
		dl := float64(len(doc))
		for _, qt := range queryTokens {
			d, ok := df[qt]
			if !ok {
				continue
			}
			idf := math.Log((n-float64(d)+0.5)/(float64(d)+0.5) + 1)
			termFreq := float64(tf[qt])
			score += idf * (termFreq * (k1 + 1)) / (termFreq + k1*(1-b+b*dl/avgDL))
		}
		if score > 0 {
			results = append(results, scored{idx: i, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	out := make([]Result, len(results))
	for i, r := range results {
		out[i] = Result{Event: events[r.idx], Score: r.score}
	}
	return out
}
