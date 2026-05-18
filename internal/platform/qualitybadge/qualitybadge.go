package qualitybadge

import "time"

// Badge level names
type Badge string

const (
	BadgePlatinum Badge = "platinum"
	BadgeGold     Badge = "gold"
	BadgeSilver   Badge = "silver"
	BadgeBronze   Badge = "bronze"
)

// BadgeRecord holds one badge award
type BadgeRecord struct {
	RepoID    string
	Badge     Badge
	Score     float64
	AwardedAt time.Time
}

// Compute returns the badge for a given score
func Compute(score float64) Badge {
	switch {
	case score >= 0.90:
		return BadgePlatinum
	case score >= 0.75:
		return BadgeGold
	case score >= 0.60:
		return BadgeSilver
	default:
		return BadgeBronze
	}
}

// History tracks badge awards over time
type History struct {
	records []BadgeRecord
}

// NewHistory creates an empty badge history
func NewHistory() *History {
	return &History{}
}

// Record stores a badge award
func (h *History) Record(r BadgeRecord) {
	if r.AwardedAt.IsZero() {
		r.AwardedAt = time.Now()
	}
	h.records = append(h.records, r)
}

// Latest returns the most recent record for a repo, or false
func (h *History) Latest(repoID string) (BadgeRecord, bool) {
	var latest BadgeRecord
	found := false
	for _, r := range h.records {
		if r.RepoID == repoID {
			if !found || r.AwardedAt.After(latest.AwardedAt) {
				latest = r
				found = true
			}
		}
	}
	return latest, found
}

// AllForRepo returns all badge records for a repo in chronological order
func (h *History) AllForRepo(repoID string) []BadgeRecord {
	var result []BadgeRecord
	for _, r := range h.records {
		if r.RepoID == repoID {
			result = append(result, r)
		}
	}
	return result
}

// Regressed returns true if the latest badge for a repo is worse than the previous one
func (h *History) Regressed(repoID string) bool {
	all := h.AllForRepo(repoID)
	if len(all) < 2 {
		return false
	}
	prev := all[len(all)-2]
	curr := all[len(all)-1]
	return curr.Score < prev.Score
}
