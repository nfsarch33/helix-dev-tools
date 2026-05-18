package programclose

import "time"

// Milestone tracks a programme close milestone
type Milestone struct {
	ID          string
	Description string
	Done        bool
	CompletedAt time.Time
}

// Complete marks the milestone done
func (m *Milestone) Complete() {
	m.Done = true
	m.CompletedAt = time.Now()
}

// ProgrammeClose manages the final closeout checklist for a 100-sprint programme
type ProgrammeClose struct {
	ProgrammeID  string
	SprintRange  string
	milestones   []Milestone
}

// New creates a ProgrammeClose with the standard closeout milestones
func New(programmeID, sprintRange string) *ProgrammeClose {
	return &ProgrammeClose{
		ProgrammeID: programmeID,
		SprintRange: sprintRange,
		milestones: []Milestone{
			{ID: "qa-sweep", Description: "QA sweep across all repos"},
			{ID: "doc-audit", Description: "Documentation completeness audit"},
			{ID: "fresh-install", Description: "Fresh install validation"},
			{ID: "orhep-cycle", Description: "Final ORHEP cycle"},
			{ID: "retro", Description: "Programme retrospective"},
			{ID: "handoff", Description: "Session handoff document"},
		},
	}
}

// Complete marks a milestone done by ID. Returns false if not found.
func (pc *ProgrammeClose) Complete(id string) bool {
	for i := range pc.milestones {
		if pc.milestones[i].ID == id {
			pc.milestones[i].Complete()
			return true
		}
	}
	return false
}

// Progress returns completed and total milestone counts
func (pc *ProgrammeClose) Progress() (done, total int) {
	total = len(pc.milestones)
	for _, m := range pc.milestones {
		if m.Done {
			done++
		}
	}
	return
}

// IsClosed returns true when all milestones are complete
func (pc *ProgrammeClose) IsClosed() bool {
	for _, m := range pc.milestones {
		if !m.Done {
			return false
		}
	}
	return true
}

// Milestones returns a copy of all milestones
func (pc *ProgrammeClose) Milestones() []Milestone {
	result := make([]Milestone, len(pc.milestones))
	copy(result, pc.milestones)
	return result
}
