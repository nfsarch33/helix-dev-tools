package memoryinspector

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type MemoryCategory string

const (
	CatOperational  MemoryCategory = "operational"
	CatPattern      MemoryCategory = "pattern"
	CatDecision     MemoryCategory = "decision"
	CatCoordination MemoryCategory = "coordination"
	CatHandoff      MemoryCategory = "handoff"
)

type MemoryEntry struct {
	ID        string         `json:"id"`
	AppID     string         `json:"app_id,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Content   string         `json:"content"`
	Category  MemoryCategory `json:"category,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

type MemoryStats struct {
	Total      int                       `json:"total"`
	ByApp      map[string]int            `json:"by_app"`
	ByCategory map[MemoryCategory]int    `json:"by_category"`
	OldestAt   time.Time                 `json:"oldest_at,omitempty"`
	NewestAt   time.Time                 `json:"newest_at,omitempty"`
}

type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]MemoryEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]MemoryEntry),
	}
}

func (s *MemoryStore) Add(entry MemoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[entry.ID]; exists {
		return fmt.Errorf("memory entry %q already exists", entry.ID)
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	s.entries[entry.ID] = entry
	return nil
}

func (s *MemoryStore) Get(id string) (MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[id]
	if !exists {
		return MemoryEntry{}, fmt.Errorf("memory entry %q not found", id)
	}
	return entry, nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[id]; !exists {
		return fmt.Errorf("memory entry %q not found", id)
	}

	delete(s.entries, id)
	return nil
}

func (s *MemoryStore) SearchByAppID(appID string) []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MemoryEntry
	for _, e := range s.entries {
		if e.AppID == appID {
			result = append(result, e)
		}
	}
	return result
}

func (s *MemoryStore) SearchByContent(query string) []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var result []MemoryEntry
	for _, e := range s.entries {
		if strings.Contains(strings.ToLower(e.Content), q) {
			result = append(result, e)
		}
	}
	return result
}

func (s *MemoryStore) ListByCategory(cat MemoryCategory) []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MemoryEntry
	for _, e := range s.entries {
		if e.Category == cat {
			result = append(result, e)
		}
	}
	return result
}

func (s *MemoryStore) Recent(window time.Duration) []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var result []MemoryEntry
	for _, e := range s.entries {
		if e.CreatedAt.After(cutoff) {
			result = append(result, e)
		}
	}
	return result
}

func (s *MemoryStore) All() []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]MemoryEntry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	return result
}

func (s *MemoryStore) Stats() MemoryStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := MemoryStats{
		Total:      len(s.entries),
		ByApp:      make(map[string]int),
		ByCategory: make(map[MemoryCategory]int),
	}

	for _, e := range s.entries {
		if e.AppID != "" {
			stats.ByApp[e.AppID]++
		}
		if e.Category != "" {
			stats.ByCategory[e.Category]++
		}
		if stats.OldestAt.IsZero() || e.CreatedAt.Before(stats.OldestAt) {
			stats.OldestAt = e.CreatedAt
		}
		if e.CreatedAt.After(stats.NewestAt) {
			stats.NewestAt = e.CreatedAt
		}
	}
	return stats
}
