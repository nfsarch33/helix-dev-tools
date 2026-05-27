package fleeteval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTaskFile_Valid(t *testing.T) {
	yaml := `
version: "1.0"
target_model: "test-model"
tasks:
  - id: eval-01
    level: 1
    title: "Echo test"
    description: "Echo back the title"
    expected_output_pattern: "^.+$"
    grading:
      pass_criteria: "exact match"
      max_score: 10
      pass_threshold: 8
  - id: eval-02
    level: 2
    title: "Count lines"
    description: "Count lines in a file"
    expected_output_pattern: "\\d+"
    grading:
      max_score: 10
      pass_threshold: 7
      quality_rubric:
        - metric: correctness
          weight: 0.8
          description: "line count is correct"
scoring:
  total_tasks: 2
  max_total_score: 20
`
	tf, err := ParseTaskFile([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tf.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tf.Tasks))
	}
	if tf.Tasks[0].ID != "eval-01" {
		t.Errorf("expected id eval-01, got %s", tf.Tasks[0].ID)
	}
	if tf.Tasks[1].Grading.MaxScore != 10 {
		t.Errorf("expected max_score 10, got %d", tf.Tasks[1].Grading.MaxScore)
	}
	if len(tf.Tasks[1].Grading.QualityRubric) != 1 {
		t.Errorf("expected 1 rubric entry, got %d", len(tf.Tasks[1].Grading.QualityRubric))
	}
}

func TestParseTaskFile_NoTasks(t *testing.T) {
	yaml := `version: "1.0"
tasks: []
`
	_, err := ParseTaskFile([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty tasks")
	}
}

func TestParseTaskFile_MissingID(t *testing.T) {
	yaml := `
tasks:
  - level: 1
    title: "no id"
    expected_output_pattern: ".*"
`
	_, err := ParseTaskFile([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestParseTaskFile_MissingPattern(t *testing.T) {
	yaml := `
tasks:
  - id: eval-01
    level: 1
    title: "no pattern"
`
	_, err := ParseTaskFile([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

func TestParseTaskFile_InvalidYAML(t *testing.T) {
	_, err := ParseTaskFile([]byte("not: [yaml: {broken"))
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestLoadTaskFile_FileNotFound(t *testing.T) {
	_, err := LoadTaskFile("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadTaskFile_RoundTrip(t *testing.T) {
	yaml := `
tasks:
  - id: eval-rt
    level: 1
    title: "round trip"
    description: "test round trip"
    expected_output_pattern: "hello"
    grading:
      max_score: 10
      pass_threshold: 5
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	tf, err := LoadTaskFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tf.Tasks[0].ID != "eval-rt" {
		t.Errorf("expected id eval-rt, got %s", tf.Tasks[0].ID)
	}
}
