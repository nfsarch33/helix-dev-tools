package mem0perf

import (
	"fmt"
	"strings"
	"time"
)

// BenchmarkResult captures the outcome of a single Mem0 operation timing.
type BenchmarkResult struct {
	Operation  string
	Duration   time.Duration
	Success    bool
	StatusCode int
	Error      string
}

// OperationStats aggregates statistics for a single operation type.
type OperationStats struct {
	Count        int
	SuccessCount int
	AvgDuration  time.Duration
	MinDuration  time.Duration
	MaxDuration  time.Duration
}

// ThresholdViolation records when an operation exceeds its target latency.
type ThresholdViolation struct {
	Operation string
	Actual    time.Duration
	Threshold time.Duration
}

// Diagnosis captures root-cause analysis from benchmark results.
type Diagnosis struct {
	EmbeddingBottleneck bool
	LLMBottleneck       bool
	NetworkBottleneck   bool
	Recommendations     []string
}

// PerformanceReport is the structured output of a benchmark run.
type PerformanceReport struct {
	Endpoint   string
	Timestamp  time.Time
	Summary    map[string]OperationStats
	Violations []ThresholdViolation
	Diagnosis  Diagnosis
}

// BenchmarkSuite manages a collection of benchmark results for Mem0.
type BenchmarkSuite struct {
	Endpoint string
	Results  []BenchmarkResult
}

// NewBenchmarkSuite creates a suite targeting a Mem0 endpoint.
func NewBenchmarkSuite(endpoint string) *BenchmarkSuite {
	return &BenchmarkSuite{
		Endpoint: endpoint,
		Results:  make([]BenchmarkResult, 0),
	}
}

// AddResult appends a benchmark result.
func (s *BenchmarkSuite) AddResult(r BenchmarkResult) {
	s.Results = append(s.Results, r)
}

// Summary computes per-operation statistics.
func (s *BenchmarkSuite) Summary() map[string]OperationStats {
	groups := make(map[string][]BenchmarkResult)
	for _, r := range s.Results {
		groups[r.Operation] = append(groups[r.Operation], r)
	}

	summary := make(map[string]OperationStats, len(groups))
	for op, results := range groups {
		stats := OperationStats{
			Count:       len(results),
			MinDuration: results[0].Duration,
			MaxDuration: results[0].Duration,
		}
		var totalDuration time.Duration
		for _, r := range results {
			totalDuration += r.Duration
			if r.Success {
				stats.SuccessCount++
			}
			if r.Duration < stats.MinDuration {
				stats.MinDuration = r.Duration
			}
			if r.Duration > stats.MaxDuration {
				stats.MaxDuration = r.Duration
			}
		}
		stats.AvgDuration = totalDuration / time.Duration(stats.Count)
		summary[op] = stats
	}
	return summary
}

// DefaultThresholds returns the target latency per operation.
func DefaultThresholds() map[string]time.Duration {
	return map[string]time.Duration{
		"healthz":      500 * time.Millisecond,
		"add_infer":    2 * time.Second,
		"add_no_infer": 500 * time.Millisecond,
		"search":       2 * time.Second,
		"get_all":      1 * time.Second,
	}
}

// CheckThresholds identifies operations that exceed their target latency.
func (s *BenchmarkSuite) CheckThresholds(thresholds map[string]time.Duration) []ThresholdViolation {
	summary := s.Summary()
	var violations []ThresholdViolation
	for op, stats := range summary {
		if threshold, ok := thresholds[op]; ok {
			if stats.AvgDuration > threshold {
				violations = append(violations, ThresholdViolation{
					Operation: op,
					Actual:    stats.AvgDuration,
					Threshold: threshold,
				})
			}
		}
	}
	return violations
}

// Diagnose performs root-cause analysis on the benchmark results.
func (s *BenchmarkSuite) Diagnose() Diagnosis {
	summary := s.Summary()
	d := Diagnosis{}

	healthzOK := true
	if stats, ok := summary["healthz"]; ok {
		if stats.AvgDuration > 2*time.Second || stats.SuccessCount == 0 {
			d.NetworkBottleneck = true
			healthzOK = false
		}
	}

	if healthzOK {
		if stats, ok := summary["add_infer"]; ok {
			if stats.AvgDuration > 10*time.Second || stats.SuccessCount < stats.Count {
				d.LLMBottleneck = true
			}
		}
		if stats, ok := summary["search"]; ok {
			if stats.AvgDuration > 10*time.Second || stats.SuccessCount < stats.Count {
				d.EmbeddingBottleneck = true
			}
		}
	}

	if d.LLMBottleneck {
		d.Recommendations = append(d.Recommendations, "Switch MiniMax model from M2.1 to m2-7-highspeed")
		d.Recommendations = append(d.Recommendations, "Consider local vLLM as LLM backend instead of external API")
	}
	if d.EmbeddingBottleneck {
		d.Recommendations = append(d.Recommendations, "Enable CUDA for Qwen embedding bridge")
		d.Recommendations = append(d.Recommendations, "Verify qwen-embedding-bridge container is running")
	}
	if d.NetworkBottleneck {
		d.Recommendations = append(d.Recommendations, "Check tunnel connectivity (runx tunnel status)")
		d.Recommendations = append(d.Recommendations, "Verify Caddy reverse proxy is healthy")
	}

	return d
}

// GenerateReport produces a structured performance report.
func (s *BenchmarkSuite) GenerateReport() PerformanceReport {
	return PerformanceReport{
		Endpoint:   s.Endpoint,
		Timestamp:  time.Now(),
		Summary:    s.Summary(),
		Violations: s.CheckThresholds(DefaultThresholds()),
		Diagnosis:  s.Diagnose(),
	}
}

// ToMarkdown renders the report as markdown.
func (r *PerformanceReport) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# Mem0 OSS Performance Report\n\n")
	sb.WriteString(fmt.Sprintf("**Endpoint**: %s\n", r.Endpoint))
	sb.WriteString(fmt.Sprintf("**Timestamp**: %s\n\n", r.Timestamp.Format(time.RFC3339)))

	sb.WriteString("## Operation Summary\n\n")
	sb.WriteString("| Operation | Count | Success | Avg | Min | Max |\n")
	sb.WriteString("|---|---|---|---|---|---|\n")
	for op, stats := range r.Summary {
		sb.WriteString(fmt.Sprintf("| %s | %d | %d/%d | %s | %s | %s |\n",
			op, stats.Count, stats.SuccessCount, stats.Count,
			stats.AvgDuration.Round(time.Millisecond),
			stats.MinDuration.Round(time.Millisecond),
			stats.MaxDuration.Round(time.Millisecond)))
	}

	if len(r.Violations) > 0 {
		sb.WriteString("\n## Threshold Violations\n\n")
		for _, v := range r.Violations {
			sb.WriteString(fmt.Sprintf("- **%s**: %s (threshold: %s)\n", v.Operation, v.Actual, v.Threshold))
		}
	}

	if len(r.Diagnosis.Recommendations) > 0 {
		sb.WriteString("\n## Diagnosis\n\n")
		if r.Diagnosis.EmbeddingBottleneck {
			sb.WriteString("- Embedding bottleneck detected\n")
		}
		if r.Diagnosis.LLMBottleneck {
			sb.WriteString("- LLM inference bottleneck detected\n")
		}
		if r.Diagnosis.NetworkBottleneck {
			sb.WriteString("- Network/tunnel bottleneck detected\n")
		}
		sb.WriteString("\n### Recommendations\n\n")
		for _, rec := range r.Diagnosis.Recommendations {
			sb.WriteString(fmt.Sprintf("- %s\n", rec))
		}
	}

	return sb.String()
}
