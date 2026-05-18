package deploymgr

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"
)

// BinaryRecord tracks a deployed binary
type BinaryRecord struct {
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	Path       string    `json:"path"`
	SHA256     string    `json:"sha256"`
	DeployedAt time.Time `json:"deployed_at"`
}

// Manager tracks binary deployments
type Manager struct {
	records []BinaryRecord
}

// NewManager creates a new deployment manager
func NewManager() *Manager {
	return &Manager{}
}

// Register adds or updates a binary record. If a record with the same name
// exists, the previous version is kept as rollback.
func (m *Manager) Register(r BinaryRecord) {
	if r.DeployedAt.IsZero() {
		r.DeployedAt = time.Now()
	}
	m.records = append(m.records, r)
}

// Latest returns the most recently registered record for the given binary name
func (m *Manager) Latest(name string) (BinaryRecord, bool) {
	for i := len(m.records) - 1; i >= 0; i-- {
		if m.records[i].Name == name {
			return m.records[i], true
		}
	}
	return BinaryRecord{}, false
}

// Previous returns the record before the latest for rollback
func (m *Manager) Previous(name string) (BinaryRecord, bool) {
	found := false
	for i := len(m.records) - 1; i >= 0; i-- {
		if m.records[i].Name == name {
			if found {
				return m.records[i], true
			}
			found = true
		}
	}
	return BinaryRecord{}, false
}

// All returns all records for the given binary name in registration order
func (m *Manager) All(name string) []BinaryRecord {
	var results []BinaryRecord
	for _, r := range m.records {
		if r.Name == name {
			results = append(results, r)
		}
	}
	return results
}

// HashFile computes the SHA256 hex digest of a file
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
