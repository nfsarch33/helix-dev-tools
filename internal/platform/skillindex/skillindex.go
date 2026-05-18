package skillindex

import (
	"fmt"
	"sort"
)

// SkillEntry represents one entry in the skill index
type SkillEntry struct {
	ID          string
	Title       string
	Category    string
	Path        string
	HasSKILLMD  bool
}

// Index manages the skill registry
type Index struct {
	skills map[string]SkillEntry
}

// NewIndex creates an empty skill index
func NewIndex() *Index {
	return &Index{skills: map[string]SkillEntry{}}
}

// Register adds a skill to the index
func (idx *Index) Register(e SkillEntry) {
	idx.skills[e.ID] = e
}

// Lookup returns the skill with the given ID
func (idx *Index) Lookup(id string) (SkillEntry, bool) {
	e, ok := idx.skills[id]
	return e, ok
}

// All returns all skills sorted by ID
func (idx *Index) All() []SkillEntry {
	var result []SkillEntry
	for _, e := range idx.skills {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ByCategory returns all skills in a given category
func (idx *Index) ByCategory(category string) []SkillEntry {
	var result []SkillEntry
	for _, e := range idx.skills {
		if e.Category == category {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// StaleEntries returns IDs of skills where HasSKILLMD is false
func (idx *Index) StaleEntries() []string {
	var stale []string
	for id, e := range idx.skills {
		if !e.HasSKILLMD {
			stale = append(stale, id)
		}
	}
	sort.Strings(stale)
	return stale
}

// Remove deletes a skill from the index. Returns error if not found.
func (idx *Index) Remove(id string) error {
	if _, ok := idx.skills[id]; !ok {
		return fmt.Errorf("skill %q not found in index", id)
	}
	delete(idx.skills, id)
	return nil
}
