package sprintboard

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"
)

// SearchResult represents a ranked search hit.
type SearchResult struct {
	ID    string
	Score float64
}

// TFIDFIndex provides term-frequency inverse-document-frequency search
// over a small corpus of ticket/sprint text. Pure Go, no external deps.
// Suitable for hundreds of documents (our operational scale).
type TFIDFIndex struct {
	docs      map[string]map[string]float64 // docID -> term -> TF
	idf       map[string]float64            // term -> IDF
	docCount  int
	termDocs  map[string]int // term -> number of docs containing it
}

// NewTFIDFIndex creates an empty index.
func NewTFIDFIndex() *TFIDFIndex {
	return &TFIDFIndex{
		docs:     make(map[string]map[string]float64),
		idf:      make(map[string]float64),
		termDocs: make(map[string]int),
	}
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	for _, word := range strings.Fields(text) {
		word = strings.Trim(word, ".,;:!?()[]{}\"'`")
		if len(word) > 1 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

func termFrequency(tokens []string) map[string]float64 {
	tf := make(map[string]float64)
	for _, t := range tokens {
		tf[t]++
	}
	total := float64(len(tokens))
	for k := range tf {
		tf[k] /= total
	}
	return tf
}

// AddDocument indexes (or re-indexes) a document by ID.
func (idx *TFIDFIndex) AddDocument(id, text string) {
	if _, exists := idx.docs[id]; exists {
		idx.RemoveDocument(id)
	}

	tokens := tokenize(text)
	if len(tokens) == 0 {
		return
	}

	tf := termFrequency(tokens)
	idx.docs[id] = tf
	idx.docCount++

	seen := make(map[string]bool)
	for _, t := range tokens {
		if !seen[t] {
			idx.termDocs[t]++
			seen[t] = true
		}
	}
	idx.rebuildIDF()
}

// RemoveDocument removes a document from the index.
func (idx *TFIDFIndex) RemoveDocument(id string) {
	tf, exists := idx.docs[id]
	if !exists {
		return
	}

	for term := range tf {
		idx.termDocs[term]--
		if idx.termDocs[term] <= 0 {
			delete(idx.termDocs, term)
		}
	}
	delete(idx.docs, id)
	idx.docCount--
	idx.rebuildIDF()
}

func (idx *TFIDFIndex) rebuildIDF() {
	idx.idf = make(map[string]float64, len(idx.termDocs))
	if idx.docCount == 0 {
		return
	}
	for term, count := range idx.termDocs {
		idx.idf[term] = math.Log(1.0+float64(idx.docCount)/float64(count)) + 1.0
	}
}

// Search finds the top-k documents most relevant to the query.
func (idx *TFIDFIndex) Search(query string, limit int) []SearchResult {
	if idx.docCount == 0 {
		return nil
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}
	queryTF := termFrequency(queryTokens)

	queryVec := make(map[string]float64)
	for term, tf := range queryTF {
		if idf, ok := idx.idf[term]; ok {
			queryVec[term] = tf * idf
		}
	}

	var results []SearchResult
	for docID, docTF := range idx.docs {
		score := cosineSimilarity(queryVec, docTF, idx.idf)
		if score > 0 {
			results = append(results, SearchResult{ID: docID, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func cosineSimilarity(queryVec, docTF map[string]float64, idf map[string]float64) float64 {
	var dot, normQ, normD float64
	for term, qWeight := range queryVec {
		dTF := docTF[term]
		dWeight := dTF * idf[term]
		dot += qWeight * dWeight
		normQ += qWeight * qWeight
		normD += dWeight * dWeight
	}
	for term, dTF := range docTF {
		if _, inQuery := queryVec[term]; !inQuery {
			dWeight := dTF * idf[term]
			normD += dWeight * dWeight
		}
	}
	if normQ == 0 || normD == 0 {
		return 0
	}
	return dot / (math.Sqrt(normQ) * math.Sqrt(normD))
}

// AgentraceEvent represents an MCP tool call event for the agentrace pipeline.
type AgentraceEvent struct {
	EventType string            `json:"event_type"`
	ToolName  string            `json:"tool_name"`
	Args      map[string]string `json:"args"`
	Timestamp string            `json:"ts"`
}

// NewAgentraceEvent creates a tool call event.
func NewAgentraceEvent(toolName string, args map[string]string) AgentraceEvent {
	return AgentraceEvent{
		EventType: "mcp_tool_call",
		ToolName:  toolName,
		Args:      args,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// ToNDJSON serializes the event as a newline-delimited JSON line.
func (e AgentraceEvent) ToNDJSON() string {
	data, _ := json.Marshal(e)
	return string(data) + "\n"
}
