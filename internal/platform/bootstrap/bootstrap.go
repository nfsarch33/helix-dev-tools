package bootstrap

import "time"

// StepResult records the outcome of one bootstrap step
type StepResult struct {
	Name      string
	Passed    bool
	Output    string
	Duration  time.Duration
}

// Report summarises a full bootstrap run
type Report struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Steps      []StepResult
}

// Passed returns true when every step passed
func (r *Report) Passed() bool {
	for _, s := range r.Steps {
		if !s.Passed {
			return false
		}
	}
	return true
}

// PassCount returns the number of steps that passed
func (r *Report) PassCount() int {
	n := 0
	for _, s := range r.Steps {
		if s.Passed {
			n++
		}
	}
	return n
}

// FailedSteps returns all step names that did not pass
func (r *Report) FailedSteps() []string {
	var names []string
	for _, s := range r.Steps {
		if !s.Passed {
			names = append(names, s.Name)
		}
	}
	return names
}

// StepFn is a function that executes one bootstrap step
type StepFn func() (output string, err error)

// Runner executes a list of named bootstrap steps
type Runner struct {
	steps []struct {
		name string
		fn   StepFn
	}
}

// NewRunner creates a bootstrap runner
func NewRunner() *Runner {
	return &Runner{}
}

// Register adds a named step
func (r *Runner) Register(name string, fn StepFn) {
	r.steps = append(r.steps, struct {
		name string
		fn   StepFn
	}{name, fn})
}

// Run executes all registered steps and returns a Report
func (r *Runner) Run() Report {
	report := Report{StartedAt: time.Now()}
	for _, s := range r.steps {
		start := time.Now()
		out, err := s.fn()
		result := StepResult{
			Name:     s.name,
			Passed:   err == nil,
			Output:   out,
			Duration: time.Since(start),
		}
		if err != nil {
			result.Output = err.Error()
		}
		report.Steps = append(report.Steps, result)
	}
	report.FinishedAt = time.Now()
	return report
}
