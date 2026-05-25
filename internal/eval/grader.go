package eval

import (
	"os/exec"
	"strings"
)

type Grader interface {
	Grade(output string) (float64, string, error)
}

type CodeGrader struct {
	Pattern string
}

func (g *CodeGrader) Grade(output string) (float64, string, error) {
	if g.Pattern == "" {
		return 0, "no pattern configured", errorf("code grader requires a pattern")
	}
	if strings.Contains(output, g.Pattern) {
		return 1.0, "pattern found", nil
	}
	return 0, "pattern not found in output", nil
}

type ShellGrader struct {
	Command string
}

func (g *ShellGrader) Grade(_ string) (float64, string, error) {
	if g.Command == "" {
		return 0, "no command configured", errorf("shell grader requires a command")
	}
	cmd := exec.Command("sh", "-c", g.Command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, string(out), nil
	}
	return 1.0, string(out), nil
}

func NewGrader(c Criterion) Grader {
	switch c.GraderType {
	case GraderShell:
		return &ShellGrader{Command: c.Command}
	case GraderCoverage:
		return &CoverageGrader{Package: c.Package, Threshold: c.Threshold}
	case GraderTest:
		return &TestGrader{Package: c.Package, Race: c.Race}
	case GraderLint:
		return &LintGrader{Config: c.Config}
	case GraderVet:
		return &VetGrader{Package: c.Package}
	case GraderCode:
		return &CodeGrader{Pattern: c.Pattern}
	default:
		return &CodeGrader{Pattern: c.Pattern}
	}
}
