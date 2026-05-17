package devopsvalidator

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseAgentConfig parses YAML bytes into an AgentConfig.
func ParseAgentConfig(configYAML []byte) (*AgentConfig, error) {
	if len(strings.TrimSpace(string(configYAML))) == 0 {
		return nil, fmt.Errorf("empty agent config")
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(configYAML, &cfg); err != nil {
		return nil, fmt.Errorf("parsing agent config: %w", err)
	}

	if cfg.Agent.Name == "" {
		return nil, fmt.Errorf("agent name is required")
	}

	return &cfg, nil
}

// ValidateLLMRouting verifies the agent has a valid tiered LLM routing config.
// At minimum, a "local" tier must be present for heartbeat operations.
func ValidateLLMRouting(config *AgentConfig) error {
	tiers := config.Agent.LLMRouting.Tiers
	if len(tiers) == 0 {
		return fmt.Errorf("no LLM routing tiers defined")
	}

	hasLocal := false
	var errs []string

	for _, tier := range tiers {
		if tier.Name == "local" {
			hasLocal = true
		}
		if tier.Endpoint == "" {
			errs = append(errs, fmt.Sprintf("tier %q missing endpoint", tier.Name))
		}
		if tier.Model == "" {
			errs = append(errs, fmt.Sprintf("tier %q missing model", tier.Name))
		}
	}

	if !hasLocal {
		errs = append(errs, "no local tier defined; required for heartbeat operations")
	}

	if len(errs) > 0 {
		return fmt.Errorf("LLM routing validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateMemoryConfig verifies the agent's memory backend configuration.
func ValidateMemoryConfig(config *AgentConfig) error {
	mem := config.Agent.Memory

	var errs []string
	if mem.Endpoint == "" {
		errs = append(errs, "memory endpoint is required")
	}
	if mem.AppID == "" {
		errs = append(errs, "memory app_id is required")
	}
	if mem.Provider == "" {
		errs = append(errs, "memory provider is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("memory config validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateHeartbeat verifies the heartbeat configuration has required fields.
func ValidateHeartbeat(config *AgentConfig) error {
	hb := config.Agent.Heartbeat

	var errs []string
	if hb.Interval == "" {
		errs = append(errs, "heartbeat interval is required")
	}
	if hb.LLMTier == "" {
		errs = append(errs, "heartbeat llm_tier is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("heartbeat validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ValidateResourceLimits validates that a DaemonSet/Deployment pod spec stays within
// the fleet budget: <=512Mi RAM, <=0.5 CPU.
func ValidateResourceLimits(podYAML []byte) error {
	var workload struct {
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Name      string `yaml:"name"`
						Resources struct {
							Limits map[string]string `yaml:"limits"`
						} `yaml:"resources"`
					} `yaml:"containers"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal(podYAML, &workload); err != nil {
		return fmt.Errorf("parsing workload: %w", err)
	}

	var errs []string
	for _, c := range workload.Spec.Template.Spec.Containers {
		if len(c.Resources.Limits) == 0 {
			errs = append(errs, fmt.Sprintf("container %q has no resource limits", c.Name))
			continue
		}

		if memStr, ok := c.Resources.Limits["memory"]; ok {
			memMi := parseMemoryMi(memStr)
			if memMi > 512 {
				errs = append(errs, fmt.Sprintf("container %q memory limit %s exceeds 512Mi budget", c.Name, memStr))
			}
		}

		if cpuStr, ok := c.Resources.Limits["cpu"]; ok {
			cpuMillis := parseCPUMillis(cpuStr)
			if cpuMillis > 500 {
				errs = append(errs, fmt.Sprintf("container %q cpu limit %s exceeds 500m (0.5 CPU) budget", c.Name, cpuStr))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("resource limits exceeded: %s", strings.Join(errs, "; "))
	}
	return nil
}

func parseMemoryMi(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "Gi") {
		val, _ := strconv.Atoi(strings.TrimSuffix(s, "Gi"))
		return val * 1024
	}
	if strings.HasSuffix(s, "Mi") {
		val, _ := strconv.Atoi(strings.TrimSuffix(s, "Mi"))
		return val
	}
	if strings.HasSuffix(s, "Ki") {
		val, _ := strconv.Atoi(strings.TrimSuffix(s, "Ki"))
		return val / 1024
	}
	val, _ := strconv.Atoi(s)
	return val / (1024 * 1024)
}

func parseCPUMillis(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "m") {
		val, _ := strconv.Atoi(strings.TrimSuffix(s, "m"))
		return val
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		whole, _ := strconv.Atoi(s)
		return whole * 1000
	}
	return int(val * 1000)
}
