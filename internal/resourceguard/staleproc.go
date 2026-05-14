// Package resourceguard detects orphan MCP server processes that
// outlive their owning Cursor session. Known MCP binaries (mem0-mcp-go,
// sentrux, pdf-mcp-server, etc.) are scanned; those whose PPID is 1
// (orphaned / reparented to init/launchd) or whose elapsed wall-clock
// time exceeds a configurable threshold are reported as stale.
package resourceguard

import (
	"fmt"
	"time"
)

// DefaultThreshold is the elapsed-time ceiling above which a matched
// process is considered stale regardless of PPID.
const DefaultThreshold = 4 * time.Hour

// KnownMCPBinaries is the default set of process name substrings used
// to identify MCP server processes. Callers may override via
// DetectorConfig.BinaryNames.
var KnownMCPBinaries = []string{
	"mem0-mcp-go",
	"sentrux",
	"pdf-mcp-server",
	"ironclaw-mcp",
	"cursor-tools",
}

// ProcessInfo describes a running process as seen by the detector.
type ProcessInfo struct {
	PID     int           `json:"pid"`
	PPID    int           `json:"ppid"`
	Name    string        `json:"name"`
	Elapsed time.Duration `json:"elapsed"`
}

// StaleProcess is a ProcessInfo that matched the stale criteria.
type StaleProcess struct {
	ProcessInfo
	Reason string `json:"reason"`
}

// ProcessLister abstracts the OS process table so tests can inject
// fakes without touching real processes.
type ProcessLister interface {
	ListProcesses() ([]ProcessInfo, error)
}

// DetectorConfig configures the stale-process detector.
type DetectorConfig struct {
	BinaryNames []string
	Threshold   time.Duration
}

// Detector scans for orphan MCP server processes.
type Detector struct {
	lister ProcessLister
	cfg    DetectorConfig
}

// NewDetector returns a Detector that reads processes from lister.
func NewDetector(lister ProcessLister, cfg DetectorConfig) *Detector {
	if len(cfg.BinaryNames) == 0 {
		cfg.BinaryNames = KnownMCPBinaries
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultThreshold
	}
	return &Detector{lister: lister, cfg: cfg}
}

// Scan returns all processes matching known MCP binary names whose
// PPID is 1 (orphaned) or whose elapsed time exceeds the threshold.
func (d *Detector) Scan() ([]StaleProcess, error) {
	procs, err := d.lister.ListProcesses()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}

	var stale []StaleProcess
	for _, p := range procs {
		if !d.matchesBinaryName(p.Name) {
			continue
		}
		if p.PPID == 1 {
			stale = append(stale, StaleProcess{
				ProcessInfo: p,
				Reason:      "orphaned (PPID=1)",
			})
			continue
		}
		if p.Elapsed > d.cfg.Threshold {
			stale = append(stale, StaleProcess{
				ProcessInfo: p,
				Reason:      fmt.Sprintf("elapsed %s >= threshold %s", p.Elapsed.Truncate(time.Second), d.cfg.Threshold),
			})
		}
	}
	return stale, nil
}

func (d *Detector) matchesBinaryName(name string) bool {
	for _, bin := range d.cfg.BinaryNames {
		if containsSubstring(name, bin) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
