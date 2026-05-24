package evalharness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Fixture is a YAML-defined eval scenario.
type Fixture struct {
	Description string             `yaml:"description"`
	Events      []AgentTraceEvent  `yaml:"events"`
	Expected    FixtureExpectation `yaml:"expected"`
}

// FixtureExpectation defines what the graders should produce.
type FixtureExpectation struct {
	AllPass        bool     `yaml:"all_pass"`
	MinScore       float64  `yaml:"min_score"`
	FailingGraders []string `yaml:"failing_graders"`
}

// LoadFixture reads a single YAML fixture file.
func LoadFixture(path string) (*Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	var f Fixture
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", filepath.Base(path), err)
	}
	return &f, nil
}

// LoadFixtureDir reads all .yaml fixtures from a directory.
func LoadFixtureDir(dir string) (map[string]*Fixture, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read fixture dir: %w", err)
	}
	fixtures := make(map[string]*Fixture)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		f, err := LoadFixture(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		fixtures[name] = f
	}
	return fixtures, nil
}

// RunFixture executes all graders against a fixture and checks expectations.
func RunFixture(f *Fixture, graders []DeterministicGrader) ([]GradeResult, error) {
	var results []GradeResult
	for _, event := range f.Events {
		for _, g := range graders {
			results = append(results, g.Grade(event))
		}
	}
	return results, nil
}

// CheckExpectations verifies grader results against fixture expectations.
func CheckExpectations(f *Fixture, results []GradeResult) []string {
	var violations []string

	allPass := true
	failedGraders := map[string]bool{}
	totalScore := 0.0
	for _, r := range results {
		if !r.Pass {
			allPass = false
			failedGraders[r.GraderName] = true
		}
		totalScore += r.Score
	}
	avgScore := totalScore / float64(len(results))

	if f.Expected.AllPass && !allPass {
		violations = append(violations, "expected all graders to pass but some failed")
	}
	if !f.Expected.AllPass && allPass {
		violations = append(violations, "expected some graders to fail but all passed")
	}

	if f.Expected.MinScore > 0 && avgScore < f.Expected.MinScore {
		violations = append(violations, fmt.Sprintf("avg score %.2f below minimum %.2f", avgScore, f.Expected.MinScore))
	}

	for _, expected := range f.Expected.FailingGraders {
		if !failedGraders[expected] {
			violations = append(violations, fmt.Sprintf("expected grader %q to fail but it passed", expected))
		}
	}

	return violations
}
