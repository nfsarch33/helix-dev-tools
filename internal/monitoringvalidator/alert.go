package monitoringvalidator

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// alertRuleFile models a Prometheus alert rules file.
type alertRuleFile struct {
	Groups []alertGroup `yaml:"groups"`
}

type alertGroup struct {
	Name  string      `yaml:"name"`
	Rules []alertRule `yaml:"rules"`
}

type alertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// ValidateAlertRules validates Prometheus alert rule definitions.
func ValidateAlertRules(data []byte, reqs AlertRuleRequirements) (*ValidationResult, error) {
	var rf alertRuleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing alert rules: %w", err)
	}

	result := &ValidationResult{Name: "AlertRules"}

	if len(rf.Groups) == 0 {
		result.Failures = append(result.Failures, "no alert groups defined")
		return result, nil
	}

	groupNames := make(map[string]bool)
	for _, g := range rf.Groups {
		groupNames[g.Name] = true
	}

	for _, required := range reqs.RequiredAlertGroups {
		if groupNames[required] {
			result.Passed = append(result.Passed, "alert group present: "+required)
		} else {
			result.Failures = append(result.Failures, "missing alert group: "+required)
		}
	}

	for _, g := range rf.Groups {
		if len(g.Rules) == 0 {
			result.Failures = append(result.Failures, fmt.Sprintf("group %q has no rules", g.Name))
			continue
		}
		for _, r := range g.Rules {
			for _, requiredLabel := range reqs.RequireLabels {
				if _, ok := r.Labels[requiredLabel]; !ok {
					result.Failures = append(result.Failures,
						fmt.Sprintf("alert %q in group %q missing label %q", r.Alert, g.Name, requiredLabel))
				}
			}
		}
	}

	if len(result.Failures) == 0 {
		result.Passed = append(result.Passed, "all alerts have required labels")
	}

	return result, nil
}
