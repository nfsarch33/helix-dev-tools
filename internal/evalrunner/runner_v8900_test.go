package evalrunner_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/evalrunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunner_NDJSONShapedCommand_v8900 is a B3 regression pin: when an eval
// criterion runs a command that emits an NDJSON-shaped line, the runner
// must capture the line verbatim in CriterionResult.Output so a future
// upstream NDJSON consumer can decode it without going through a
// re-stringification step. This pins the contract before B26..B28 land
// the canonical NDJSON consumer.
func TestRunner_NDJSONShapedCommand_v8900(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	const ndjsonLine = `{"ts":"2026-05-23T00:00:00Z","tool":"echo","duration_ms":1,"success":true}`
	result := r.ExecCriterion(context.Background(), evalrunner.Criterion{
		Name:    "ndjson-passthrough",
		Command: "printf '%s\\n' '" + ndjsonLine + "'",
		Check:   "output_contains",
		Pattern: `"tool":"echo"`,
	})
	require.True(t, result.Pass, "criterion should pass")
	require.NotEmpty(t, result.Output, "output must be non-empty")
	first := strings.TrimSpace(strings.SplitN(result.Output, "\n", 2)[0])
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(first), &event), "first output line must be valid JSON")
	assert.Equal(t, "echo", event["tool"])
	assert.Equal(t, true, event["success"])
}

// TestRunner_NDJSONMultiLine_v8900 pins multi-line NDJSON emission. Every
// non-empty line must round-trip through encoding/json without per-line
// repair; this is the contract the eval harness consumer (B26) will rely
// on.
func TestRunner_NDJSONMultiLine_v8900(t *testing.T) {
	r := evalrunner.NewRunner(evalrunner.Config{Timeout: 5 * time.Second})
	cmd := `printf '%s\n%s\n' '{"a":1}' '{"a":2}'`
	result := r.ExecCriterion(context.Background(), evalrunner.Criterion{
		Name:    "ndjson-multi",
		Command: cmd,
		Check:   "exit_code_zero",
	})
	require.True(t, result.Pass)
	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	require.Len(t, lines, 2, "expected exactly two NDJSON lines")
	for i, line := range lines {
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &got), "line %d must parse", i)
	}
}
