package evalbatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/evalrunner"
	"gopkg.in/yaml.v3"
)

// EvalFile is the YAML structure for an eval definition file.
type EvalFile struct {
	Name          string          `yaml:"name"`
	Type          string          `yaml:"type"`
	Criteria      []CriterionDef  `yaml:"criteria"`
	PassThreshold float64         `yaml:"pass_threshold"`
}

// CriterionDef matches the YAML criterion structure.
type CriterionDef struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
	Check   string `yaml:"check"`
	Pattern string `yaml:"pattern,omitempty"`
}

// EvalOutcome captures one eval's pass/fail result.
type EvalOutcome struct {
	Name     string
	Pass     bool
	PassRate float64
	Duration time.Duration
	Error    string
}

// BatchReport aggregates all eval outcomes.
type BatchReport struct {
	Total   int
	Passed  int
	Failed  int
	Results []EvalOutcome
}

// Config controls batch runner behavior.
type Config struct {
	EvalDir    string
	TimeoutSec int
	FailFast   bool
}

// BatchRunner discovers and executes all evals in a directory.
type BatchRunner struct {
	config Config
}

// NewBatchRunner creates a runner for the given config.
func NewBatchRunner(cfg Config) *BatchRunner {
	return &BatchRunner{config: cfg}
}

// LoadEvalFile reads and parses a single eval YAML file.
func LoadEvalFile(path string) (*EvalFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval file: %w", err)
	}
	var ef EvalFile
	if err := yaml.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parse eval file %s: %w", path, err)
	}
	return &ef, nil
}

// DiscoverEvals finds all .yaml/.yml files in a directory.
func DiscoverEvals(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files
}

// RunAll discovers and executes all evals, returning an aggregate report.
func (b *BatchRunner) RunAll() BatchReport {
	files := DiscoverEvals(b.config.EvalDir)
	report := BatchReport{}

	timeout := time.Duration(b.config.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	runner := evalrunner.NewRunner(evalrunner.Config{Timeout: timeout})

	for _, f := range files {
		ef, err := LoadEvalFile(f)
		if err != nil {
			report.Total++
			report.Failed++
			report.Results = append(report.Results, EvalOutcome{
				Name: f, Pass: false, Error: err.Error(),
			})
			if b.config.FailFast {
				return report
			}
			continue
		}

		var criteria []evalrunner.Criterion
		for _, c := range ef.Criteria {
			criteria = append(criteria, evalrunner.Criterion{
				Name:    c.Name,
				Command: c.Command,
				Check:   c.Check,
				Pattern: c.Pattern,
			})
		}

		evalDef := evalrunner.EvalDef{
			Name:          ef.Name,
			Criteria:      criteria,
			PassThreshold: ef.PassThreshold,
		}

		start := time.Now()
		result := runner.RunEval(context.Background(), evalDef)
		duration := time.Since(start)

		outcome := EvalOutcome{
			Name:     ef.Name,
			Pass:     result.Pass,
			PassRate: result.PassRate,
			Duration: duration,
		}

		report.Total++
		if result.Pass {
			report.Passed++
		} else {
			report.Failed++
		}
		report.Results = append(report.Results, outcome)

		if !result.Pass && b.config.FailFast {
			return report
		}
	}

	return report
}

// ToMarkdown renders the batch report as markdown.
func (r *BatchReport) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Eval Batch Report (%d/%d passed)\n\n", r.Passed, r.Total))
	sb.WriteString("| Eval | Status | Pass Rate | Duration |\n")
	sb.WriteString("|---|---|---|---|\n")
	for _, o := range r.Results {
		status := "PASS"
		if !o.Pass {
			status = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %.0f%% | %s |\n",
			o.Name, status, o.PassRate*100, o.Duration.Round(time.Millisecond)))
	}
	if r.Failed > 0 {
		sb.WriteString(fmt.Sprintf("\n**%d eval(s) FAILED**\n", r.Failed))
	}
	return sb.String()
}
