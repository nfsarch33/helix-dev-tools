package qasweep

import "time"

// RepoResult holds QA sweep results for one repo
type RepoResult struct {
	RepoName    string
	SentruxPass bool
	TestPass    bool
	ShellLeak   bool
	RunAt       time.Time
}

// Overall returns true when no issues were found
func (r RepoResult) Overall() bool {
	return r.SentruxPass && r.TestPass && !r.ShellLeak
}

// Report aggregates QA sweep results across multiple repos
type Report struct {
	StartedAt  time.Time
	Results    []RepoResult
}

// NewReport creates an empty sweep report
func NewReport() *Report {
	return &Report{StartedAt: time.Now()}
}

// Add appends a repo result
func (r *Report) Add(result RepoResult) {
	if result.RunAt.IsZero() {
		result.RunAt = time.Now()
	}
	r.Results = append(r.Results, result)
}

// AllPassed returns true when every repo passed
func (r *Report) AllPassed() bool {
	for _, res := range r.Results {
		if !res.Overall() {
			return false
		}
	}
	return true
}

// FailedRepos returns the names of repos that had QA failures
func (r *Report) FailedRepos() []string {
	var failed []string
	for _, res := range r.Results {
		if !res.Overall() {
			failed = append(failed, res.RepoName)
		}
	}
	return failed
}

// PassCount returns the number of repos that passed
func (r *Report) PassCount() int {
	n := 0
	for _, res := range r.Results {
		if res.Overall() {
			n++
		}
	}
	return n
}
