package monitoringvalidator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// promConfig models the embedded prometheus.yml inside a ConfigMap.
type promConfig struct {
	Global struct {
		ScrapeInterval string `yaml:"scrape_interval"`
	} `yaml:"global"`
	ScrapeConfigs []struct {
		JobName string `yaml:"job_name"`
	} `yaml:"scrape_configs"`
}

// configMap models a K8s ConfigMap.
type configMap struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
}

// ValidatePrometheusConfig validates a Prometheus ConfigMap manifest.
func ValidatePrometheusConfig(data []byte, reqs PrometheusRequirements) (*ValidationResult, error) {
	var cm configMap
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return nil, fmt.Errorf("parsing ConfigMap: %w", err)
	}

	result := &ValidationResult{Name: "Prometheus-ConfigMap"}

	if reqs.Namespace != "" && cm.Metadata.Namespace != reqs.Namespace {
		result.Failures = append(result.Failures,
			fmt.Sprintf("namespace: want %q, got %q", reqs.Namespace, cm.Metadata.Namespace))
	} else {
		result.Passed = append(result.Passed, "namespace OK")
	}

	promYAMLStr, ok := cm.Data["prometheus.yml"]
	if !ok {
		result.Failures = append(result.Failures, "prometheus.yml key missing from ConfigMap data")
		return result, nil
	}

	var pc promConfig
	if err := yaml.Unmarshal([]byte(promYAMLStr), &pc); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("invalid prometheus.yml: %v", err))
		return result, nil
	}

	foundJobs := make(map[string]bool)
	for _, sc := range pc.ScrapeConfigs {
		foundJobs[sc.JobName] = true
	}

	for _, target := range reqs.RequiredScrapeTargets {
		if foundJobs[target] {
			result.Passed = append(result.Passed, "scrape target: "+target)
		} else {
			result.Failures = append(result.Failures, "missing scrape target: "+target)
		}
	}

	return result, nil
}

// ValidateGrafanaDeployment validates a Grafana Deployment manifest for provisioning mounts.
func ValidateGrafanaDeployment(data []byte, reqs GrafanaRequirements) (*ValidationResult, error) {
	var dep struct {
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
						VolumeMounts []struct {
							Name      string `yaml:"name"`
							MountPath string `yaml:"mountPath"`
						} `yaml:"volumeMounts"`
					} `yaml:"containers"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &dep); err != nil {
		return nil, fmt.Errorf("parsing Grafana Deployment: %w", err)
	}

	result := &ValidationResult{Name: "Grafana-Deployment"}

	if reqs.Namespace != "" && dep.Metadata.Namespace != reqs.Namespace {
		result.Failures = append(result.Failures,
			fmt.Sprintf("namespace: want %q, got %q", reqs.Namespace, dep.Metadata.Namespace))
	} else {
		result.Passed = append(result.Passed, "namespace OK")
	}

	hasDashboardMount := false
	hasDataSourceMount := false

	for _, c := range dep.Spec.Template.Spec.Containers {
		for _, vm := range c.VolumeMounts {
			if strings.Contains(vm.MountPath, "dashboard") {
				hasDashboardMount = true
			}
			if strings.Contains(vm.MountPath, "datasource") {
				hasDataSourceMount = true
			}
		}
	}

	if reqs.RequireDashboardProvisioning {
		if hasDashboardMount {
			result.Passed = append(result.Passed, "dashboard provisioning mount found")
		} else {
			result.Failures = append(result.Failures, "dashboard provisioning mount not found")
		}
	}

	if reqs.RequireDataSourceConfig {
		if hasDataSourceMount {
			result.Passed = append(result.Passed, "datasource config mount found")
		} else {
			result.Failures = append(result.Failures, "datasource config mount not found")
		}
	}

	return result, nil
}

var storageSizeRe = regexp.MustCompile(`^(\d+)(Gi|Mi|Ti)$`)

// ValidateRetentionPV validates a PersistentVolumeClaim for Prometheus data retention.
func ValidateRetentionPV(data []byte) (*ValidationResult, error) {
	var pvc struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			AccessModes []string `yaml:"accessModes"`
			Resources   struct {
				Requests struct {
					Storage string `yaml:"storage"`
				} `yaml:"requests"`
			} `yaml:"resources"`
			StorageClassName string `yaml:"storageClassName"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return nil, fmt.Errorf("parsing PVC: %w", err)
	}

	result := &ValidationResult{Name: "Prometheus-RetentionPV"}

	if pvc.Kind != "PersistentVolumeClaim" && pvc.Kind != "" {
		result.Failures = append(result.Failures, fmt.Sprintf("expected PVC, got %s", pvc.Kind))
		return result, nil
	}

	const minStorageGi = 10
	sizeGi := parseStorageGi(pvc.Spec.Resources.Requests.Storage)
	if sizeGi >= minStorageGi {
		result.Passed = append(result.Passed, fmt.Sprintf("storage %s meets minimum %dGi", pvc.Spec.Resources.Requests.Storage, minStorageGi))
	} else {
		result.Failures = append(result.Failures, fmt.Sprintf("storage %s below minimum %dGi", pvc.Spec.Resources.Requests.Storage, minStorageGi))
	}

	hasRWO := false
	for _, am := range pvc.Spec.AccessModes {
		if am == "ReadWriteOnce" {
			hasRWO = true
		}
	}
	if hasRWO {
		result.Passed = append(result.Passed, "ReadWriteOnce access mode")
	} else {
		result.Failures = append(result.Failures, "ReadWriteOnce access mode not set")
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
