package eval

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// CoverageGrader checks that Go test coverage meets a threshold.
type CoverageGrader struct {
	Package   string
	Threshold float64
}

func (g *CoverageGrader) Grade(_ string) (float64, string, error) {
	pkg := g.Package
	if pkg == "" {
		pkg = "./..."
	}
	threshold := g.Threshold
	if threshold <= 0 {
		threshold = 70.0
	}

	cmd := exec.Command("go", "test", "-cover", pkg)
	out, err := cmd.CombinedOutput()
	output := string(out)

	coverage := parseCoverage(output)
	if err != nil && coverage == 0 {
		return 0, output, nil
	}

	score := coverage / 100.0
	if score > 1.0 {
		score = 1.0
	}

	if coverage >= threshold {
		return score, fmt.Sprintf("coverage %.1f%% >= %.1f%% threshold", coverage, threshold), nil
	}
	return score, fmt.Sprintf("coverage %.1f%% < %.1f%% threshold", coverage, threshold), nil
}

var coverageRe = regexp.MustCompile(`coverage:\s*([\d.]+)%`)

func parseCoverage(output string) float64 {
	matches := coverageRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0
	}
	var total float64
	for _, m := range matches {
		v, _ := strconv.ParseFloat(m[1], 64)
		total += v
	}
	return total / float64(len(matches))
}

// TestGrader runs go test and grades based on pass/fail.
type TestGrader struct {
	Package string
	Race    bool
}

func (g *TestGrader) Grade(_ string) (float64, string, error) {
	pkg := g.Package
	if pkg == "" {
		pkg = "./..."
	}

	args := []string{"test", "-count=1"}
	if g.Race {
		args = append(args, "-race")
	}
	args = append(args, pkg)

	cmd := exec.Command("go", args...)
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		failCount := strings.Count(output, "FAIL")
		return 0, fmt.Sprintf("%d test failure(s)\n%s", failCount, truncateGrader(output, 1000)), nil
	}

	passCount := strings.Count(output, "ok")
	return 1.0, fmt.Sprintf("%d package(s) passed", passCount), nil
}

// LintGrader runs golangci-lint and grades based on findings.
type LintGrader struct {
	Config string
}

func (g *LintGrader) Grade(_ string) (float64, string, error) {
	args := []string{"run", "./..."}
	if g.Config != "" {
		args = append(args, "--config", g.Config)
	}

	cmd := exec.Command("golangci-lint", args...)
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		lines := strings.Count(strings.TrimSpace(output), "\n") + 1
		score := 0.0
		if lines <= 5 {
			score = 0.5
		}
		return score, fmt.Sprintf("%d lint issue(s)\n%s", lines, truncateGrader(output, 1000)), nil
	}
	return 1.0, "no lint issues", nil
}

// VetGrader runs go vet and grades on clean output.
type VetGrader struct {
	Package string
}

func (g *VetGrader) Grade(_ string) (float64, string, error) {
	pkg := g.Package
	if pkg == "" {
		pkg = "./..."
	}

	cmd := exec.Command("go", "vet", pkg)
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		return 0, fmt.Sprintf("vet failed:\n%s", truncateGrader(output, 1000)), nil
	}
	return 1.0, "go vet clean", nil
}

func truncateGrader(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}
