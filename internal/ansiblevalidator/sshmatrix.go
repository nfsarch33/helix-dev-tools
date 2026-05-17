package ansiblevalidator

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// NewSSHMatrix creates an empty SSH matrix.
func NewSSHMatrix() *SSHMatrix {
	return &SSHMatrix{Routes: make(map[string]SSHRoute)}
}

// AddRoute adds a named route to the matrix.
func (m *SSHMatrix) AddRoute(name string, route SSHRoute) {
	m.Routes[name] = route
}

// NewDefaultSSHMatrix returns the fleet SSH matrix with generic route names.
// Integration tests provide actual aliases via env vars.
func NewDefaultSSHMatrix() *SSHMatrix {
	m := NewSSHMatrix()
	m.AddRoute("control-node", SSHRoute{
		Alias:       "your-control-node",
		Description: "primary Ansible control node",
		Port:        2233,
	})
	m.AddRoute("linux-host-2", SSHRoute{
		Alias:       "your-linux-host-2",
		Description: "second Linux host",
		Port:        2233,
	})
	m.AddRoute("windows-host-2", SSHRoute{
		Alias:       "your-windows-host-2",
		Description: "Windows host via jump",
		Port:        22,
		Fallback: &SSHRoute{
			Alias:       "your-windows-host-2-wslexe",
			Description: "wsl.exe relay fallback",
		},
	})
	return m
}

// ValidateSSHMatrix checks that all required route names exist in the matrix.
func ValidateSSHMatrix(matrix *SSHMatrix, requiredNames []string) ValidationResult {
	var errs []string
	for _, name := range requiredNames {
		if _, ok := matrix.Routes[name]; !ok {
			errs = append(errs, fmt.Sprintf("required SSH route %q missing from matrix", name))
		}
	}
	return ValidationResult{Valid: len(errs) == 0, Errors: errs}
}

// ProbeSSHMatrix runs SSHCanary against every route in the matrix.
func ProbeSSHMatrix(matrix *SSHMatrix) SSHMatrixProbeResult {
	results := make(map[string]SSHCanaryResult, len(matrix.Routes))
	reachable := 0

	for name, route := range matrix.Routes {
		result := SSHCanary(route.Alias)
		if !result.Reachable && route.Fallback != nil {
			result = SSHCanary(route.Fallback.Alias)
		}
		results[name] = result
		if result.Reachable {
			reachable++
		}
	}

	return SSHMatrixProbeResult{
		Results:   results,
		Reachable: reachable,
		Total:     len(matrix.Routes),
	}
}

// SimulateWslExeFallback creates a WslExeFallbackResult for unit testing
// without requiring actual SSH access.
func SimulateWslExeFallback(hostAlias, distro, command string) WslExeFallbackResult {
	return WslExeFallbackResult{
		HostAlias: hostAlias,
		WslDistro: distro,
		Reachable: false,
		Output:    "",
		Error:     "simulated (no actual SSH connection)",
	}
}

// WslExeFallbackProbe runs a command on a WSL distro inside a Windows host
// using the wsl.exe relay pattern via runx SSH.
func WslExeFallbackProbe(winAlias, distro string) WslExeFallbackResult {
	script := fmt.Sprintf("wsl.exe -d %s -e bash -c 'hostname && uptime'", distro)
	cmd := exec.Command("runx", "ssh", "exec", "--target", winAlias, "--", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())

	if err != nil {
		return WslExeFallbackResult{
			HostAlias: winAlias,
			WslDistro: distro,
			Reachable: false,
			Output:    out,
			Error:     fmt.Sprintf("%v: %s", err, strings.TrimSpace(stderr.String())),
		}
	}

	return WslExeFallbackResult{
		HostAlias: winAlias,
		WslDistro: distro,
		Reachable: out != "",
		Output:    out,
	}
}
