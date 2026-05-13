// Package agentrace provides in-memory full-text search over NDJSON log streams.
//
// BuildIndex reads all .ndjson files from a directory and builds a
// case-insensitive term-frequency index. Search returns entries ranked
// by TF-IDF score. This avoids a CGO/SQLite dependency while providing
// useful relevance ranking for agentrace queries.
package agentrace

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SearchResult is a single matching NDJSON entry.
type SearchResult struct {
	File  string  `json:"file"`
	Line  string  `json:"line"`
	Score float64 `json:"score"`
}

// Index holds parsed NDJSON entries and an inverted term index.
type Index struct {
	entries []indexEntry
	idf     map[string]float64
}

type indexEntry struct {
	file     string
	raw      string
	termFreq map[string]int
}

// BuildIndex reads all .ndjson files in dir and builds a searchable index.
func BuildIndex(dir string) (*Index, error) {
	pattern := filepath.Join(dir, "*.ndjson")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	idx := &Index{}
	docFreq := make(map[string]int)

	for _, f := range files {
		name := filepath.Base(f)
		data, readErr := os.ReadFile(f)
		if readErr != nil {
			slog.Warn("agentrace: skip file", "file", f, "error", readErr)
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if !json.Valid([]byte(line)) {
				continue
			}

			lower := strings.ToLower(line)
			tf := tokenize(lower)

			entry := indexEntry{
				file:     name,
				raw:      line,
				termFreq: tf,
			}
			idx.entries = append(idx.entries, entry)

			seen := make(map[string]bool)
			for term := range tf {
				if !seen[term] {
					docFreq[term]++
					seen[term] = true
				}
			}
		}
	}

	idx.idf = make(map[string]float64)
	n := float64(len(idx.entries))
	if n > 0 {
		for term, df := range docFreq {
			idx.idf[term] = math.Log(1 + n/float64(df))
		}
	}

	return idx, nil
}

// Search returns the top-k entries matching the query, ranked by TF-IDF score.
func (idx *Index) Search(query string, limit int) []SearchResult {
	if len(idx.entries) == 0 || query == "" {
		return nil
	}

	queryTerms := tokenize(strings.ToLower(query))
	if len(queryTerms) == 0 {
		return nil
	}

	type scored struct {
		idx   int
		score float64
	}
	var candidates []scored

	for i, entry := range idx.entries {
		score := 0.0
		for term := range queryTerms {
			tf, ok := entry.termFreq[term]
			if !ok {
				continue
			}
			idf := idx.idf[term]
			if idf == 0 {
				idf = 1
			}
			score += float64(tf) * idf
		}
		if score > 0 {
			candidates = append(candidates, scored{idx: i, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]SearchResult, len(candidates))
	for i, c := range candidates {
		e := idx.entries[c.idx]
		results[i] = SearchResult{
			File:  e.file,
			Line:  e.raw,
			Score: c.score,
		}
	}
	return results
}

func tokenize(s string) map[string]int {
	freq := make(map[string]int)
	for _, word := range strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_')
	}) {
		if len(word) >= 2 {
			freq[word]++
		}
	}
	return freq
}
