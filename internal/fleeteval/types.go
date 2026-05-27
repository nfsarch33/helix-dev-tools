package fleeteval

import "time"

// Task defines an evaluation task loaded from YAML.
type Task struct {
	ID              string  `yaml:"id"`
	Level           int     `yaml:"level"`
	Title           string  `yaml:"title"`
	Description     string  `yaml:"description"`
	ExpectedPattern string  `yaml:"expected_output_pattern"`
	Grading         Grading `yaml:"grading"`
}

// Grading defines how a task response is scored.
type Grading struct {
	PassCriteria  string        `yaml:"pass_criteria"`
	QualityRubric []RubricEntry `yaml:"quality_rubric"`
	MaxScore      int           `yaml:"max_score"`
	PassThreshold int           `yaml:"pass_threshold"`
}

// RubricEntry is a single dimension in the quality rubric.
type RubricEntry struct {
	Metric      string  `yaml:"metric"`
	Weight      float64 `yaml:"weight"`
	Description string  `yaml:"description"`
}

// TaskFile is the top-level YAML structure for eval task definitions.
type TaskFile struct {
	Version     string  `yaml:"version"`
	TargetModel string  `yaml:"target_model"`
	Runner      string  `yaml:"runner"`
	Tasks       []Task  `yaml:"tasks"`
	Scoring     Scoring `yaml:"scoring"`
}

// Scoring holds global scoring configuration.
type Scoring struct {
	TotalTasks int            `yaml:"total_tasks"`
	MaxTotal   int            `yaml:"max_total_score"`
	PassLevels map[string]int `yaml:"pass_levels"`
	Timing     TimingConfig   `yaml:"timing"`
}

// TimingConfig sets timeout thresholds.
type TimingConfig struct {
	MaxSecondsPerTask int `yaml:"max_seconds_per_task"`
	TimeoutScore      int `yaml:"timeout_score"`
}

// Result captures the outcome of evaluating a single task.
type Result struct {
	TaskID       string       `json:"task_id"`
	Level        int          `json:"level"`
	Title        string       `json:"title"`
	Score        int          `json:"score"`
	MaxScore     int          `json:"max_score"`
	Pass         bool         `json:"pass"`
	PatternMatch bool         `json:"pattern_match"`
	Response     string       `json:"response"`
	Error        string       `json:"error,omitempty"`
	DurationMS   int64        `json:"duration_ms"`
	Timestamp    time.Time    `json:"timestamp"`
	GradeDetail  []GradeEntry `json:"grade_detail,omitempty"`
}

// GradeEntry records a single rubric dimension score.
type GradeEntry struct {
	Metric string  `json:"metric"`
	Score  float64 `json:"score"`
	Weight float64 `json:"weight"`
	Note   string  `json:"note,omitempty"`
}

// RunReport summarizes a complete eval run.
type RunReport struct {
	RunID         string    `json:"run_id"`
	Model         string    `json:"model"`
	Timestamp     time.Time `json:"timestamp"`
	TotalTasks    int       `json:"total_tasks"`
	TotalScore    int       `json:"total_score"`
	MaxScore      int       `json:"max_score"`
	PassCount     int       `json:"pass_count"`
	FailCount     int       `json:"fail_count"`
	ErrorCount    int       `json:"error_count"`
	PassRate      float64   `json:"pass_rate"`
	AvgDurationMS int64     `json:"avg_duration_ms"`
	Results       []Result  `json:"results"`
	Verdict       string    `json:"verdict"`
}
