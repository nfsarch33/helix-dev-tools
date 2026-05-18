package tasteprofile

import (
	"testing"
	"time"
)

func TestExport_RoundTrip(t *testing.T) {
	original := Profile{
		AgentID:   "test-agent",
		CreatedAt: time.Now().UTC(),
		Entries: []Entry{
			{Key: "language", Value: "Go", Confidence: 0.9},
			{Key: "framework", Value: "React", Confidence: 0.7},
		},
	}

	// Export the profile
	data, err := Export(original)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import the profile
	imported, err := Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Check basic fields
	if imported.AgentID != original.AgentID {
		t.Errorf("AgentID mismatch: got %v, want %v", imported.AgentID, original.AgentID)
	}

	// Allow small time difference due to potential microsecond precision loss
	timeDiff := imported.CreatedAt.Sub(original.CreatedAt)
	if timeDiff < 0 || timeDiff > time.Millisecond {
		t.Errorf("CreatedAt mismatch: got %v, want close to %v", imported.CreatedAt, original.CreatedAt)
	}

	// Check entries
	if len(imported.Entries) != len(original.Entries) {
		t.Fatalf("Entries length mismatch: got %d, want %d", len(imported.Entries), len(original.Entries))
	}

	for i, entry := range original.Entries {
		if imported.Entries[i].Key != entry.Key ||
			imported.Entries[i].Value != entry.Value ||
			imported.Entries[i].Confidence != entry.Confidence {
			t.Errorf("Entry %d mismatch: got %v, want %v", i, imported.Entries[i], entry)
		}
	}
}

func TestImport_InvalidYAML(t *testing.T) {
	invalidYAML := []byte("invalid: yaml: content")

	_, err := Import(invalidYAML)
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}
}

func TestMerge_PreservesAllKeys(t *testing.T) {
	a := Profile{
		AgentID:   "agent-a",
		CreatedAt: time.Now().UTC(),
		Entries: []Entry{
			{Key: "language", Value: "Go", Confidence: 0.9},
		},
	}

	b := Profile{
		AgentID:   "agent-b",
		CreatedAt: time.Now().Add(time.Hour),
		Entries: []Entry{
			{Key: "framework", Value: "React", Confidence: 0.7},
		},
	}

	merged := Merge(a, b)

	if len(merged.Entries) != 2 {
		t.Fatalf("Merged entries length incorrect: got %d, want 2", len(merged.Entries))
	}

	// Check that both unique entries are preserved
	hasLanguage := false
	hasFramework := false
	for _, entry := range merged.Entries {
		if entry.Key == "language" {
			hasLanguage = true
		}
		if entry.Key == "framework" {
			hasFramework = true
		}
	}

	if !hasLanguage || !hasFramework {
		t.Error("Merged profile missing one or more entries")
	}
}

func TestMerge_HigherConfidenceWins(t *testing.T) {
	a := Profile{
		AgentID:   "agent-a",
		CreatedAt: time.Now().UTC(),
		Entries: []Entry{
			{Key: "language", Value: "Go", Confidence: 0.5},
		},
	}

	b := Profile{
		AgentID:   "agent-b",
		CreatedAt: time.Now().Add(time.Hour),
		Entries: []Entry{
			{Key: "language", Value: "Python", Confidence: 0.9},
		},
	}

	merged := Merge(a, b)

	// Find the entry with the key "language"
	var langEntry Entry
	for _, entry := range merged.Entries {
		if entry.Key == "language" {
			langEntry = entry
			break
		}
	}

	if langEntry.Value != "Python" || langEntry.Confidence != 0.9 {
		t.Errorf("Higher confidence entry not selected: got %+v", langEntry)
	}
}

func TestDiff_NoChanges(t *testing.T) {
	profile := Profile{
		AgentID:   "test-agent",
		CreatedAt: time.Now().UTC(),
		Entries: []Entry{
			{Key: "language", Value: "Go", Confidence: 0.9},
		},
	}

	diff := Diff(profile, profile)
	if len(diff) != 0 {
		t.Errorf("Expected no differences, got %d", len(diff))
	}
}

func TestDiff_DetectsNewValue(t *testing.T) {
	a := Profile{
		AgentID:   "agent-a",
		CreatedAt: time.Now().UTC(),
		Entries: []Entry{
			{Key: "language", Value: "Go", Confidence: 0.9},
		},
	}

	b := Profile{
		AgentID:   "agent-b",
		CreatedAt: time.Now().Add(time.Hour),
		Entries: []Entry{
			{Key: "language", Value: "Python", Confidence: 0.7},
		},
	}

	diff := Diff(a, b)
	if len(diff) != 1 {
		t.Fatalf("Expected 1 difference, got %d", len(diff))
	}

	if !diff[0].Changed ||
	   diff[0].Key != "language" ||
	   diff[0].OldVal != "Go" ||
	   diff[0].NewVal != "Python" {
		t.Errorf("Unexpected diff: %+v", diff[0])
	}
}

func TestProfile_EmptyEntries(t *testing.T) {
	profile := Profile{
		AgentID:   "test-agent",
		CreatedAt: time.Now().UTC(),
		Entries:   []Entry{},
	}

	// Test Export
	data, err := Export(profile)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Test Import
	imported, err := Import(data)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify imported profile matches
	if imported.AgentID != profile.AgentID {
		t.Errorf("AgentID mismatch: got %v, want %v", imported.AgentID, profile.AgentID)
	}

	// Allow small time difference due to potential microsecond precision loss
	timeDiff := imported.CreatedAt.Sub(profile.CreatedAt)
	if timeDiff < 0 || timeDiff > time.Millisecond {
		t.Errorf("CreatedAt mismatch: got %v, want close to %v", imported.CreatedAt, profile.CreatedAt)
	}

	if len(imported.Entries) != 0 {
		t.Errorf("Expected empty entries, got %d", len(imported.Entries))
	}
}