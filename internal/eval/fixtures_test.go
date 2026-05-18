package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return path
}

func TestListFixtures_ReturnsSummary(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "cap.yaml", `
id: cap1
name: capability eval
type: capability
criteria:
  - name: c1
    pattern: hello
`)
	writeFixture(t, dir, "reg.yaml", `
id: reg1
name: regression eval
type: regression
criteria:
  - name: c1
    pattern: hello
`)
	writeFixture(t, dir, "perf.yaml", `
id: perf1
name: performance eval
type: performance
criteria:
  - name: c1
    grader_type: shell
    command: "true"
`)

	fixtures, err := ListFixtures(dir)
	if err != nil {
		t.Fatalf("ListFixtures: %v", err)
	}
	if len(fixtures) != 3 {
		t.Errorf("expected 3 fixtures, got %d", len(fixtures))
	}
}

func TestListFixtures_SkipsInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "good.yaml", `
id: g1
name: good
type: capability
criteria:
  - name: c1
    pattern: hello
`)
	writeFixture(t, dir, "schema.yaml", `# this is the schema comment file -- no id`)

	fixtures, err := ListFixtures(dir)
	if err != nil {
		t.Fatalf("ListFixtures: %v", err)
	}
	if len(fixtures) != 1 {
		t.Errorf("expected 1 valid fixture, got %d", len(fixtures))
	}
	if fixtures[0].ID != "g1" {
		t.Errorf("expected fixture id 'g1', got %q", fixtures[0].ID)
	}
}

func TestListFixtures_PerformanceType(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "perf.yaml", `
id: p1
name: perf test
type: performance
max_iterations: 1
criteria:
  - name: healthz
    grader_type: shell
    command: "true"
`)

	fixtures, err := ListFixtures(dir)
	if err != nil {
		t.Fatalf("ListFixtures: %v", err)
	}
	if len(fixtures) != 1 {
		t.Fatalf("expected 1 fixture, got %d", len(fixtures))
	}
	if fixtures[0].Type != EvalPerformance {
		t.Errorf("expected performance type, got %q", fixtures[0].Type)
	}
}

func TestListFixtures_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	fixtures, err := ListFixtures(dir)
	if err != nil {
		t.Fatalf("ListFixtures: %v", err)
	}
	if len(fixtures) != 0 {
		t.Errorf("expected 0 fixtures, got %d", len(fixtures))
	}
}

func TestEvalDef_Validate_PerformanceType(t *testing.T) {
	def := EvalDef{
		ID:   "p1",
		Name: "perf test",
		Type: EvalPerformance,
		Criteria: []Criterion{
			{Name: "c1", GraderType: GraderShell, Command: "true"},
		},
	}
	if err := def.Validate(); err != nil {
		t.Errorf("Validate performance: %v", err)
	}
}

func TestEvalDef_Validate_UnknownType(t *testing.T) {
	def := EvalDef{
		ID:   "x1",
		Name: "bad type",
		Type: "unknown",
		Criteria: []Criterion{
			{Name: "c1", Pattern: "x"},
		},
	}
	if err := def.Validate(); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestDefaultFixturesDir(t *testing.T) {
	dir := DefaultFixturesDir()
	if dir == "" {
		t.Error("DefaultFixturesDir should not be empty")
	}
}
