package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEvalFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-eval.yaml")
	os.WriteFile(path, []byte(`
id: e1
name: build check
type: capability
task: "func main() {}"
criteria:
  - name: has_main
    grader_type: code
    pattern: "func main"
`), 0644)

	def, err := LoadEvalFile(path)
	if err != nil {
		t.Fatalf("LoadEvalFile: %v", err)
	}
	if def.ID != "e1" {
		t.Errorf("ID = %q, want e1", def.ID)
	}
	if def.Name != "build check" {
		t.Errorf("Name = %q, want 'build check'", def.Name)
	}
	if len(def.Criteria) != 1 {
		t.Errorf("Criteria count = %d, want 1", len(def.Criteria))
	}
}

func TestLoadEvalFile_AutoID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-eval.yaml")
	os.WriteFile(path, []byte(`
name: auto id test
type: capability
task: test
criteria:
  - name: c1
    pattern: test
`), 0644)

	def, err := LoadEvalFile(path)
	if err != nil {
		t.Fatalf("LoadEvalFile: %v", err)
	}
	if def.ID != "my-eval" {
		t.Errorf("ID = %q, want 'my-eval' (auto from filename)", def.ID)
	}
}

func TestLoadEvalFile_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("not: valid: yaml: ["), 0644)

	_, err := LoadEvalFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadEvalFile_Missing(t *testing.T) {
	_, err := LoadEvalFile("/nonexistent/eval.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestListEvalFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "eval1.yaml"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "eval2.yml"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte(""), 0644)

	files, err := ListEvalFiles(dir)
	if err != nil {
		t.Fatalf("ListEvalFiles: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2 (only .yaml/.yml)", len(files))
	}
}

func TestListEvalFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := ListEvalFiles(dir)
	if err != nil {
		t.Fatalf("ListEvalFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}

func TestRunEvalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "passing-eval.yaml")
	os.WriteFile(path, []byte(`
id: pass1
name: passing eval
type: capability
task: "func main() {}"
criteria:
  - name: has_main
    grader_type: code
    pattern: "func main"
`), 0644)

	result, err := RunEvalFile(path)
	if err != nil {
		t.Fatalf("RunEvalFile: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected pass, got fail: %+v", result)
	}
}

func TestRunAllEvalsInDir(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "pass.yaml"), []byte(`
id: pass1
name: passing
type: capability
task: "func main() {}"
criteria:
  - name: has_main
    grader_type: code
    pattern: "func main"
`), 0644)

	os.WriteFile(filepath.Join(dir, "fail.yaml"), []byte(`
id: fail1
name: failing
type: capability
task: nothing here
criteria:
  - name: has_main
    grader_type: code
    pattern: "func main"
`), 0644)

	results, err := RunAllEvalsInDir(dir)
	if err != nil {
		t.Fatalf("RunAllEvalsInDir: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	passCount := 0
	for _, r := range results {
		if r.Pass {
			passCount++
		}
	}
	if passCount != 1 {
		t.Errorf("passCount = %d, want 1", passCount)
	}
}
