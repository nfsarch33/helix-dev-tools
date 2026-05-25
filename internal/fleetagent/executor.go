package fleetagent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// TaskType classifies the kind of work a ticket describes.
type TaskType string

const (
	TaskGoTest  TaskType = "go-test"
	TaskGoBuild TaskType = "go-build"
	TaskGoVet   TaskType = "go-vet"
	TaskLint    TaskType = "lint"
	TaskGeneric TaskType = "generic"
)

// ShellExecutor runs concrete build/test commands instead of delegating to an LLM.
type ShellExecutor struct {
	WorkDir string
	Timeout time.Duration
}

// NewShellExecutor creates an executor that runs commands in workDir.
func NewShellExecutor(workDir string, timeout time.Duration) *ShellExecutor {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &ShellExecutor{WorkDir: workDir, Timeout: timeout}
}

// Execute runs the appropriate command for the given task type and returns output.
func (e *ShellExecutor) Execute(ctx context.Context, taskType TaskType, pkg string) (string, error) {
	cmdStr := e.commandFor(taskType, pkg)
	if cmdStr == "" {
		return "", fmt.Errorf("unsupported task type %q", taskType)
	}

	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = e.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n--- stderr ---\n" + stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("command failed: %w\noutput: %s", err, truncate(output, 2000))
	}
	return output, nil
}

func (e *ShellExecutor) commandFor(taskType TaskType, pkg string) string {
	if pkg == "" {
		pkg = "./..."
	}
	switch taskType {
	case TaskGoTest:
		return fmt.Sprintf("go test -race -count=1 %s", pkg)
	case TaskGoBuild:
		return fmt.Sprintf("go build %s", pkg)
	case TaskGoVet:
		return fmt.Sprintf("go vet %s", pkg)
	case TaskLint:
		return "golangci-lint run ./..."
	default:
		return ""
	}
}

// ClassifyTicket determines the task type from a ticket's title and description.
func ClassifyTicket(t Ticket) (TaskType, string) {
	combined := strings.ToLower(t.Title + " " + t.Description)

	if strings.Contains(combined, "go test") || strings.Contains(combined, "run tests") {
		return TaskGoTest, extractPackage(combined)
	}
	if strings.Contains(combined, "go build") || strings.Contains(combined, "build") {
		return TaskGoBuild, extractPackage(combined)
	}
	if strings.Contains(combined, "go vet") || strings.Contains(combined, "vet") {
		return TaskGoVet, extractPackage(combined)
	}
	if strings.Contains(combined, "lint") || strings.Contains(combined, "golangci") {
		return TaskLint, ""
	}
	return TaskGeneric, ""
}

func extractPackage(text string) string {
	for _, word := range strings.Fields(text) {
		if strings.HasPrefix(word, "./") || strings.Contains(word, "/internal/") {
			return word
		}
	}
	return "./..."
}

// executeRaw runs a raw shell command in workDir and returns stdout.
func (e *ShellExecutor) executeRaw(ctx context.Context, cmdStr string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = e.WorkDir

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
