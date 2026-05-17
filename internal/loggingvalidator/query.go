package loggingvalidator

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	streamSelectorRe = regexp.MustCompile(`\{[^}]+\}`)
	labelRe          = regexp.MustCompile(`(\w+)\s*[=!~]+`)
)

// ValidateLogQLSyntax performs structural validation on a LogQL query string.
func ValidateLogQLSyntax(query string) (*ValidationResult, error) {
	result := &ValidationResult{Name: "LogQL-Syntax"}
	query = strings.TrimSpace(query)

	if query == "" {
		result.Failures = append(result.Failures, "empty query")
		return result, nil
	}

	if !streamSelectorRe.MatchString(query) {
		result.Failures = append(result.Failures, "no stream selector {…} found")
		return result, nil
	}

	if strings.Count(query, "{") != strings.Count(query, "}") {
		result.Failures = append(result.Failures, "unbalanced braces in stream selector")
		return result, nil
	}

	if strings.Count(query, "(") != strings.Count(query, ")") {
		result.Failures = append(result.Failures, "unbalanced parentheses")
		return result, nil
	}

	if strings.Count(query, "[") != strings.Count(query, "]") {
		result.Failures = append(result.Failures, "unbalanced brackets in range vector")
		return result, nil
	}

	result.Passed = append(result.Passed, "stream selector present")
	result.Passed = append(result.Passed, "balanced delimiters")

	return result, nil
}

// ValidateLabelMatching checks that a LogQL query references all required labels.
func ValidateLabelMatching(query string, reqs LogQueryRequirements) (*ValidationResult, error) {
	result := &ValidationResult{Name: "LogQL-LabelMatching"}

	match := streamSelectorRe.FindString(query)
	if match == "" {
		result.Failures = append(result.Failures, "no stream selector found for label check")
		return result, nil
	}

	labelMatches := labelRe.FindAllStringSubmatch(match, -1)
	foundLabels := make(map[string]bool)
	for _, m := range labelMatches {
		if len(m) > 1 {
			foundLabels[m[1]] = true
		}
	}

	for _, required := range reqs.RequiredLabels {
		if foundLabels[required] {
			result.Passed = append(result.Passed, fmt.Sprintf("required label %q present", required))
		} else {
			result.Failures = append(result.Failures, fmt.Sprintf("required label %q missing", required))
		}
	}

	return result, nil
}

// ValidateRetentionCompliance checks that Loki config meets retention policy boundaries.
func ValidateRetentionCompliance(data []byte, reqs LogRetentionRequirements) (*ValidationResult, error) {
	var cfg lokiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing Loki config: %w", err)
	}

	result := &ValidationResult{Name: "Retention-Compliance"}

	if !cfg.Compactor.RetentionEnabled {
		result.Failures = append(result.Failures, "retention not enabled in compactor")
		return result, nil
	}
	result.Passed = append(result.Passed, "retention enabled")

	hours := parseHours(cfg.LimitsConfig.RetentionPeriod)
	days := hours / 24

	if days < reqs.MinRetentionDays {
		result.Failures = append(result.Failures,
			fmt.Sprintf("retention %d days below minimum %d", days, reqs.MinRetentionDays))
	} else if days > reqs.MaxRetentionDays {
		result.Failures = append(result.Failures,
			fmt.Sprintf("retention %d days exceeds maximum %d", days, reqs.MaxRetentionDays))
	} else {
		result.Passed = append(result.Passed,
			fmt.Sprintf("retention %d days within policy (%d-%d)", days, reqs.MinRetentionDays, reqs.MaxRetentionDays))
	}

	return result, nil
}
