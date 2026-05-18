package freshinstall

// CheckCategory groups validation checks into logical areas
type CheckCategory string

const (
	CategoryMCP      CheckCategory = "mcp"
	CategoryHooks    CheckCategory = "hooks"
	CategoryAgent    CheckCategory = "agent"
	CategoryTools    CheckCategory = "tools"
)

// CheckResult holds one validation outcome
type CheckResult struct {
	Category CheckCategory
	Name     string
	Passed   bool
	Detail   string
}

// ValidationSuite runs fresh-install validation checks
type ValidationSuite struct {
	checks []struct {
		category CheckCategory
		name     string
		fn       func() (bool, string)
	}
}

// NewValidationSuite creates an empty suite
func NewValidationSuite() *ValidationSuite {
	return &ValidationSuite{}
}

// Register adds a named check to the suite
func (vs *ValidationSuite) Register(category CheckCategory, name string, fn func() (bool, string)) {
	vs.checks = append(vs.checks, struct {
		category CheckCategory
		name     string
		fn       func() (bool, string)
	}{category, name, fn})
}

// Run executes all checks and returns results
func (vs *ValidationSuite) Run() []CheckResult {
	var results []CheckResult
	for _, c := range vs.checks {
		passed, detail := c.fn()
		results = append(results, CheckResult{
			Category: c.category,
			Name:     c.name,
			Passed:   passed,
			Detail:   detail,
		})
	}
	return results
}

// Summary returns pass/fail counts
func Summary(results []CheckResult) (passed, failed int) {
	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}
	return
}

// ByCategory filters results to a single category
func ByCategory(results []CheckResult, cat CheckCategory) []CheckResult {
	var filtered []CheckResult
	for _, r := range results {
		if r.Category == cat {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
