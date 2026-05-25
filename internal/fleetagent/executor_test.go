package fleetagent

import (
	"context"
	"testing"
	"time"
)

func TestClassifyTicket_GoTest(t *testing.T) {
	tests := []struct {
		name     string
		ticket   Ticket
		wantType TaskType
	}{
		{"explicit go test", Ticket{Title: "Run go test on eval", Description: "go test -race ./internal/eval/..."}, TaskGoTest},
		{"run tests keyword", Ticket{Title: "run tests for fleetagent", Description: ""}, TaskGoTest},
		{"go build", Ticket{Title: "go build cmd/helix-dev-tools", Description: ""}, TaskGoBuild},
		{"build keyword", Ticket{Title: "Build the binary", Description: ""}, TaskGoBuild},
		{"go vet", Ticket{Title: "go vet all packages", Description: ""}, TaskGoVet},
		{"lint", Ticket{Title: "Run lint checks", Description: ""}, TaskLint},
		{"golangci", Ticket{Title: "golangci-lint run", Description: ""}, TaskLint},
		{"generic", Ticket{Title: "Review documentation", Description: ""}, TaskGeneric},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskType, _ := ClassifyTicket(tt.ticket)
			if taskType != tt.wantType {
				t.Errorf("ClassifyTicket(%q) = %q, want %q", tt.ticket.Title, taskType, tt.wantType)
			}
		})
	}
}

func TestClassifyTicket_ExtractsPackage(t *testing.T) {
	ticket := Ticket{Title: "go test ./internal/eval/...", Description: ""}
	_, pkg := ClassifyTicket(ticket)
	if pkg != "./internal/eval/..." {
		t.Errorf("expected ./internal/eval/..., got %q", pkg)
	}
}

func TestClassifyTicket_DefaultPackage(t *testing.T) {
	ticket := Ticket{Title: "Run go test on everything", Description: ""}
	_, pkg := ClassifyTicket(ticket)
	if pkg != "./..." {
		t.Errorf("expected ./..., got %q", pkg)
	}
}

func TestShellExecutor_CommandFor(t *testing.T) {
	e := NewShellExecutor("/tmp", time.Minute)

	tests := []struct {
		taskType TaskType
		pkg      string
		wantNil  bool
	}{
		{TaskGoTest, "./...", false},
		{TaskGoBuild, "./cmd/x", false},
		{TaskGoVet, "./...", false},
		{TaskLint, "", false},
		{TaskGeneric, "", true},
	}

	for _, tt := range tests {
		cmd := e.commandFor(tt.taskType, tt.pkg)
		if tt.wantNil && cmd != "" {
			t.Errorf("commandFor(%q) should be empty", tt.taskType)
		}
		if !tt.wantNil && cmd == "" {
			t.Errorf("commandFor(%q) should not be empty", tt.taskType)
		}
	}
}

func TestShellExecutor_Execute_UnsupportedType(t *testing.T) {
	e := NewShellExecutor(t.TempDir(), time.Minute)
	_, err := e.Execute(context.Background(), TaskGeneric, "")
	if err == nil {
		t.Error("expected error for unsupported task type")
	}
}

func TestShellExecutor_Execute_Echo(t *testing.T) {
	e := &ShellExecutor{WorkDir: t.TempDir(), Timeout: 5 * time.Second}
	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := "echo hello"
	c := &echoExecutor{cmd: cmd, workDir: e.WorkDir, timeout: e.Timeout}
	output, err := c.run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", output)
	}
}

type echoExecutor struct {
	cmd     string
	workDir string
	timeout time.Duration
}

func (e *echoExecutor) run(ctx context.Context) (string, error) {
	se := &ShellExecutor{WorkDir: e.workDir, Timeout: e.timeout}
	return se.executeRaw(ctx, e.cmd)
}

func TestNewShellExecutor_DefaultTimeout(t *testing.T) {
	e := NewShellExecutor("/tmp", 0)
	if e.Timeout != 5*time.Minute {
		t.Errorf("expected 5m default timeout, got %v", e.Timeout)
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 100) != short {
		t.Error("should not truncate short strings")
	}

	long := "abcdefghij"
	result := truncate(long, 5)
	if result != "abcde...[truncated]" {
		t.Errorf("unexpected truncation: %q", result)
	}
}
