package helmvalidator

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Chart contains the Helm Chart.yaml fields needed by Helixon validators.
type Chart struct {
	APIVersion  string `yaml:"apiVersion" json:"api_version"`
	Name        string `yaml:"name"       json:"name"`
	Description string `yaml:"description" json:"description"`
	Type        string `yaml:"type"       json:"type"`
	Version     string `yaml:"version"    json:"version"`
	AppVersion  string `yaml:"appVersion" json:"app_version"`
}

// Result describes whether a chart satisfies the minimum release contract.
type Result struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

var semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// ParseChart decodes a Helm Chart.yaml document.
func ParseChart(data []byte) (Chart, error) {
	var chart Chart
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return Chart{}, fmt.Errorf("parsing chart: %w", err)
	}
	return chart, nil
}

// ValidateChart checks required Chart.yaml fields and semver chart versions.
func ValidateChart(chart Chart) Result {
	var result Result
	if chart.APIVersion == "" {
		result.Errors = append(result.Errors, "apiVersion is required")
	}
	if chart.Name == "" {
		result.Errors = append(result.Errors, "name is required")
	}
	if chart.Version == "" {
		result.Errors = append(result.Errors, "version is required")
	} else if !semver.MatchString(chart.Version) {
		result.Errors = append(result.Errors, "version must use semver format X.Y.Z")
	}
	result.Valid = len(result.Errors) == 0
	return result
}
