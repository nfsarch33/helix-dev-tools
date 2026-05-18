package evalgate

import (
	"encoding/json"
	"errors"
	"time"

	"gopkg.in/yaml.v3"
)

// Criterion defines a single pass/fail check within an eval.
type Criterion struct {
	Name    string `yaml:"name" json:"name"`
	Check   string `yaml:"check" json:"check"`
	Command string `yaml:"command" json:"command"`
}

// EvalDef is a YAML-defined evaluation specification.
type EvalDef struct {
	Name          string      `yaml:"name" json:"name"`
	Type          string      `yaml:"type" json:"type"`
	Criteria      []Criterion `yaml:"criteria" json:"criteria"`
	PassThreshold float64     `yaml:"pass_threshold" json:"pass_threshold"`
}

// ParseEvalDef deserializes an eval definition from YAML.
func ParseEvalDef(data []byte) (*EvalDef, error) {
	var def EvalDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

// Validate checks the eval definition for required fields.
func (d *EvalDef) Validate() error {
	if d.Name == "" {
		return errors.New("eval name is required")
	}
	if d.Type == "" {
		return errors.New("eval type is required")
	}
	if len(d.Criteria) == 0 {
		return errors.New("at least one criterion is required")
	}
	return nil
}

// BatchConfig defines a set of evals to run together.
type BatchConfig struct {
	Evals    []BatchEntry `yaml:"evals"`
	FailFast bool         `yaml:"fail_fast"`
}

// BatchEntry points to a single eval file.
type BatchEntry struct {
	Path string `yaml:"path"`
}

// ParseBatchConfig deserializes a batch configuration from YAML.
func ParseBatchConfig(data []byte) (*BatchConfig, error) {
	var cfg BatchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// EvalResult captures the outcome of running an eval.
type EvalResult struct {
	Name            string          `json:"name"`
	PassRate        float64         `json:"pass_rate"`
	CriteriaResults map[string]bool `json:"criteria_results,omitempty"`
	Timestamp       string          `json:"ts,omitempty"`
}

// ToNDJSON serializes the result as a newline-delimited JSON line.
func (r EvalResult) ToNDJSON() string {
	if r.Timestamp == "" {
		r.Timestamp = time.Now().Format(time.RFC3339)
	}
	data, _ := json.Marshal(r)
	return string(data) + "\n"
}

// Baseline stores a previous eval result for regression comparison.
type Baseline struct {
	Name      string  `json:"name" yaml:"name"`
	PassRate  float64 `json:"pass_rate" yaml:"pass_rate"`
	Timestamp string  `json:"ts,omitempty" yaml:"timestamp,omitempty"`
}

// DetectRegression returns true if the current result regressed beyond threshold.
func (b Baseline) DetectRegression(current EvalResult, threshold float64) bool {
	return (b.PassRate - current.PassRate) > threshold
}

// SprintGate evaluates whether a sprint passes all eval baselines.
type SprintGate struct {
	Baselines           []Baseline
	Results             []EvalResult
	RegressionThreshold float64
}

// GateVerdict is the pass/fail outcome of the sprint gate.
type GateVerdict struct {
	Pass        bool
	Regressions []string
}

// Evaluate checks all results against baselines.
func (g *SprintGate) Evaluate() GateVerdict {
	baselineMap := make(map[string]Baseline, len(g.Baselines))
	for _, b := range g.Baselines {
		baselineMap[b.Name] = b
	}

	verdict := GateVerdict{Pass: true}
	for _, result := range g.Results {
		if baseline, ok := baselineMap[result.Name]; ok {
			if baseline.DetectRegression(result, g.RegressionThreshold) {
				verdict.Pass = false
				verdict.Regressions = append(verdict.Regressions, result.Name)
			}
		}
	}
	return verdict
}
