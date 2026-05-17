package fleetreadiness

// Check is a single fleet-readiness gate.
type Check struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Required bool   `json:"required"`
}

// Result summarises whether the fleet can move to the next release gate.
type Result struct {
	Ready          bool     `json:"ready"`
	Total          int      `json:"total"`
	Passed         int      `json:"passed"`
	Failed         int      `json:"failed"`
	RequiredFailed int      `json:"required_failed"`
	OptionalFailed int      `json:"optional_failed"`
	Errors         []string `json:"errors"`
	Warnings       []string `json:"warnings"`
}

// EvaluateReadiness aggregates required and optional checks into one verdict.
func EvaluateReadiness(checks []Check) Result {
	var result Result
	result.Total = len(checks)
	if len(checks) == 0 {
		result.Errors = append(result.Errors, "at least one readiness check is required")
		result.Ready = false
		return result
	}
	for _, check := range checks {
		if check.Passed {
			result.Passed++
			continue
		}
		result.Failed++
		if check.Required {
			result.RequiredFailed++
			result.Errors = append(result.Errors, "required check failed: "+check.Name)
		} else {
			result.OptionalFailed++
			result.Warnings = append(result.Warnings, "optional check failed: "+check.Name)
		}
	}
	result.Ready = result.RequiredFailed == 0
	return result
}
