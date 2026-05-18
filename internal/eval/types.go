package eval

import (
	"encoding/json"
	"time"
)

type EvalType string

const (
	EvalCapability EvalType = "capability"
	EvalRegression EvalType = "regression"
)

type GraderType string

const (
	GraderCode  GraderType = "code"
	GraderShell GraderType = "shell"
	GraderModel GraderType = "model"
)

type Criterion struct {
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	GraderType  GraderType `json:"grader_type" yaml:"grader_type"`
	Command     string     `json:"command,omitempty" yaml:"command,omitempty"`
	Pattern     string     `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Weight      float64    `json:"weight,omitempty" yaml:"weight,omitempty"`
}

type EvalDef struct {
	ID            string      `json:"id" yaml:"id"`
	Name          string      `json:"name" yaml:"name"`
	Type          EvalType    `json:"type" yaml:"type"`
	Task          string      `json:"task" yaml:"task"`
	Criteria      []Criterion `json:"criteria" yaml:"criteria"`
	MaxIterations int         `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty"`
	TimeoutSec    int         `json:"timeout_sec,omitempty" yaml:"timeout_sec,omitempty"`
	Baseline      string      `json:"baseline,omitempty" yaml:"baseline,omitempty"`
}

type CriterionResult struct {
	Name    string  `json:"name"`
	Pass    bool    `json:"pass"`
	Score   float64 `json:"score"`
	Details string  `json:"details,omitempty"`
}

type EvalResult struct {
	EvalID     string            `json:"eval_id"`
	EvalName   string            `json:"eval_name"`
	EvalType   EvalType          `json:"eval_type"`
	Pass       bool              `json:"pass"`
	Score      float64           `json:"score"`
	Criteria   []CriterionResult `json:"criteria"`
	Iterations int               `json:"iterations"`
	DurationMS int64             `json:"duration_ms"`
	Timestamp  time.Time         `json:"timestamp"`
	Error      string            `json:"error,omitempty"`
}

func (d *EvalDef) Validate() error {
	if d.ID == "" {
		return errorf("eval id is required")
	}
	if d.Name == "" {
		return errorf("eval name is required")
	}
	if len(d.Criteria) == 0 {
		return errorf("at least one criterion is required")
	}
	if d.MaxIterations <= 0 {
		d.MaxIterations = 3
	}
	if d.TimeoutSec <= 0 {
		d.TimeoutSec = 300
	}
	return nil
}

func (r *EvalResult) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

func (r *EvalResult) ToNDJSON() string {
	data, _ := json.Marshal(r)
	return string(data)
}
