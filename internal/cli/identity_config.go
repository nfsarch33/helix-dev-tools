package cli

import "os"

// personalEmail returns the expected personal git email for identity
// enforcement. Set via RUNX_PERSONAL_EMAIL env var. When empty, the
// identity gate still blocks Zendesk emails but skips the exact-match
// check against a specific address.
func personalEmail() string {
	return os.Getenv("RUNX_PERSONAL_EMAIL")
}

// personalEmailOrPlaceholder returns the configured email for use in
// remediation messages, or a generic placeholder when unconfigured.
func personalEmailOrPlaceholder() string {
	if e := personalEmail(); e != "" {
		return e
	}
	return "<your-personal-email>"
}
