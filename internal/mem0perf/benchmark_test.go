package mem0perf_test

import (
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/mem0perf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBenchmarkResult_Fields(t *testing.T) {
	r := mem0perf.BenchmarkResult{
		Operation: "search",
		Duration:  150 * time.Millisecond,
		Success:   true,
		StatusCode: 200,
	}
	assert.Equal(t, "search", r.Operation)
	assert.Equal(t, 150*time.Millisecond, r.Duration)
	assert.True(t, r.Success)
	assert.Equal(t, 200, r.StatusCode)
}

func TestBenchmarkSuite_AddResult(t *testing.T) {
	suite := mem0perf.NewBenchmarkSuite("http://localhost:18888")
	suite.AddResult(mem0perf.BenchmarkResult{
		Operation:  "add",
		Duration:   200 * time.Millisecond,
		Success:    true,
		StatusCode: 200,
	})
	suite.AddResult(mem0perf.BenchmarkResult{
		Operation:  "search",
		Duration:   500 * time.Millisecond,
		Success:    true,
		StatusCode: 200,
	})
	assert.Len(t, suite.Results, 2)
}

func TestBenchmarkSuite_Summary(t *testing.T) {
	suite := mem0perf.NewBenchmarkSuite("http://localhost:18888")
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "add", Duration: 100 * time.Millisecond, Success: true})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "add", Duration: 200 * time.Millisecond, Success: true})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "add", Duration: 300 * time.Millisecond, Success: false})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "search", Duration: 500 * time.Millisecond, Success: true})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "search", Duration: 1500 * time.Millisecond, Success: true})

	summary := suite.Summary()
	require.Contains(t, summary, "add")
	require.Contains(t, summary, "search")

	addStats := summary["add"]
	assert.Equal(t, 3, addStats.Count)
	assert.Equal(t, 2, addStats.SuccessCount)
	assert.Equal(t, 200*time.Millisecond, addStats.AvgDuration)
	assert.Equal(t, 100*time.Millisecond, addStats.MinDuration)
	assert.Equal(t, 300*time.Millisecond, addStats.MaxDuration)

	searchStats := summary["search"]
	assert.Equal(t, 2, searchStats.Count)
	assert.Equal(t, 2, searchStats.SuccessCount)
	assert.Equal(t, 1000*time.Millisecond, searchStats.AvgDuration)
}

func TestBenchmarkSuite_Thresholds(t *testing.T) {
	thresholds := mem0perf.DefaultThresholds()
	assert.Equal(t, 500*time.Millisecond, thresholds["healthz"])
	assert.Equal(t, 2*time.Second, thresholds["add_infer"])
	assert.Equal(t, 500*time.Millisecond, thresholds["add_no_infer"])
	assert.Equal(t, 2*time.Second, thresholds["search"])
}

func TestBenchmarkSuite_CheckThresholds(t *testing.T) {
	suite := mem0perf.NewBenchmarkSuite("http://localhost:18888")
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "healthz", Duration: 100 * time.Millisecond, Success: true})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "search", Duration: 5 * time.Second, Success: true})

	violations := suite.CheckThresholds(mem0perf.DefaultThresholds())
	assert.Len(t, violations, 1)
	assert.Equal(t, "search", violations[0].Operation)
	assert.Equal(t, 5*time.Second, violations[0].Actual)
	assert.Equal(t, 2*time.Second, violations[0].Threshold)
}

func TestPerformanceReport_ToMarkdown(t *testing.T) {
	suite := mem0perf.NewBenchmarkSuite("http://localhost:18888")
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "healthz", Duration: 80 * time.Millisecond, Success: true})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "search", Duration: 800 * time.Millisecond, Success: true})

	report := suite.GenerateReport()
	md := report.ToMarkdown()
	assert.Contains(t, md, "# Mem0 OSS Performance Report")
	assert.Contains(t, md, "http://localhost:18888")
	assert.Contains(t, md, "healthz")
	assert.Contains(t, md, "search")
}

func TestDiagnosis_BottleneckIdentification(t *testing.T) {
	suite := mem0perf.NewBenchmarkSuite("http://localhost:18888")
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "healthz", Duration: 80 * time.Millisecond, Success: true})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "add_no_infer", Duration: 200 * time.Millisecond, Success: true})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "add_infer", Duration: 35 * time.Second, Success: false})
	suite.AddResult(mem0perf.BenchmarkResult{Operation: "search", Duration: 30 * time.Second, Success: false})

	diagnosis := suite.Diagnose()
	assert.True(t, diagnosis.EmbeddingBottleneck)
	assert.True(t, diagnosis.LLMBottleneck)
	assert.False(t, diagnosis.NetworkBottleneck)
	assert.Contains(t, diagnosis.Recommendations, "Switch MiniMax model from M2.1 to m2-7-highspeed")
	assert.Contains(t, diagnosis.Recommendations, "Enable CUDA for Qwen embedding bridge")
}
