package terraformvalidator

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	credentialsFieldRe   = regexp.MustCompile(`(?i)credentials\s*=`)
	googleCredentialsRe  = regexp.MustCompile(`(?i)GOOGLE_CREDENTIALS`)
	accessTokenRe        = regexp.MustCompile(`access_token\s*=\s*"`)
	googleAppCredsRe     = regexp.MustCompile(`(?i)GOOGLE_APPLICATION_CREDENTIALS`)
)

// ValidateGCPProject validates GCP-specific project configuration in HCL.
func ValidateGCPProject(hcl string, reqs GCPProjectRequirements) (*ValidationResult, error) {
	result := &ValidationResult{Name: "GCP-Project"}

	projectMatches := projectRe.FindAllStringSubmatch(hcl, -1)
	if len(projectMatches) > 0 {
		foundCorrect := false
		for _, m := range projectMatches {
			if len(m) > 1 && m[1] == reqs.ProjectID {
				foundCorrect = true
			}
		}
		if foundCorrect {
			result.Passed = append(result.Passed, fmt.Sprintf("project ID %q", reqs.ProjectID))
		} else {
			result.Failures = append(result.Failures,
				fmt.Sprintf("project ID: want %q, got %q", reqs.ProjectID, projectMatches[0][1]))
		}
	} else {
		result.Failures = append(result.Failures, "no project field found")
	}

	regionMatches := regionRe.FindAllStringSubmatch(hcl, -1)
	if len(regionMatches) > 0 {
		foundCorrect := false
		for _, m := range regionMatches {
			if len(m) > 1 && m[1] == reqs.Region {
				foundCorrect = true
			}
		}
		if foundCorrect {
			result.Passed = append(result.Passed, fmt.Sprintf("region %q", reqs.Region))
		} else {
			result.Failures = append(result.Failures,
				fmt.Sprintf("region: want %q, got %q", reqs.Region, regionMatches[0][1]))
		}
	} else {
		result.Failures = append(result.Failures, "no region field found")
	}

	if reqs.StateBucket != "" {
		bucketMatches := bucketRe.FindStringSubmatch(hcl)
		if len(bucketMatches) > 1 {
			if bucketMatches[1] == reqs.StateBucket {
				result.Passed = append(result.Passed, fmt.Sprintf("state bucket %q", reqs.StateBucket))
			} else {
				result.Failures = append(result.Failures,
					fmt.Sprintf("state bucket: want %q, got %q", reqs.StateBucket, bucketMatches[1]))
			}
		} else {
			result.Failures = append(result.Failures, "state bucket not found in backend config")
		}
	}

	if reqs.ForbidJSONKeys {
		jsonKeyIssues := detectJSONKeyReferences(hcl)
		if len(jsonKeyIssues) > 0 {
			result.Failures = append(result.Failures, jsonKeyIssues...)
		} else {
			result.Passed = append(result.Passed, "no JSON key file references detected")
		}
	}

	return result, nil
}

// ValidateNoJSONKeys checks that no JSON key file references exist in HCL.
func ValidateNoJSONKeys(hcl string) (*ValidationResult, error) {
	result := &ValidationResult{Name: "No-JSON-Keys"}

	issues := detectJSONKeyReferences(hcl)
	if len(issues) > 0 {
		result.Failures = append(result.Failures, issues...)
	} else {
		result.Passed = append(result.Passed, "no JSON key references found")
	}

	return result, nil
}

func detectJSONKeyReferences(hcl string) []string {
	var issues []string

	lines := strings.Split(hcl, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if credentialsFieldRe.MatchString(trimmed) {
			issues = append(issues, fmt.Sprintf("line %d: credentials field detected (use ADC or impersonation)", i+1))
		}

		if accessTokenRe.MatchString(trimmed) {
			issues = append(issues, fmt.Sprintf("line %d: hardcoded access_token detected", i+1))
		}

		if googleCredentialsRe.MatchString(trimmed) {
			issues = append(issues, fmt.Sprintf("line %d: GOOGLE_CREDENTIALS env reference (use ADC)", i+1))
		}

		if googleAppCredsRe.MatchString(trimmed) {
			issues = append(issues, fmt.Sprintf("line %d: GOOGLE_APPLICATION_CREDENTIALS reference (use ADC)", i+1))
		}
	}

	return issues
}
