package evalharness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFixtureDir(t *testing.T) {
	dir := findFixtureDir(t)
	fixtures, err := LoadFixtureDir(dir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(fixtures), 5, "expected at least 5 fixtures")
	for name, f := range fixtures {
		assert.NotEmpty(t, f.Description, "fixture %s missing description", name)
		assert.NotEmpty(t, f.Events, "fixture %s has no events", name)
	}
}

func TestRunFixture_ToolSuccess(t *testing.T) {
	dir := findFixtureDir(t)
	f, err := LoadFixture(filepath.Join(dir, "tool_success.yaml"))
	require.NoError(t, err)

	graders := AllGraders(DefaultGraderConfig())
	results, err := RunFixture(f, graders)
	require.NoError(t, err)
	violations := CheckExpectations(f, results)
	assert.Empty(t, violations, "tool_success fixture should have no violations: %v", violations)
}

func TestRunFixture_LatencyFailure(t *testing.T) {
	dir := findFixtureDir(t)
	f, err := LoadFixture(filepath.Join(dir, "latency_failure.yaml"))
	require.NoError(t, err)

	graders := AllGraders(DefaultGraderConfig())
	results, err := RunFixture(f, graders)
	require.NoError(t, err)
	violations := CheckExpectations(f, results)
	assert.Empty(t, violations, "latency_failure expectations should match: %v", violations)
}

func TestRunFixture_ErrorEvent(t *testing.T) {
	dir := findFixtureDir(t)
	f, err := LoadFixture(filepath.Join(dir, "error_event.yaml"))
	require.NoError(t, err)

	graders := AllGraders(DefaultGraderConfig())
	results, err := RunFixture(f, graders)
	require.NoError(t, err)
	violations := CheckExpectations(f, results)
	assert.Empty(t, violations, "error_event expectations should match: %v", violations)
}

func TestRunFixture_LowCoverage(t *testing.T) {
	dir := findFixtureDir(t)
	f, err := LoadFixture(filepath.Join(dir, "low_coverage.yaml"))
	require.NoError(t, err)

	graders := AllGraders(DefaultGraderConfig())
	results, err := RunFixture(f, graders)
	require.NoError(t, err)
	violations := CheckExpectations(f, results)
	assert.Empty(t, violations, "low_coverage expectations should match: %v", violations)
}

func TestRunFixture_TokenOveruse(t *testing.T) {
	dir := findFixtureDir(t)
	f, err := LoadFixture(filepath.Join(dir, "token_overuse.yaml"))
	require.NoError(t, err)

	graders := AllGraders(DefaultGraderConfig())
	results, err := RunFixture(f, graders)
	require.NoError(t, err)
	violations := CheckExpectations(f, results)
	assert.Empty(t, violations, "token_overuse expectations should match: %v", violations)
}

func findFixtureDir(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"fixtures",
		"internal/evalharness/fixtures",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	cwd, _ := os.Getwd()
	dir := filepath.Join(cwd, "fixtures")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	t.Skip("fixture directory not found")
	return ""
}
