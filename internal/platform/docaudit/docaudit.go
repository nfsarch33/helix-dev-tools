package docaudit

import "time"

// DocStatus indicates whether a document is fresh or stale
type DocStatus string

const (
	DocFresh DocStatus = "fresh"
	DocStale DocStatus = "stale"
)

// DocEntry represents one document in the audit
type DocEntry struct {
	Path        string
	LastUpdated time.Time
	MaxAgeDays  int
	Status      DocStatus
}

// IsStale returns true when the doc exceeds its MaxAgeDays
func (d DocEntry) IsStale(now time.Time) bool {
	if d.MaxAgeDays <= 0 {
		return false
	}
	age := now.Sub(d.LastUpdated)
	return age.Hours()/24 > float64(d.MaxAgeDays)
}

// Auditor runs document freshness checks
type Auditor struct {
	entries []DocEntry
	now     time.Time
}

// NewAuditor creates an auditor using the given reference time
func NewAuditor(now time.Time) *Auditor {
	return &Auditor{now: now}
}

// Register adds a document to audit
func (a *Auditor) Register(e DocEntry) {
	a.entries = append(a.entries, e)
}

// Run computes the status of all entries and returns them
func (a *Auditor) Run() []DocEntry {
	result := make([]DocEntry, len(a.entries))
	for i, e := range a.entries {
		if e.IsStale(a.now) {
			e.Status = DocStale
		} else {
			e.Status = DocFresh
		}
		result[i] = e
	}
	return result
}

// StaleCount returns the number of stale docs
func (a *Auditor) StaleCount() int {
	n := 0
	for _, e := range a.Run() {
		if e.Status == DocStale {
			n++
		}
	}
	return n
}
