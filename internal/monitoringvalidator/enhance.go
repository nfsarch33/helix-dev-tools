package monitoringvalidator

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

type grafanaDashboard struct {
	Title  string           `json:"title"`
	UID    string           `json:"uid"`
	Panels []grafanaPanel   `json:"panels"`
	Schema int              `json:"schemaVersion"`
}

type grafanaPanel struct {
	ID         int              `json:"id"`
	Type       string           `json:"type"`
	Title      string           `json:"title"`
	Targets    []grafanaTarget  `json:"targets"`
	Datasource any              `json:"datasource"`
}

type grafanaTarget struct {
	Expr         string `json:"expr"`
	LegendFormat string `json:"legendFormat"`
}

// ValidateDashboardJSON validates a Grafana dashboard JSON for required structure:
// title, non-empty panels, and datasource references.
func ValidateDashboardJSON(dashboard []byte) (*ValidationResult, error) {
	var db grafanaDashboard
	if err := json.Unmarshal(dashboard, &db); err != nil {
		return nil, fmt.Errorf("parsing Grafana dashboard JSON: %w", err)
	}

	result := &ValidationResult{Name: "Grafana-DashboardJSON"}

	if db.Title == "" {
		result.Failures = append(result.Failures, "dashboard title is empty")
	} else {
		result.Passed = append(result.Passed, "dashboard has title: "+db.Title)
	}

	if len(db.Panels) == 0 {
		result.Failures = append(result.Failures, "dashboard has no panels")
	} else {
		result.Passed = append(result.Passed, fmt.Sprintf("dashboard has %d panels", len(db.Panels)))
	}

	for _, p := range db.Panels {
		if p.Datasource == nil || p.Datasource == "" {
			result.Failures = append(result.Failures, fmt.Sprintf("panel %q (id=%d) missing datasource", p.Title, p.ID))
		}
	}

	if len(result.Failures) == 0 && len(db.Panels) > 0 {
		result.Passed = append(result.Passed, "all panels have datasource configured")
	}

	return result, nil
}

// ValidateScrapeTargets validates a raw Prometheus config (not a ConfigMap) for
// the presence of required scrape job names.
func ValidateScrapeTargets(config []byte, expectedTargets []string) (*ValidationResult, error) {
	var pc promConfig
	if err := yaml.Unmarshal(config, &pc); err != nil {
		return nil, fmt.Errorf("parsing prometheus config: %w", err)
	}

	result := &ValidationResult{Name: "ScrapeTargets"}

	foundJobs := make(map[string]bool)
	for _, sc := range pc.ScrapeConfigs {
		foundJobs[sc.JobName] = true
	}

	for _, target := range expectedTargets {
		if foundJobs[target] {
			result.Passed = append(result.Passed, "scrape target present: "+target)
		} else {
			result.Failures = append(result.Failures, "missing scrape target: "+target)
		}
	}

	return result, nil
}
