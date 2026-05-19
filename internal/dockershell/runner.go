package dockershell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ExecResult captures the output of a container execution.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// Runner executes commands inside ephemeral Docker containers.
type Runner struct {
	DockerBin string
}

// NewRunner creates a runner using the system Docker binary.
func NewRunner() *Runner {
	bin, err := exec.LookPath("docker")
	if err != nil {
		bin = "docker"
	}
	return &Runner{DockerBin: bin}
}

// Exec runs a command inside a container configured by ContainerConfig.
func (r *Runner) Exec(ctx context.Context, cfg *ContainerConfig, command ...string) (*ExecResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("dockershell: nil config")
	}

	args := cfg.BuildRunArgs(command...)
	fullArgs := append([]string{}, args...)

	start := time.Now()
	cmd := exec.CommandContext(ctx, r.DockerBin, fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("dockershell: exec failed: %w", err)
		}
	}

	return &ExecResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}, nil
}

// Available checks if Docker is accessible.
func (r *Runner) Available() bool {
	cmd := exec.Command(r.DockerBin, "version", "--format", "{{.Server.Version}}")
	return cmd.Run() == nil
}
