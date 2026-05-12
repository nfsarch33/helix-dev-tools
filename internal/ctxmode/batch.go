package ctxmode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IndexTarget represents a file or directory to index via context-mode.
type IndexTarget struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

// BatchIndexConfig holds configuration for batch indexing.
type BatchIndexConfig struct {
	Targets    []IndexTarget `json:"targets"`
	TimeoutSec int           `json:"timeout_sec,omitempty"`
}

// BatchIndexResult records the outcome of indexing a single target.
type BatchIndexResult struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// DefaultConfigPath returns the default path for the session-index config.
func DefaultConfigPath(home string) string {
	return filepath.Join(home, ".cursor", "session-index.json")
}

// LoadConfig reads a BatchIndexConfig from the given path.
func LoadConfig(path string) (*BatchIndexConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg BatchIndexConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("config has no targets")
	}
	return &cfg, nil
}

// DefaultTargets returns the standard set of targets for pre-session indexing.
func DefaultTargets(home string) []IndexTarget {
	globalKB := filepath.Join(home, "Code", "global-kb")
	return []IndexTarget{
		{
			Path:   filepath.Join(globalKB, "daily-startup-prompt.md"),
			Source: "global-kb: daily-startup-prompt",
		},
		{
			Path:   filepath.Join(globalKB, "backlog", "roadmap-v358-v367.md"),
			Source: "global-kb: active roadmap",
		},
		{
			Path:   filepath.Join(globalKB, "sop", "sprint-execution-sop.md"),
			Source: "global-kb: sprint execution SOP",
		},
	}
}

// RunBatchIndex indexes all targets using the context-mode MCP ctx_index tool
// via the context-mode CLI. Falls back to direct file reading if the CLI is
// unavailable.
func RunBatchIndex(targets []IndexTarget, timeoutSec int) []BatchIndexResult {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	results := make([]BatchIndexResult, 0, len(targets))
	for _, t := range targets {
		r := indexTarget(t, time.Duration(timeoutSec)*time.Second)
		results = append(results, r)
	}
	return results
}

func indexTarget(t IndexTarget, timeout time.Duration) BatchIndexResult {
	resolved := expandHome(t.Path)

	if _, err := os.Stat(resolved); err != nil {
		return BatchIndexResult{
			Source: t.Source,
			Path:   t.Path,
			Status: "skipped",
			Error:  "file not found",
		}
	}

	ctxMode, err := exec.LookPath("context-mode")
	if err != nil {
		return BatchIndexResult{
			Source: t.Source,
			Path:   t.Path,
			Status: "skipped",
			Error:  "context-mode CLI not on PATH",
		}
	}

	args := []string{"index", "--path", resolved, "--source", t.Source}
	cmd := exec.Command(ctxMode, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("CTX_TIMEOUT=%d", int(timeout.Seconds())))

	out, err := cmd.CombinedOutput()
	if err != nil {
		return BatchIndexResult{
			Source: t.Source,
			Path:   t.Path,
			Status: "error",
			Error:  fmt.Sprintf("%s: %s", err, strings.TrimSpace(string(out))),
		}
	}

	return BatchIndexResult{
		Source: t.Source,
		Path:   t.Path,
		Status: "indexed",
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
