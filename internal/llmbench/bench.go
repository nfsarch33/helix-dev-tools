package llmbench

import (
	"fmt"
	"strings"
	"time"
)

// ModelConfig identifies an LLM endpoint for benchmarking.
type ModelConfig struct {
	Name    string
	BaseURL string
	Model   string
	APIKey  string
}

// BenchmarkRun captures one invocation's timing metrics.
type BenchmarkRun struct {
	Model            string
	Prompt           string
	PromptTokens     int
	CompletionTokens int
	TotalDuration    time.Duration
	TTFT             time.Duration
	Success          bool
	Error            string
}

// TokensPerSecond calculates the generation throughput.
func (r BenchmarkRun) TokensPerSecond() float64 {
	if r.TotalDuration == 0 {
		return 0
	}
	return float64(r.CompletionTokens) / r.TotalDuration.Seconds()
}

// ModelStats aggregates metrics for one model across runs.
type ModelStats struct {
	Model          string
	Runs           int
	SuccessCount   int
	AvgTokensPerSec float64
	AvgTTFT        time.Duration
	AvgDuration    time.Duration
}

// BenchmarkSuite manages comparison runs across models.
type BenchmarkSuite struct {
	Runs []BenchmarkRun
}

// NewBenchmarkSuite creates an empty suite.
func NewBenchmarkSuite() *BenchmarkSuite {
	return &BenchmarkSuite{}
}

// AddRun appends a benchmark run.
func (s *BenchmarkSuite) AddRun(r BenchmarkRun) {
	s.Runs = append(s.Runs, r)
}

// Compare produces per-model statistics.
func (s *BenchmarkSuite) Compare() map[string]ModelStats {
	groups := make(map[string][]BenchmarkRun)
	for _, r := range s.Runs {
		groups[r.Model] = append(groups[r.Model], r)
	}

	result := make(map[string]ModelStats, len(groups))
	for model, runs := range groups {
		stats := ModelStats{Model: model, Runs: len(runs)}
		var totalTPS float64
		var totalTTFT, totalDuration time.Duration
		for _, r := range runs {
			if r.Success {
				stats.SuccessCount++
			}
			totalTPS += r.TokensPerSecond()
			totalTTFT += r.TTFT
			totalDuration += r.TotalDuration
		}
		if stats.Runs > 0 {
			stats.AvgTokensPerSec = totalTPS / float64(stats.Runs)
			stats.AvgTTFT = totalTTFT / time.Duration(stats.Runs)
			stats.AvgDuration = totalDuration / time.Duration(stats.Runs)
		}
		result[model] = stats
	}
	return result
}

// Winner returns the model with highest avg tokens/sec.
func (s *BenchmarkSuite) Winner() string {
	comparison := s.Compare()
	best := ""
	bestTPS := 0.0
	for model, stats := range comparison {
		if stats.AvgTokensPerSec > bestTPS {
			best = model
			bestTPS = stats.AvgTokensPerSec
		}
	}
	return best
}

// Recommendation produces a human-readable recommendation.
func (s *BenchmarkSuite) Recommendation() string {
	comparison := s.Compare()
	winner := s.Winner()
	if winner == "" {
		return "No benchmark data available."
	}
	winStats := comparison[winner]
	var others []string
	for model, stats := range comparison {
		if model != winner {
			speedup := winStats.AvgTokensPerSec / stats.AvgTokensPerSec
			others = append(others, fmt.Sprintf("%.1fx faster than %s", speedup, model))
		}
	}
	return fmt.Sprintf("Recommendation: Use %s (%.0f tok/s, %s). %s",
		winner, winStats.AvgTokensPerSec, strings.Join(others, "; "),
		"Switch Mem0 config to this model for sub-second inference.")
}

// ToMarkdown generates a comparison report.
func (s *BenchmarkSuite) ToMarkdown() string {
	comparison := s.Compare()
	winner := s.Winner()

	var sb strings.Builder
	sb.WriteString("# LLM Benchmark Comparison\n\n")
	sb.WriteString("| Model | Runs | Success | Avg tok/s | Avg TTFT | Avg Duration |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")

	for model, stats := range comparison {
		marker := ""
		if model == winner {
			marker = " (Winner)"
		}
		sb.WriteString(fmt.Sprintf("| %s%s | %d | %d/%d | %.1f | %s | %s |\n",
			model, marker, stats.Runs, stats.SuccessCount, stats.Runs,
			stats.AvgTokensPerSec,
			stats.AvgTTFT.Round(time.Millisecond),
			stats.AvgDuration.Round(time.Millisecond)))
	}

	sb.WriteString("\n## Verdict\n\n")
	sb.WriteString(s.Recommendation() + "\n")
	return sb.String()
}

// DefaultPrompts returns standard benchmark prompts for consistent comparison.
func DefaultPrompts() []string {
	return []string{
		"Summarize this text in 3 bullet points: The quick brown fox jumps over the lazy dog. This sentence contains every letter of the English alphabet.",
		"Extract the key entities from: A developer is building a platform on a workstation with Go, running vLLM on Linux with multiple NVIDIA GPUs.",
		"Generate a JSON object with fields: name, role, and 3 capabilities for a coding agent called cursor-parent.",
		"What are the top 3 performance bottlenecks when running an LLM inference server behind a reverse proxy?",
		"Write a single Go test function that verifies a map has exactly 3 entries with specific keys.",
	}
}
