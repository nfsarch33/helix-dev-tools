package ansiblevalidator

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var defaultRequiredGroups = []string{"fleet_linux", "fleet_windows", "wsl_fleet"}

// ValidateInventoryYAML validates an Ansible inventory YAML byte slice
// against the fleet-required structure and host field rules.
func ValidateInventoryYAML(data []byte) ValidationResult {
	return ValidateInventoryWithGroups(data, defaultRequiredGroups)
}

// ValidateInventoryWithGroups validates inventory YAML against custom
// required group names.
func ValidateInventoryWithGroups(data []byte, requiredGroups []string) ValidationResult {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ValidationResult{Valid: false, Errors: []string{
			fmt.Sprintf("invalid YAML: %v", err),
		}}
	}

	var errs []string

	allNode, ok := raw["all"]
	if !ok {
		return ValidationResult{Valid: false, Errors: []string{"missing top-level 'all' key"}}
	}

	allMap, ok := toMap(allNode)
	if !ok {
		return ValidationResult{Valid: false, Errors: []string{"'all' is not a mapping"}}
	}

	childrenNode, ok := allMap["children"]
	if !ok {
		return ValidationResult{Valid: false, Errors: []string{"missing 'all.children' key"}}
	}

	childrenMap, ok := toMap(childrenNode)
	if !ok {
		return ValidationResult{Valid: false, Errors: []string{"'all.children' is not a mapping"}}
	}

	for _, g := range requiredGroups {
		if _, exists := childrenMap[g]; !exists {
			errs = append(errs, fmt.Sprintf("required group %q missing from inventory", g))
		}
	}

	for groupName, groupNode := range childrenMap {
		groupMap, ok := toMap(groupNode)
		if !ok {
			continue
		}
		hostsNode, ok := groupMap["hosts"]
		if !ok {
			continue
		}
		hostsMap, ok := toMap(hostsNode)
		if !ok {
			continue
		}
		for hostName, hostNode := range hostsMap {
			hostVars, ok := toMap(hostNode)
			if !ok {
				continue
			}
			if _, hasAH := hostVars["ansible_host"]; !hasAH {
				errs = append(errs, fmt.Sprintf(
					"host %q in group %q missing ansible_host", hostName, groupName,
				))
			}
		}
	}

	return ValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	}
}

// ValidatePlaybookYAML validates an Ansible playbook YAML byte slice
// for required structural fields.
func ValidatePlaybookYAML(data []byte) ValidationResult {
	var plays []map[string]interface{}
	if err := yaml.Unmarshal(data, &plays); err != nil {
		return ValidationResult{Valid: false, Errors: []string{
			fmt.Sprintf("invalid playbook YAML: %v", err),
		}}
	}

	var errs []string

	if len(plays) == 0 {
		errs = append(errs, "playbook has no plays")
		return ValidationResult{Valid: false, Errors: errs}
	}

	for i, play := range plays {
		playLabel := fmt.Sprintf("play[%d]", i)
		if name, ok := play["name"].(string); ok && name != "" {
			playLabel = fmt.Sprintf("play %q", name)
		}

		if _, ok := play["hosts"]; !ok {
			errs = append(errs, fmt.Sprintf("%s: missing 'hosts' field", playLabel))
		}

		tasksRaw, ok := play["tasks"]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: missing 'tasks' field", playLabel))
			continue
		}

		tasks, ok := toSlice(tasksRaw)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: 'tasks' is not a list", playLabel))
			continue
		}

		for j, taskRaw := range tasks {
			task, ok := toMap(taskRaw)
			if !ok {
				continue
			}
			if _, hasName := task["name"]; !hasName {
				errs = append(errs, fmt.Sprintf(
					"%s: task[%d] missing 'name' field", playLabel, j,
				))
			}
		}
	}

	return ValidationResult{Valid: len(errs) == 0, Errors: errs}
}

// ValidatePlaybookFile reads a playbook file from disk and validates it.
func ValidatePlaybookFile(path string) ValidationResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{Valid: false, Errors: []string{
			fmt.Sprintf("cannot read file %q: %v", path, err),
		}}
	}
	return ValidatePlaybookYAML(data)
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(m))
		for k, val := range m {
			result[fmt.Sprint(k)] = val
		}
		return result, true
	default:
		return nil, false
	}
}

func toSlice(v interface{}) ([]interface{}, bool) {
	s, ok := v.([]interface{})
	return s, ok
}

// String returns a human-readable summary of validation errors.
func (r ValidationResult) String() string {
	if r.Valid {
		return "valid"
	}
	return fmt.Sprintf("invalid: %s", strings.Join(r.Errors, "; "))
}
