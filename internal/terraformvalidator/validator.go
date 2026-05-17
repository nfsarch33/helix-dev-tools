package terraformvalidator

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	terraformBlockRe     = regexp.MustCompile(`(?m)^terraform\s*\{`)
	requiredProvidersRe  = regexp.MustCompile(`required_providers\s*\{`)
	providerBlockRe      = regexp.MustCompile(`(?m)^provider\s+"([^"]+)"`)
	backendBlockRe       = regexp.MustCompile(`backend\s+"([^"]+)"`)
	bucketRe             = regexp.MustCompile(`bucket\s*=\s*"([^"]+)"`)
	prefixRe             = regexp.MustCompile(`prefix\s*=\s*"([^"]+)"`)
	versionConstraintRe  = regexp.MustCompile(`version\s*=\s*"([^"]+)"`)
	impersonateRe        = regexp.MustCompile(`impersonate_service_account\s*=\s*"([^"]+)"`)
	projectRe            = regexp.MustCompile(`project\s*=\s*"([^"]+)"`)
	regionRe             = regexp.MustCompile(`region\s*=\s*"([^"]+)"`)
)

// ValidateHCLSyntax performs structural validation on a Terraform HCL config string.
func ValidateHCLSyntax(hcl string, reqs HCLRequirements) (*ValidationResult, error) {
	result := &ValidationResult{Name: "HCL-Syntax"}

	hcl = strings.TrimSpace(hcl)
	if hcl == "" {
		result.Failures = append(result.Failures, "empty HCL input")
		return result, nil
	}

	if reqs.RequireTerraformBlock {
		if terraformBlockRe.MatchString(hcl) {
			result.Passed = append(result.Passed, "terraform block present")
		} else {
			result.Failures = append(result.Failures, "terraform block not found")
		}
	}

	if reqs.RequireBackendType != "" {
		matches := backendBlockRe.FindStringSubmatch(hcl)
		if len(matches) > 1 {
			if matches[1] == reqs.RequireBackendType {
				result.Passed = append(result.Passed, fmt.Sprintf("backend type %q", matches[1]))
			} else {
				result.Failures = append(result.Failures,
					fmt.Sprintf("backend type: want %q, got %q", reqs.RequireBackendType, matches[1]))
			}
		} else {
			result.Failures = append(result.Failures, "no backend block found")
		}
	}

	providerMatches := providerBlockRe.FindAllStringSubmatch(hcl, -1)
	foundProviders := make(map[string]bool)
	for _, m := range providerMatches {
		if len(m) > 1 {
			foundProviders[m[1]] = true
		}
	}

	for _, p := range reqs.RequireProviders {
		if foundProviders[p] {
			result.Passed = append(result.Passed, fmt.Sprintf("provider %q declared", p))
		} else {
			result.Failures = append(result.Failures, fmt.Sprintf("provider %q not declared", p))
		}
	}

	return result, nil
}

// ValidateProviderConfig validates provider blocks for version constraints and impersonation.
func ValidateProviderConfig(hcl string, reqs ProviderRequirements) (*ValidationResult, error) {
	result := &ValidationResult{Name: "Provider-Config"}

	if reqs.RequireVersionConstraint {
		if requiredProvidersRe.MatchString(hcl) {
			versionMatches := versionConstraintRe.FindAllString(hcl, -1)
			if len(versionMatches) >= len(reqs.RequiredProviders) {
				result.Passed = append(result.Passed, "version constraints present for providers")
			} else {
				result.Failures = append(result.Failures,
					fmt.Sprintf("found %d version constraints, need %d", len(versionMatches), len(reqs.RequiredProviders)))
			}
		} else {
			result.Failures = append(result.Failures, "required_providers block not found")
		}
	}

	if reqs.RequireImpersonation {
		impersonateMatches := impersonateRe.FindAllStringSubmatch(hcl, -1)
		if len(impersonateMatches) > 0 {
			result.Passed = append(result.Passed, "SA impersonation configured")
			if reqs.ImpersonationSA != "" {
				found := false
				for _, m := range impersonateMatches {
					if len(m) > 1 && m[1] == reqs.ImpersonationSA {
						found = true
					}
				}
				if found {
					result.Passed = append(result.Passed, fmt.Sprintf("impersonation SA matches %q", reqs.ImpersonationSA))
				} else {
					result.Failures = append(result.Failures,
						fmt.Sprintf("impersonation SA does not match expected %q", reqs.ImpersonationSA))
				}
			}
		} else {
			result.Failures = append(result.Failures, "impersonate_service_account not configured")
		}
	}

	providerMatches := providerBlockRe.FindAllStringSubmatch(hcl, -1)
	foundProviders := make(map[string]bool)
	for _, m := range providerMatches {
		if len(m) > 1 {
			foundProviders[m[1]] = true
		}
	}

	for _, p := range reqs.RequiredProviders {
		if foundProviders[p] {
			result.Passed = append(result.Passed, fmt.Sprintf("provider %q block found", p))
		} else {
			result.Failures = append(result.Failures, fmt.Sprintf("provider %q block missing", p))
		}
	}

	return result, nil
}

// ValidateBackendConfig validates the Terraform backend configuration.
func ValidateBackendConfig(hcl string, reqs BackendRequirements) (*ValidationResult, error) {
	result := &ValidationResult{Name: "Backend-Config"}

	backendMatches := backendBlockRe.FindStringSubmatch(hcl)
	if len(backendMatches) < 2 {
		result.Failures = append(result.Failures, "no backend block found")
		return result, nil
	}

	if backendMatches[1] != reqs.Type {
		result.Failures = append(result.Failures,
			fmt.Sprintf("backend type: want %q, got %q", reqs.Type, backendMatches[1]))
		return result, nil
	}
	result.Passed = append(result.Passed, fmt.Sprintf("backend type %q", reqs.Type))

	if reqs.BucketName != "" {
		bucketMatches := bucketRe.FindStringSubmatch(hcl)
		if len(bucketMatches) > 1 && bucketMatches[1] == reqs.BucketName {
			result.Passed = append(result.Passed, fmt.Sprintf("bucket %q", reqs.BucketName))
		} else if len(bucketMatches) > 1 {
			result.Failures = append(result.Failures,
				fmt.Sprintf("bucket: want %q, got %q", reqs.BucketName, bucketMatches[1]))
		} else {
			result.Failures = append(result.Failures, "bucket name not specified")
		}
	}

	if reqs.Prefix != "" {
		prefixMatches := prefixRe.FindStringSubmatch(hcl)
		if len(prefixMatches) > 1 && prefixMatches[1] == reqs.Prefix {
			result.Passed = append(result.Passed, fmt.Sprintf("prefix %q", reqs.Prefix))
		}
	}

	return result, nil
}
