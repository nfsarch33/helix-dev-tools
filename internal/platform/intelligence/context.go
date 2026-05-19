package intelligence

import (
	"strings"
	"sync"
)

type WindowEntry struct {
	Role    string
	Content string
	Tokens  int
}

type ContextWindow struct {
	mu         sync.RWMutex
	entries    []WindowEntry
	maxTokens  int
	usedTokens int
}

func NewContextWindow(maxTokens int) *ContextWindow {
	return &ContextWindow{maxTokens: maxTokens}
}

func (w *ContextWindow) Add(role, content string) bool {
	tokens := EstimateTokens(content)
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.usedTokens+tokens > w.maxTokens {
		return false
	}
	w.entries = append(w.entries, WindowEntry{Role: role, Content: content, Tokens: tokens})
	w.usedTokens += tokens
	return true
}

func (w *ContextWindow) UsedTokens() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.usedTokens
}

func (w *ContextWindow) RemainingTokens() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.maxTokens - w.usedTokens
}

func (w *ContextWindow) Entries() []WindowEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]WindowEntry, len(w.entries))
	copy(result, w.entries)
	return result
}

func (w *ContextWindow) Truncate(keepLast int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if keepLast >= len(w.entries) {
		return
	}
	w.entries = w.entries[len(w.entries)-keepLast:]
	w.usedTokens = 0
	for _, e := range w.entries {
		w.usedTokens += e.Tokens
	}
}

func (w *ContextWindow) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = nil
	w.usedTokens = 0
}

func EstimateTokens(text string) int {
	words := len(strings.Fields(text))
	return int(float64(words) * 1.3)
}
