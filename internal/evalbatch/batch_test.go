package evalbatch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/evalbatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEvalFile(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `name: test-eval
type: capability
criteria:
  - name: echo-check
    command: "echo hello"
    check: exit_code_zero
pass_threshold: 1.0
`
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte(yamlContent), 0644)

	def, err := evalbatch.LoadEvalFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test-eval", def.Name)
	assert.Len(t, def.Criteria, 1)
}

func TestLoadEvalFile_NotFound(t *testing.T) {
	_, err := evalbatch.LoadEvalFile("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestDiscoverEvals(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("name: a\ntype: capability\ncriteria:\n  - name: c1\n    command: echo\n    check: exit_code_zero\npass_threshold: 1.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.yml"), []byte("name: b\ntype: regression\ncriteria:\n  - name: c1\n    command: echo\n    check: exit_code_zero\npass_threshold: 0.9\n"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("not yaml"), 0644)

	files := evalbatch.DiscoverEvals(dir)
	assert.Len(t, files, 2)
}

func TestBatchRunner_RunAll(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pass.yaml"), []byte("name: pass-eval\ntype: capability\ncriteria:\n  - name: c1\n    command: \"echo pass\"\n    check: exit_code_zero\npass_threshold: 1.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "fail.yaml"), []byte("name: fail-eval\ntype: capability\ncriteria:\n  - name: c1\n    command: \"exit 1\"\n    check: exit_code_zero\npass_threshold: 1.0\n"), 0644)

	runner := evalbatch.NewBatchRunner(evalbatch.Config{
		EvalDir:    dir,
		TimeoutSec: 10,
	})
	report := runner.RunAll()
	assert.Equal(t, 2, report.Total)
	assert.Equal(t, 1, report.Passed)
	assert.Equal(t, 1, report.Failed)
	assert.Len(t, report.Results, 2)
}

func TestBatchRunner_FailFast(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "01-fail.yaml"), []byte("name: first-fail\ntype: capability\ncriteria:\n  - name: c1\n    command: \"exit 1\"\n    check: exit_code_zero\npass_threshold: 1.0\n"), 0644)
	os.WriteFile(filepath.Join(dir, "02-pass.yaml"), []byte("name: second-pass\ntype: capability\ncriteria:\n  - name: c1\n    command: \"echo ok\"\n    check: exit_code_zero\npass_threshold: 1.0\n"), 0644)

	runner := evalbatch.NewBatchRunner(evalbatch.Config{
		EvalDir:    dir,
		TimeoutSec: 10,
		FailFast:   true,
	})
	report := runner.RunAll()
	assert.Equal(t, 1, report.Total)
	assert.Equal(t, 1, report.Failed)
}

func TestBatchReport_ToMarkdown(t *testing.T) {
	report := evalbatch.BatchReport{
		Total:  3,
		Passed: 2,
		Failed: 1,
		Results: []evalbatch.EvalOutcome{
			{Name: "a", Pass: true, PassRate: 1.0},
			{Name: "b", Pass: true, PassRate: 1.0},
			{Name: "c", Pass: false, PassRate: 0.5},
		},
	}
	md := report.ToMarkdown()
	assert.Contains(t, md, "2/3 passed")
	assert.Contains(t, md, "FAIL")
	assert.Contains(t, md, "PASS")
}
