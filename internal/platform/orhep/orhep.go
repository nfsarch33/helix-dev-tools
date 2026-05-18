package orhep

import "time"

// Phase name constants for the ORHEP cycle
const (
	PhaseObserve = "observe"
	PhaseReflect = "reflect"
	PhaseHeal    = "heal"
	PhaseEvolve  = "evolve"
	PhasePromote = "promote"
)

// CapsuleEntry is one observation or action in an ORHEP cycle
type CapsuleEntry struct {
	Phase     string
	Summary   string
	Timestamp time.Time
}

// Capsule is one complete ORHEP cycle
type Capsule struct {
	ID          string
	SprintRange string
	Entries     []CapsuleEntry
	Promoted    bool
	ClosedAt    time.Time
}

// Add appends an entry to the capsule
func (c *Capsule) Add(phase, summary string) {
	c.Entries = append(c.Entries, CapsuleEntry{
		Phase:     phase,
		Summary:   summary,
		Timestamp: time.Now(),
	})
}

// IsComplete returns true when all 5 ORHEP phases have at least one entry
func (c *Capsule) IsComplete() bool {
	phases := map[string]bool{}
	for _, e := range c.Entries {
		phases[e.Phase] = true
	}
	return phases[PhaseObserve] && phases[PhaseReflect] && phases[PhaseHeal] &&
		phases[PhaseEvolve] && phases[PhasePromote]
}

// Promote marks the capsule as promoted and records the close time
func (c *Capsule) Promote() {
	c.Promoted = true
	c.ClosedAt = time.Now()
}

// Store holds all ORHEP capsules
type Store struct {
	capsules []Capsule
}

// NewStore creates an empty store
func NewStore() *Store {
	return &Store{}
}

// Save adds a capsule to the store
func (s *Store) Save(c Capsule) {
	s.capsules = append(s.capsules, c)
}

// Get returns the capsule with the given ID, or false
func (s *Store) Get(id string) (Capsule, bool) {
	for _, c := range s.capsules {
		if c.ID == id {
			return c, true
		}
	}
	return Capsule{}, false
}

// Promoted returns all capsules that were promoted
func (s *Store) Promoted() []Capsule {
	var result []Capsule
	for _, c := range s.capsules {
		if c.Promoted {
			result = append(result, c)
		}
	}
	return result
}
