package observe

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Span struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Operation string
	StartTime time.Time
	Duration  time.Duration
	Tags      map[string]string
}

type TraceContext struct {
	mu    sync.RWMutex
	spans []Span
}

func NewTraceContext() *TraceContext {
	return &TraceContext{}
}

func (tc *TraceContext) StartSpan(operation string, parentID string) Span {
	span := Span{
		TraceID:   generateID(),
		SpanID:    generateID(),
		ParentID:  parentID,
		Operation: operation,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
	}
	return span
}

func (tc *TraceContext) FinishSpan(span Span) {
	span.Duration = time.Since(span.StartTime)
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.spans = append(tc.spans, span)
}

func (tc *TraceContext) Spans() []Span {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	result := make([]Span, len(tc.spans))
	copy(result, tc.spans)
	return result
}

func (tc *TraceContext) SpanCount() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.spans)
}

func (tc *TraceContext) FindByOperation(op string) []Span {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	var found []Span
	for _, s := range tc.spans {
		if s.Operation == op {
			found = append(found, s)
		}
	}
	return found
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
