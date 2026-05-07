package identity

import "strings"

// TokenKeys mirrors the personal-repo strict gate token family.
var TokenKeys = []string{
	"GITHUB_TOKEN",
	"GITHUB_API_TOKEN",
	"HOMEBREW_GITHUB_API_TOKEN",
	"VENDIR_GITHUB_API_TOKEN",
}

// SelfTestResult is the machine-readable identity shell hygiene check.
type SelfTestResult struct {
	Pass     bool
	Failures []string
}

// SelfTest detects inherited token variables that make personal-repo push gates
// fail. The values are intentionally ignored so secrets never reach output.
func SelfTest(env map[string]string) SelfTestResult {
	var failures []string
	for _, key := range TokenKeys {
		if strings.TrimSpace(env[key]) != "" {
			failures = append(failures, key+" must be unset; run `runx env personal-shell` or `runx env scrub`")
		}
	}
	return SelfTestResult{Pass: len(failures) == 0, Failures: failures}
}
