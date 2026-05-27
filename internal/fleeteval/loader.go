package fleeteval

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadTaskFile reads and parses a YAML task definition file.
func LoadTaskFile(path string) (*TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseTaskFile(data)
}

// ParseTaskFile parses YAML bytes into a TaskFile.
func ParseTaskFile(data []byte) (*TaskFile, error) {
	var tf TaskFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if len(tf.Tasks) == 0 {
		return nil, fmt.Errorf("no tasks defined")
	}
	for i, t := range tf.Tasks {
		if t.ID == "" {
			return nil, fmt.Errorf("task[%d] missing id", i)
		}
		if t.ExpectedPattern == "" {
			return nil, fmt.Errorf("task %q missing expected_output_pattern", t.ID)
		}
	}
	return &tf, nil
}
