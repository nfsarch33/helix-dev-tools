package dockerhealth

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseDockerPS parses pipe-delimited docker ps output into a HealthCheckResult.
// Expected format per line: ID|NAME|IMAGE|STATUS|STATE|RESTART_POLICY|RESTART_COUNT
func ParseDockerPS(output, host string) HealthCheckResult {
	result := HealthCheckResult{Host: host}
	if strings.TrimSpace(output) == "" {
		return result
	}

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 7)
		if len(parts) < 7 {
			result.Errors = append(result.Errors, fmt.Sprintf("malformed line: %q", line))
			continue
		}

		restartCount, _ := strconv.Atoi(strings.TrimSpace(parts[6]))
		status := strings.TrimSpace(parts[3])
		state := strings.TrimSpace(parts[4])

		cs := ContainerState{
			Name:          strings.TrimSpace(parts[1]),
			Image:         strings.TrimSpace(parts[2]),
			Status:        status,
			Health:        extractHealth(status),
			RestartPolicy: strings.TrimSpace(parts[5]),
			Running:       state == "running",
			RestartCount:  restartCount,
		}

		result.Containers = append(result.Containers, cs)
		result.Total++

		if cs.Running && cs.Health != "unhealthy" {
			result.Healthy++
		} else {
			result.Unhealthy++
		}
	}

	return result
}

// ValidateRestartPolicies checks that all containers use the required restart policy.
func ValidateRestartPolicies(containers []ContainerState, requiredPolicy string) []string {
	var errs []string
	for _, c := range containers {
		if c.RestartPolicy != requiredPolicy {
			errs = append(errs, fmt.Sprintf(
				"container %q has restart policy %q, expected %q",
				c.Name, c.RestartPolicy, requiredPolicy,
			))
		}
	}
	return errs
}

// ParseAlertRuleFile parses a Prometheus alert rules YAML file.
func ParseAlertRuleFile(data []byte) (*AlertRuleFile, error) {
	var f AlertRuleFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("invalid alert rule YAML: %w", err)
	}
	return &f, nil
}

// ValidateAlertRule checks that an alert rule has required fields.
func ValidateAlertRule(rule AlertRule) []string {
	var errs []string
	if rule.Alert == "" {
		errs = append(errs, "alert name is empty")
	}
	if rule.Expr == "" {
		errs = append(errs, "expr is empty")
	}
	if rule.For == "" {
		errs = append(errs, "for duration is empty")
	}
	return errs
}

// RemoteDockerHealthCheck runs docker ps on a remote host via runx SSH
// and returns the parsed health result.
func RemoteDockerHealthCheck(alias string) HealthCheckResult {
	script := "export DOCKER_HOST=tcp://127.0.0.1:2375; " +
		"docker ps --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.State}}|{{.Label \"com.docker.compose.service\"}}|{{.RunningFor}}' " +
		"--no-trunc 2>&1 || echo 'DOCKER_ERROR'"

	cmd := exec.Command("runx", "ssh", "exec", "--target", alias, "--", "bash", "-c", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return HealthCheckResult{
			Host:   alias,
			Errors: []string{fmt.Sprintf("ssh exec failed: %v: %s", err, stderr.String())},
		}
	}

	return ParseDockerPS(stdout.String(), alias)
}

func extractHealth(status string) string {
	lower := strings.ToLower(status)
	if strings.Contains(lower, "(healthy)") {
		return "healthy"
	}
	if strings.Contains(lower, "(unhealthy)") {
		return "unhealthy"
	}
	return ""
}
