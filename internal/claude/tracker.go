package claude

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Option configures a Run invocation.
type Option func(*runConfig)

type runConfig struct {
	model    string
	usageDir string
	timeout  time.Duration
}

// WithModel overrides the model for tracking purposes.
func WithModel(m string) Option {
	return func(c *runConfig) { c.model = m }
}

// WithUsageDir sets the directory for usage logs.
func WithUsageDir(d string) Option {
	return func(c *runConfig) { c.usageDir = d }
}

// WithTimeout sets an execution timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *runConfig) { c.timeout = d }
}

var tokenRe = regexp.MustCompile(`(?i)(\d[\d,]*)\s*(?:input|output|cache.?read|cache.?write)\s*tokens?`)
var costRe = regexp.MustCompile(`(?i)\$\s*([\d.]+)`)

// Run executes Claude CLI with the given prompt and returns the output plus usage.
// It captures stdout as the response and parses stderr for any token/cost info.
func Run(ctx context.Context, prompt string, opts ...Option) (string, Usage, error) {
	cfg := runConfig{
		usageDir: DefaultUsageDir(),
		timeout:  5 * time.Minute,
	}
	for _, o := range opts {
		o(&cfg)
	}

	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt)
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)

	u := Usage{
		Timestamp:   time.Now().UTC(),
		Model:       cfg.model,
		PromptBytes: len(prompt),
		OutputBytes: stdout.Len(),
		DurationMs:  dur.Milliseconds(),
		Backend:     detectBackend(),
		Prompt:      truncate(prompt, 200),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			u.ExitCode = exitErr.ExitCode()
		} else {
			u.ExitCode = -1
		}
		u.Error = truncate(stderr.String(), 500)
	}

	parseTokenInfo(stderr.String(), &u)

	if appendErr := AppendUsage(cfg.usageDir, u); appendErr != nil {
		fmt.Printf("warning: failed to log claude usage: %v\n", appendErr)
	}

	if err != nil {
		return stdout.String(), u, fmt.Errorf("claude cli: %w", err)
	}
	return stdout.String(), u, nil
}

// LookupEnv is a test-friendly wrapper around os.LookupEnv.
var LookupEnv = os.LookupEnv

func detectBackend() string {
	if v, ok := LookupEnv("CLAUDE_CODE_USE_BEDROCK"); ok && strings.EqualFold(v, "true") {
		return "bedrock"
	}
	if _, ok := LookupEnv("ANTHROPIC_API_KEY"); ok {
		return "anthropic"
	}
	return "unknown"
}

func parseTokenInfo(stderr string, u *Usage) {
	lines := strings.Split(stderr, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if matches := tokenRe.FindAllStringSubmatch(line, -1); len(matches) > 0 {
			for _, m := range matches {
				count := parseCount(m[1])
				switch {
				case strings.Contains(lower, "input"):
					u.InputTokens = count
				case strings.Contains(lower, "output"):
					u.OutputTokens = count
				case strings.Contains(lower, "cache") && strings.Contains(lower, "read"):
					u.CacheRead = count
				case strings.Contains(lower, "cache") && strings.Contains(lower, "write"):
					u.CacheWrite = count
				}
			}
		}
		if m := costRe.FindStringSubmatch(line); len(m) > 0 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				u.Cost = v
			}
		}
	}
}

func parseCount(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	v, _ := strconv.Atoi(s)
	return v
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
