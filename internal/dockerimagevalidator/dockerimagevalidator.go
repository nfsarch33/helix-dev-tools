package dockerimagevalidator

import "strings"

// Result describes image reference validation.
type Result struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ValidateImageReference checks that an image reference is qualified and pinned.
func ValidateImageReference(ref string) Result {
	var result Result
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		result.Errors = append(result.Errors, "image reference is required")
		result.Valid = false
		return result
	}
	if !strings.Contains(trimmed, "/") {
		result.Errors = append(result.Errors, "image reference must include a registry or namespace")
	}
	if strings.HasSuffix(trimmed, ":latest") {
		result.Warnings = append(result.Warnings, "latest tag is mutable; prefer semver or digest pin")
	}
	if !strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "@sha256:") {
		result.Errors = append(result.Errors, "image reference must include a tag or sha256 digest")
	}
	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateImageSet checks a set of image references and rejects duplicates.
func ValidateImageSet(refs []string) Result {
	var result Result
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if seen[ref] {
			result.Errors = append(result.Errors, "duplicate image reference "+ref)
		}
		seen[ref] = true
		child := ValidateImageReference(ref)
		result.Errors = append(result.Errors, child.Errors...)
		result.Warnings = append(result.Warnings, child.Warnings...)
	}
	result.Valid = len(result.Errors) == 0
	return result
}
