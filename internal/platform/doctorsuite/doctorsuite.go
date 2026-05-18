package doctorsuite

// CheckLevel indicates the severity of a doctor check result
type CheckLevel string

const (
	LevelPass  CheckLevel = "PASS"
	LevelWarn  CheckLevel = "WARN"
	LevelFail  CheckLevel = "FAIL"
)

// CheckResult holds the outcome of one doctor check
type CheckResult struct {
	Name    string
	Level   CheckLevel
	Message string
}

// CheckFn is a function that performs one health check
type CheckFn func() CheckResult

// Suite runs a collection of health checks
type Suite struct {
	checks []struct {
		name string
		fn   CheckFn
	}
}

// NewSuite creates an empty doctor suite
func NewSuite() *Suite {
	return &Suite{}
}

// Register adds a named health check
func (s *Suite) Register(name string, fn CheckFn) {
	s.checks = append(s.checks, struct {
		name string
		fn   CheckFn
	}{name, fn})
}

// Run executes all checks and returns results
func (s *Suite) Run() []CheckResult {
	var results []CheckResult
	for _, c := range s.checks {
		results = append(results, c.fn())
	}
	return results
}

// ExitCode returns 0 if all pass, 1 if any warn, 2 if any fail
func ExitCode(results []CheckResult) int {
	code := 0
	for _, r := range results {
		switch r.Level {
		case LevelFail:
			return 2
		case LevelWarn:
			if code < 1 {
				code = 1
			}
		}
	}
	return code
}

// Summary returns PASS/WARN/FAIL counts from results
func Summary(results []CheckResult) (pass, warn, fail int) {
	for _, r := range results {
		switch r.Level {
		case LevelPass:
			pass++
		case LevelWarn:
			warn++
		case LevelFail:
			fail++
		}
	}
	return
}
