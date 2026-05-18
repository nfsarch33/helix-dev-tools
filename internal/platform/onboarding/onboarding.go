package onboarding

import "time"

// StepStatus tracks completion of an onboarding step
type StepStatus string

const (
	StepPending  StepStatus = "pending"
	StepComplete StepStatus = "complete"
	StepSkipped  StepStatus = "skipped"
)

// Step is one item in the onboarding checklist
type Step struct {
	ID          string
	Title       string
	Status      StepStatus
	CompletedAt time.Time
}

// Checklist tracks a new user's onboarding progress
type Checklist struct {
	steps []Step
}

// NewChecklist returns a checklist with the standard onboarding steps
func NewChecklist() *Checklist {
	return &Checklist{
		steps: []Step{
			{ID: "store", Title: "Set up store", Status: StepPending},
			{ID: "product", Title: "Add first product", Status: StepPending},
			{ID: "payment", Title: "Configure payment", Status: StepPending},
			{ID: "domain", Title: "Connect domain", Status: StepPending},
		},
	}
}

// Complete marks a step as done. Returns false if the step ID is not found.
func (c *Checklist) Complete(id string) bool {
	for i := range c.steps {
		if c.steps[i].ID == id {
			c.steps[i].Status = StepComplete
			c.steps[i].CompletedAt = time.Now()
			return true
		}
	}
	return false
}

// Skip marks a step as skipped. Returns false if the step ID is not found.
func (c *Checklist) Skip(id string) bool {
	for i := range c.steps {
		if c.steps[i].ID == id {
			c.steps[i].Status = StepSkipped
			return true
		}
	}
	return false
}

// Progress returns the number of completed steps and the total.
func (c *Checklist) Progress() (done, total int) {
	total = len(c.steps)
	for _, s := range c.steps {
		if s.Status == StepComplete {
			done++
		}
	}
	return
}

// IsFinished returns true when every step is either complete or skipped.
func (c *Checklist) IsFinished() bool {
	for _, s := range c.steps {
		if s.Status == StepPending {
			return false
		}
	}
	return true
}

// Steps returns a copy of all steps.
func (c *Checklist) Steps() []Step {
	result := make([]Step, len(c.steps))
	copy(result, c.steps)
	return result
}
