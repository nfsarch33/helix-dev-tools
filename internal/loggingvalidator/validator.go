package loggingvalidator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var storageSizeRe = regexp.MustCompile(`^(\d+)(Gi|Mi|Ti)$`)

type statefulSet struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		ServiceName string `yaml:"serviceName"`
		Template    struct {
			Spec struct {
				Containers []struct {
					Name         string `yaml:"name"`
					Image        string `yaml:"image"`
					VolumeMounts []struct {
						Name      string `yaml:"name"`
						MountPath string `yaml:"mountPath"`
					} `yaml:"volumeMounts"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
		VolumeClaimTemplates []struct {
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Spec struct {
				AccessModes []string `yaml:"accessModes"`
				Resources   struct {
					Requests struct {
						Storage string `yaml:"storage"`
					} `yaml:"requests"`
				} `yaml:"resources"`
			} `yaml:"spec"`
		} `yaml:"volumeClaimTemplates"`
	} `yaml:"spec"`
}

// ValidateLokiStatefulSet validates a Loki StatefulSet manifest.
func ValidateLokiStatefulSet(data []byte, reqs LokiRequirements) (*ValidationResult, error) {
	var ss statefulSet
	if err := yaml.Unmarshal(data, &ss); err != nil {
		return nil, fmt.Errorf("parsing StatefulSet: %w", err)
	}

	result := &ValidationResult{Name: "Loki-StatefulSet"}

	if ss.Kind != "StatefulSet" {
		result.Failures = append(result.Failures, fmt.Sprintf("expected StatefulSet, got %s", ss.Kind))
		return result, nil
	}

	if reqs.Namespace != "" && ss.Metadata.Namespace != reqs.Namespace {
		result.Failures = append(result.Failures,
			fmt.Sprintf("namespace: want %q, got %q", reqs.Namespace, ss.Metadata.Namespace))
	} else {
		result.Passed = append(result.Passed, "namespace OK")
	}

	if reqs.RequirePersistence {
		if len(ss.Spec.VolumeClaimTemplates) == 0 {
			result.Failures = append(result.Failures, "no volumeClaimTemplates for persistent storage")
		} else {
			result.Passed = append(result.Passed, "persistent volume claim template present")
			vct := ss.Spec.VolumeClaimTemplates[0]
			sizeGi := parseStorageGi(vct.Spec.Resources.Requests.Storage)
			if sizeGi >= reqs.MinStorageGi {
				result.Passed = append(result.Passed,
					fmt.Sprintf("storage %s meets minimum %dGi", vct.Spec.Resources.Requests.Storage, reqs.MinStorageGi))
			} else {
				result.Failures = append(result.Failures,
					fmt.Sprintf("storage %s below minimum %dGi", vct.Spec.Resources.Requests.Storage, reqs.MinStorageGi))
			}
		}
	}

	return result, nil
}

type daemonSet struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name         string `yaml:"name"`
					Image        string `yaml:"image"`
					VolumeMounts []struct {
						Name      string `yaml:"name"`
						MountPath string `yaml:"mountPath"`
					} `yaml:"volumeMounts"`
					Env []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
				} `yaml:"containers"`
				Volumes []struct {
					Name     string `yaml:"name"`
					HostPath *struct {
						Path string `yaml:"path"`
					} `yaml:"hostPath,omitempty"`
				} `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// ValidatePromtailDaemonSet validates a Promtail DaemonSet manifest.
func ValidatePromtailDaemonSet(data []byte, reqs PromtailRequirements) (*ValidationResult, error) {
	var ds daemonSet
	if err := yaml.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parsing DaemonSet: %w", err)
	}

	result := &ValidationResult{Name: "Promtail-DaemonSet"}

	if ds.Kind != "DaemonSet" {
		result.Failures = append(result.Failures, fmt.Sprintf("expected DaemonSet, got %s", ds.Kind))
		return result, nil
	}

	if reqs.Namespace != "" && ds.Metadata.Namespace != reqs.Namespace {
		result.Failures = append(result.Failures,
			fmt.Sprintf("namespace: want %q, got %q", reqs.Namespace, ds.Metadata.Namespace))
	} else {
		result.Passed = append(result.Passed, "namespace OK")
	}

	hasHostLog := false
	hasPodLog := false
	hasLokiEndpoint := false

	for _, c := range ds.Spec.Template.Spec.Containers {
		for _, vm := range c.VolumeMounts {
			if vm.MountPath == "/var/log" {
				hasHostLog = true
			}
			if vm.MountPath == "/var/log/pods" {
				hasPodLog = true
			}
		}
		for _, env := range c.Env {
			if env.Name == "LOKI_ENDPOINT" && strings.Contains(env.Value, "loki") {
				hasLokiEndpoint = true
			}
		}
	}

	if reqs.RequireHostLogMount {
		if hasHostLog {
			result.Passed = append(result.Passed, "/var/log host mount present")
		} else {
			result.Failures = append(result.Failures, "/var/log host mount not found")
		}
	}

	if reqs.RequirePodLogMount {
		if hasPodLog {
			result.Passed = append(result.Passed, "/var/log/pods mount present")
		} else {
			result.Failures = append(result.Failures, "/var/log/pods mount not found")
		}
	}

	if reqs.RequireLokiEndpoint {
		if hasLokiEndpoint {
			result.Passed = append(result.Passed, "Loki endpoint configured")
		} else {
			result.Failures = append(result.Failures, "Loki endpoint not configured")
		}
	}

	return result, nil
}

type lokiConfig struct {
	LimitsConfig struct {
		RetentionPeriod string `yaml:"retention_period"`
	} `yaml:"limits_config"`
	Compactor struct {
		RetentionEnabled    bool   `yaml:"retention_enabled"`
		RetentionDeleteDelay string `yaml:"retention_delete_delay"`
	} `yaml:"compactor"`
}

// ValidateLogRetention validates Loki retention configuration.
func ValidateLogRetention(data []byte, reqs LogRetentionRequirements) (*ValidationResult, error) {
	var cfg lokiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing Loki config: %w", err)
	}

	result := &ValidationResult{Name: "Log-Retention"}

	hours := parseHours(cfg.LimitsConfig.RetentionPeriod)
	days := hours / 24

	if days < reqs.MinRetentionDays {
		result.Failures = append(result.Failures,
			fmt.Sprintf("retention %s (%d days) below minimum %d days", cfg.LimitsConfig.RetentionPeriod, days, reqs.MinRetentionDays))
	} else if days > reqs.MaxRetentionDays {
		result.Failures = append(result.Failures,
			fmt.Sprintf("retention %s (%d days) above maximum %d days", cfg.LimitsConfig.RetentionPeriod, days, reqs.MaxRetentionDays))
	} else {
		result.Passed = append(result.Passed,
			fmt.Sprintf("retention %s (%d days) within %d-%d day range", cfg.LimitsConfig.RetentionPeriod, days, reqs.MinRetentionDays, reqs.MaxRetentionDays))
	}

	if reqs.RequireCompaction {
		if cfg.Compactor.RetentionEnabled {
			result.Passed = append(result.Passed, "compactor retention enabled")
		} else {
			result.Failures = append(result.Failures, "compactor retention_enabled not set")
		}
	}

	return result, nil
}

// ValidateNamespaceIsolation validates that a Namespace manifest matches expectations.
func ValidateNamespaceIsolation(data []byte, expectedName string) (*ValidationResult, error) {
	var ns struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name   string            `yaml:"name"`
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("parsing Namespace: %w", err)
	}

	result := &ValidationResult{Name: "Namespace-Isolation"}

	if ns.Kind != "Namespace" {
		result.Failures = append(result.Failures, fmt.Sprintf("expected Namespace, got %s", ns.Kind))
		return result, nil
	}

	if ns.Metadata.Name != expectedName {
		result.Failures = append(result.Failures,
			fmt.Sprintf("namespace name: want %q, got %q", expectedName, ns.Metadata.Name))
	} else {
		result.Passed = append(result.Passed, "namespace name OK")
	}

	return result, nil
}

func parseStorageGi(s string) int {
	matches := storageSizeRe.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return 0
	}
	val, _ := strconv.Atoi(matches[1])
	switch matches[2] {
	case "Ti":
		return val * 1024
	case "Gi":
		return val
	case "Mi":
		return val / 1024
	default:
		return 0
	}
}

func parseHours(s string) int {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "h") {
		return 0
	}
	val, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
	if err != nil {
		return 0
	}
	return val
}
