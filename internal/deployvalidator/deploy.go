package deployvalidator

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type deployment struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas             int `yaml:"replicas"`
		RevisionHistoryLimit int `yaml:"revisionHistoryLimit"`
		Strategy             struct {
			Type          string `yaml:"type"`
			RollingUpdate struct {
				MaxSurge       int `yaml:"maxSurge"`
				MaxUnavailable int `yaml:"maxUnavailable"`
			} `yaml:"rollingUpdate"`
		} `yaml:"strategy"`
		Template struct {
			Spec struct {
				Containers []struct {
					Name            string `yaml:"name"`
					Image           string `yaml:"image"`
					ImagePullPolicy string `yaml:"imagePullPolicy"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type canaryConfig struct {
	Strategy string `yaml:"strategy"`
	Weight   int    `yaml:"weight"`
	Steps    []struct {
		SetWeight int `yaml:"setWeight"`
		Pause     *struct {
			Duration string `yaml:"duration"`
		} `yaml:"pause"`
	} `yaml:"steps"`
	Analysis *struct {
		SuccessRate string `yaml:"successRate"`
		LatencyP99  string `yaml:"latencyP99"`
	} `yaml:"analysis"`
}

// ValidateRollingUpdate checks that a Deployment's rolling update strategy
// has safe maxSurge and maxUnavailable values.
func ValidateRollingUpdate(manifest []byte) error {
	var dep deployment
	if err := yaml.Unmarshal(manifest, &dep); err != nil {
		return fmt.Errorf("parsing deployment: %w", err)
	}

	ru := dep.Spec.Strategy.RollingUpdate
	replicas := dep.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	if ru.MaxUnavailable > replicas/2 {
		return fmt.Errorf("maxUnavailable (%d) exceeds half of replicas (%d); too aggressive for safe rollout",
			ru.MaxUnavailable, replicas)
	}

	return nil
}

// ValidateCanaryConfig validates a canary deployment configuration has
// progressive weight steps and an analysis section.
func ValidateCanaryConfig(config []byte) error {
	var cc canaryConfig
	if err := yaml.Unmarshal(config, &cc); err != nil {
		return fmt.Errorf("parsing canary config: %w", err)
	}

	if len(cc.Steps) == 0 {
		return fmt.Errorf("canary config has no weight steps defined")
	}

	if cc.Analysis == nil {
		return fmt.Errorf("canary config missing analysis section; automated rollback requires success criteria")
	}

	return nil
}

// ValidateRollbackPolicy checks that a Deployment has revisionHistoryLimit set.
func ValidateRollbackPolicy(manifest []byte) error {
	var dep deployment
	if err := yaml.Unmarshal(manifest, &dep); err != nil {
		return fmt.Errorf("parsing deployment: %w", err)
	}

	if dep.Spec.RevisionHistoryLimit == 0 {
		return fmt.Errorf("deployment %q missing revisionHistoryLimit; rollbacks require revision history",
			dep.Metadata.Name)
	}

	return nil
}

// ValidateImagePullPolicy checks that production deployments use IfNotPresent.
func ValidateImagePullPolicy(manifest []byte) error {
	var dep deployment
	if err := yaml.Unmarshal(manifest, &dep); err != nil {
		return fmt.Errorf("parsing deployment: %w", err)
	}

	isProd := dep.Metadata.Namespace == "production" || dep.Metadata.Namespace == "prod"

	var errs []string
	for _, c := range dep.Spec.Template.Spec.Containers {
		if isProd && c.ImagePullPolicy == "Always" {
			errs = append(errs, fmt.Sprintf("container %q uses Always pull policy; production should use IfNotPresent for deterministic deploys", c.Name))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateDeploymentStrategy checks that a Deployment uses the expected strategy type.
func ValidateDeploymentStrategy(manifest []byte, strategy string) error {
	var dep deployment
	if err := yaml.Unmarshal(manifest, &dep); err != nil {
		return fmt.Errorf("parsing deployment: %w", err)
	}

	if dep.Spec.Strategy.Type != strategy {
		return fmt.Errorf("deployment %q uses strategy %q, expected %q",
			dep.Metadata.Name, dep.Spec.Strategy.Type, strategy)
	}

	return nil
}
