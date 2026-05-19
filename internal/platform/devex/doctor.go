package devex

import "fmt"

type CheckStatus int

const (
	CheckPass CheckStatus = iota
	CheckWarn
	CheckFail
)

type DiagnosticCheck struct {
	Name    string
	Status  CheckStatus
	Message string
	FixHint string
}

type DiagnosticRunner struct {
	checks []func() DiagnosticCheck
}

func NewDiagnosticRunner() *DiagnosticRunner {
	return &DiagnosticRunner{}
}

func (d *DiagnosticRunner) Register(check func() DiagnosticCheck) {
	d.checks = append(d.checks, check)
}

func (d *DiagnosticRunner) Run() []DiagnosticCheck {
	results := make([]DiagnosticCheck, 0, len(d.checks))
	for _, check := range d.checks {
		results = append(results, check())
	}
	return results
}

func (d *DiagnosticRunner) CheckCount() int {
	return len(d.checks)
}

func SummarizeResults(results []DiagnosticCheck) (pass, warn, fail int) {
	for _, r := range results {
		switch r.Status {
		case CheckPass:
			pass++
		case CheckWarn:
			warn++
		case CheckFail:
			fail++
		}
	}
	return
}

func OverallStatus(results []DiagnosticCheck) string {
	_, _, fail := SummarizeResults(results)
	if fail > 0 {
		return "RED"
	}
	_, warn, _ := SummarizeResults(results)
	if warn > 0 {
		return "YELLOW"
	}
	return "GREEN"
}

func FormatResults(results []DiagnosticCheck) string {
	var out string
	for _, r := range results {
		icon := "PASS"
		if r.Status == CheckWarn {
			icon = "WARN"
		} else if r.Status == CheckFail {
			icon = "FAIL"
		}
		out += fmt.Sprintf("[%s] %s: %s\n", icon, r.Name, r.Message)
		if r.FixHint != "" && r.Status != CheckPass {
			out += fmt.Sprintf("  Fix: %s\n", r.FixHint)
		}
	}
	pass, warn, fail := SummarizeResults(results)
	out += fmt.Sprintf("\nSummary: %d pass, %d warn, %d fail -> %s\n", pass, warn, fail, OverallStatus(results))
	return out
}
