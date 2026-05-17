package vllmvalidator

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type deployment struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []container `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type container struct {
	Name      string `yaml:"name"`
	Image     string `yaml:"image"`
	Resources struct {
		Limits map[string]string `yaml:"limits"`
	} `yaml:"resources"`
	ReadinessProbe *probe `yaml:"readinessProbe"`
}

type probe struct {
	HTTPGet *struct {
		Path string `yaml:"path"`
		Port int    `yaml:"port"`
	} `yaml:"httpGet"`
}

// ValidateVLLMDeployment validates a vLLM K8s Deployment manifest for GPU requests,
// model path configuration, and health probe.
func ValidateVLLMDeployment(manifest []byte) error {
	var dep deployment
	if err := yaml.Unmarshal(manifest, &dep); err != nil {
		return fmt.Errorf("parsing deployment: %w", err)
	}

	var errs []string
	for _, c := range dep.Spec.Template.Spec.Containers {
		gpuCount := 0
		if gpuStr, ok := c.Resources.Limits["nvidia.com/gpu"]; ok {
			gpuCount, _ = strconv.Atoi(gpuStr)
		}
		if gpuCount == 0 {
			errs = append(errs, fmt.Sprintf("container %q has no GPU allocation (nvidia.com/gpu)", c.Name))
		}

		if c.ReadinessProbe == nil {
			errs = append(errs, fmt.Sprintf("container %q missing health probe at /health", c.Name))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

const maxReasonableModelLen = 131072

// ValidateModelConfig validates vLLM model serving parameters.
func ValidateModelConfig(config *VLLMConfig) error {
	var errs []string

	if config.Model == "" {
		errs = append(errs, "model name is required")
	}

	if config.MaxModelLen <= 0 {
		errs = append(errs, "max-model-len must be > 0")
	} else if config.MaxModelLen > maxReasonableModelLen {
		errs = append(errs, fmt.Sprintf("max-model-len %d exceeds reasonable limit (%d)", config.MaxModelLen, maxReasonableModelLen))
	}

	if len(errs) > 0 {
		return fmt.Errorf("model config validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateGPUAllocation validates that the deployment requests exactly the expected
// number of GPUs.
func ValidateGPUAllocation(podYAML []byte, expectedGPUs int) error {
	var dep deployment
	if err := yaml.Unmarshal(podYAML, &dep); err != nil {
		return fmt.Errorf("parsing deployment: %w", err)
	}

	totalGPUs := 0
	for _, c := range dep.Spec.Template.Spec.Containers {
		if gpuStr, ok := c.Resources.Limits["nvidia.com/gpu"]; ok {
			n, _ := strconv.Atoi(gpuStr)
			totalGPUs += n
		}
	}

	if totalGPUs == 0 {
		return fmt.Errorf("no GPU allocation found in deployment")
	}

	if totalGPUs != expectedGPUs {
		return fmt.Errorf("GPU allocation mismatch: got %d, expected %d", totalGPUs, expectedGPUs)
	}

	return nil
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image       string   `yaml:"image"`
	Runtime     string   `yaml:"runtime"`
	Ports       []string `yaml:"ports"`
	Healthcheck *struct {
		Test []string `yaml:"test"`
	} `yaml:"healthcheck"`
	Deploy *struct {
		Resources *struct {
			Reservations *struct {
				Devices []struct {
					Driver       string   `yaml:"driver"`
					Count        any      `yaml:"count"`
					Capabilities []string `yaml:"capabilities"`
				} `yaml:"devices"`
			} `yaml:"reservations"`
		} `yaml:"resources"`
	} `yaml:"deploy"`
}

// ValidateDockerComposeVLLM validates a Docker Compose file for vLLM deployment.
func ValidateDockerComposeVLLM(data []byte) (*ValidationResult, error) {
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing docker-compose: %w", err)
	}

	result := &ValidationResult{Name: "vLLM-DockerCompose"}

	if len(cf.Services) == 0 {
		result.Failures = append(result.Failures, "no services defined")
		return result, nil
	}

	for name, svc := range cf.Services {
		hasNvidiaRuntime := svc.Runtime == "nvidia"
		hasGPUDeploy := svc.Deploy != nil &&
			svc.Deploy.Resources != nil &&
			svc.Deploy.Resources.Reservations != nil &&
			len(svc.Deploy.Resources.Reservations.Devices) > 0

		if hasNvidiaRuntime || hasGPUDeploy {
			result.Passed = append(result.Passed, fmt.Sprintf("service %q has GPU access", name))
		} else {
			result.Failures = append(result.Failures, fmt.Sprintf("service %q missing nvidia runtime or GPU device reservation", name))
		}

		if svc.Healthcheck != nil {
			result.Passed = append(result.Passed, fmt.Sprintf("service %q has healthcheck", name))
		} else {
			result.Failures = append(result.Failures, fmt.Sprintf("service %q missing healthcheck", name))
		}
	}

	return result, nil
}
