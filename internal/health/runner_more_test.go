package health_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/health"
)

func TestRunnerAndAssertFileMatches(t *testing.T) {
	tmpDir := t.TempDir()
	matchFile := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(matchFile, []byte("token=abc123"), 0o644); err != nil {
		t.Fatal(err)
	}

	suite := &health.Suite{Name: "regex"}
	suite.AssertFileMatches("token pattern", matchFile, `token=abc\d+`)
	if suite.PassCount() != 1 || suite.Total() != 1 {
		t.Fatalf("AssertFileMatches() results = %d/%d", suite.PassCount(), suite.Total())
	}

	runner := health.NewRunner()
	runner.Add(suite)
	pass, total := runner.Run()
	if pass != 1 || total != 1 {
		t.Fatalf("Run() = %d/%d, want 1/1", pass, total)
	}
}
