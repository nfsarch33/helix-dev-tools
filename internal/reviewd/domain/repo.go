package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Tier classifies a repository's review depth and write permissions.
type Tier int

const (
	TierExcluded Tier = iota
	TierPilot
	TierA
	TierB
	TierC
)

var tierNames = [...]string{
	TierExcluded: "excluded",
	TierPilot:    "pilot",
	TierA:        "a",
	TierB:        "b",
	TierC:        "c",
}

func (t Tier) String() string {
	if int(t) < len(tierNames) {
		return tierNames[t]
	}
	return fmt.Sprintf("tier(%d)", int(t))
}

func (t Tier) MarshalJSON() ([]byte, error) { return json.Marshal(t.String()) }

func (t *Tier) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, ok := ParseTier(s)
	if !ok {
		return fmt.Errorf("reviewd: unknown tier %q", s)
	}
	*t = parsed
	return nil
}

// ParseTier maps a string to a Tier, returning false if unknown.
func ParseTier(s string) (Tier, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "excluded":
		return TierExcluded, true
	case "pilot":
		return TierPilot, true
	case "a":
		return TierA, true
	case "b":
		return TierB, true
	case "c":
		return TierC, true
	default:
		return TierExcluded, false
	}
}

// AllowsIssueWrite returns true if the tier permits posting findings
// as GitHub issues. Excluded and Pilot tiers are read-only.
func (t Tier) AllowsIssueWrite() bool {
	return t >= TierA
}

// Repo is a repository registered for automated code review.
type Repo struct {
	Alias       string `json:"alias"`
	Tier        Tier   `json:"tier"`
	DefaultRef  string `json:"default_ref"`
	LastScanned string `json:"last_scanned_sha,omitempty"`
}
