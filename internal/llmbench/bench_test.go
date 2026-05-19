package llmbench_test

import (
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/llmbench"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelConfig_Fields(t *testing.T) {
	cfg := llmbench.ModelConfig{
		Name:    "MiniMax-M2.7-highspeed",
		BaseURL: "https://api.minimax.chat/v1",
		Model:   "MiniMax-M2.7-highspeed",
	}
	assert.Equal(t, "MiniMax-M2.7-highspeed", cfg.Name)
	assert.Equal(t, "MiniMax-M2.7-highspeed", cfg.Model)
}

func TestBenchmarkRun_Metrics(t *testing.T) {
	run := llmbench.BenchmarkRun{
		Model:           "MiniMax-M2.7-highspeed",
		PromptTokens:    100,
		CompletionTokens: 200,
		TotalDuration:   time.Second,
		TTFT:            150 * time.Millisecond,
	}
	assert.InDelta(t, 200.0, run.TokensPerSecond(), 0.01)
	assert.Equal(t, 150*time.Millisecond, run.TTFT)
}

func TestBenchmarkRun_TokensPerSecondZeroDuration(t *testing.T) {
	run := llmbench.BenchmarkRun{CompletionTokens: 100, TotalDuration: 0}
	assert.Equal(t, 0.0, run.TokensPerSecond())
}

func TestBenchmarkSuite_CompareModels(t *testing.T) {
	suite := llmbench.NewBenchmarkSuite()
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.1", CompletionTokens: 50,
		TotalDuration: 5 * time.Second, TTFT: 2 * time.Second, Success: true,
	})
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.7-highspeed", CompletionTokens: 200,
		TotalDuration: time.Second, TTFT: 100 * time.Millisecond, Success: true,
	})

	comparison := suite.Compare()
	require.Len(t, comparison, 2)

	m21 := comparison["MiniMax-M2.1"]
	m27 := comparison["MiniMax-M2.7-highspeed"]
	assert.InDelta(t, 10.0, m21.AvgTokensPerSec, 0.01)
	assert.InDelta(t, 200.0, m27.AvgTokensPerSec, 0.01)
	assert.Equal(t, 2*time.Second, m21.AvgTTFT)
	assert.Equal(t, 100*time.Millisecond, m27.AvgTTFT)
}

func TestBenchmarkSuite_Winner(t *testing.T) {
	suite := llmbench.NewBenchmarkSuite()
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.1", CompletionTokens: 50,
		TotalDuration: 5 * time.Second, Success: true,
	})
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.7-highspeed", CompletionTokens: 200,
		TotalDuration: time.Second, Success: true,
	})
	winner := suite.Winner()
	assert.Equal(t, "MiniMax-M2.7-highspeed", winner)
}

func TestBenchmarkSuite_ToMarkdown(t *testing.T) {
	suite := llmbench.NewBenchmarkSuite()
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.1", CompletionTokens: 50,
		TotalDuration: 5 * time.Second, TTFT: 2 * time.Second, Success: true,
	})
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.7-highspeed", CompletionTokens: 200,
		TotalDuration: time.Second, TTFT: 100 * time.Millisecond, Success: true,
	})

	md := suite.ToMarkdown()
	assert.Contains(t, md, "MiniMax-M2.1")
	assert.Contains(t, md, "MiniMax-M2.7-highspeed")
	assert.Contains(t, md, "tok/s")
	assert.Contains(t, md, "Winner")
}

func TestBenchmarkSuite_Recommendation(t *testing.T) {
	suite := llmbench.NewBenchmarkSuite()
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.1", CompletionTokens: 30,
		TotalDuration: 10 * time.Second, Success: true,
	})
	suite.AddRun(llmbench.BenchmarkRun{
		Model: "MiniMax-M2.7-highspeed", CompletionTokens: 200,
		TotalDuration: time.Second, Success: true,
	})
	rec := suite.Recommendation()
	assert.Contains(t, rec, "MiniMax-M2.7-highspeed")
	assert.Contains(t, rec, "faster")
}

func TestDefaultPrompts(t *testing.T) {
	prompts := llmbench.DefaultPrompts()
	require.NotEmpty(t, prompts)
	assert.GreaterOrEqual(t, len(prompts), 3)
}
