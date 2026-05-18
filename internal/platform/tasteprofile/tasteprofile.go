package tasteprofile

import (
	"time"

	"gopkg.in/yaml.v3"
)

// Entry is a single preference entry in a taste profile
type Entry struct {
	Key        string  `yaml:"key" json:"key"`
	Value      string  `yaml:"value" json:"value"`
	Confidence float64 `yaml:"confidence" json:"confidence"`
}

// Profile represents an agent's taste profile
type Profile struct {
	AgentID   string    `yaml:"agent_id" json:"agent_id"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	Entries   []Entry   `yaml:"entries" json:"entries"`
}

// Export marshals the profile to YAML bytes
func Export(p Profile) ([]byte, error) {
	return yaml.Marshal(p)
}

// Import parses YAML bytes into a Profile
func Import(data []byte) (Profile, error) {
	var p Profile
	err := yaml.Unmarshal(data, &p)
	return p, err
}

// Merge combines two profiles, keeping entries from both, preferring higher confidence on key conflict
func Merge(a, b Profile) Profile {
	// Create a map to track entries by key
	entryMap := make(map[string]Entry)

	// Add entries from profile a
	for _, entry := range a.Entries {
		entryMap[entry.Key] = entry
	}

	// Add or replace entries from profile b, keeping higher confidence
	for _, entry := range b.Entries {
		existingEntry, exists := entryMap[entry.Key]
		if !exists || entry.Confidence > existingEntry.Confidence {
			entryMap[entry.Key] = entry
		}
	}

	// Convert map back to slice
	var mergedEntries []Entry
	for _, entry := range entryMap {
		mergedEntries = append(mergedEntries, entry)
	}

	// Prefer a's AgentID and CreatedAt if not changed significantly
	mergedProfile := Profile{
		AgentID:   a.AgentID,
		CreatedAt: a.CreatedAt,
		Entries:   mergedEntries,
	}

	return mergedProfile
}

// Diff returns entries that differ between a and b by key
type DiffEntry struct {
	Key     string
	OldVal  string
	NewVal  string
	Changed bool
}

// Diff compares two profiles and returns their differences
func Diff(a, b Profile) []DiffEntry {
	var diffs []DiffEntry

	// Create a map of entries in profile a
	aEntries := make(map[string]Entry)
	for _, entry := range a.Entries {
		aEntries[entry.Key] = entry
	}

	// Compare entries from profile b
	for _, bEntry := range b.Entries {
		aEntry, exists := aEntries[bEntry.Key]

		if !exists {
			// New key in b
			diffs = append(diffs, DiffEntry{
				Key:      bEntry.Key,
				NewVal:   bEntry.Value,
				Changed:  true,
			})
		} else if aEntry.Value != bEntry.Value {
			// Different value for existing key
			diffs = append(diffs, DiffEntry{
				Key:      bEntry.Key,
				OldVal:   aEntry.Value,
				NewVal:   bEntry.Value,
				Changed:  true,
			})
		}
	}

	return diffs
}