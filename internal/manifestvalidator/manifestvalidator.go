package manifestvalidator

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Manifest describes an installer manifest with a list of distributable components.
type Manifest struct {
	Name        string      `yaml:"name"        json:"name"`
	Version     string      `yaml:"version"     json:"version"`
	Description string      `yaml:"description" json:"description"`
	Components  []Component `yaml:"components"  json:"components"`
}

// Component describes a single distributable unit within a manifest.
type Component struct {
	Name       string `yaml:"name"        json:"name"`
	Type       string `yaml:"type"        json:"type"`
	BinaryPath string `yaml:"binary_path" json:"binary_path"`
	ConfigPath string `yaml:"config_path" json:"config_path"`
	Required   bool   `yaml:"required"    json:"required"`
	Platform   string `yaml:"platform"    json:"platform"`
}

// ValidationResult holds the outcome of manifest validation.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

var validPlatforms = map[string]bool{
	"darwin":  true,
	"linux":   true,
	"windows": true,
	"all":     true,
}

// ParseManifest deserialises a YAML manifest into a Manifest struct.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parsing manifest: %w", err)
	}
	return m, nil
}

// ValidateManifest checks required fields, version format, component
// uniqueness, and delegates per-component validation.
func ValidateManifest(m Manifest) ValidationResult {
	var result ValidationResult

	if m.Name == "" {
		result.Errors = append(result.Errors, "manifest name is required")
	}
	if m.Version == "" {
		result.Errors = append(result.Errors, "manifest version is required")
	} else if !semverRe.MatchString(m.Version) {
		result.Errors = append(result.Errors, fmt.Sprintf("version %q must be semver format (X.Y.Z)", m.Version))
	}

	if len(m.Components) == 0 {
		result.Warnings = append(result.Warnings, "manifest has no components defined")
	}

	seen := make(map[string]bool, len(m.Components))
	for _, c := range m.Components {
		if seen[c.Name] {
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate component name %q", c.Name))
		}
		seen[c.Name] = true
		result.Errors = append(result.Errors, ValidateComponent(c)...)
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateComponent checks an individual component for required fields and
// valid platform values.
func ValidateComponent(c Component) []string {
	var errs []string

	if c.Name == "" {
		errs = append(errs, "component name is required")
	}
	if c.Type == "" {
		errs = append(errs, "component type is required")
	}
	if c.Platform != "" && !validPlatforms[c.Platform] {
		errs = append(errs, fmt.Sprintf("component %q has invalid platform %q; must be darwin, linux, windows, or all", c.Name, c.Platform))
	}

	return errs
}
