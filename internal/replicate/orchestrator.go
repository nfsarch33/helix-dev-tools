package replicate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Options controls one Replicate run.
//
// CursorHome is typically "$HOME/.cursor" (resolved by the caller).
// CursorGlobalKB is "$HOME/Code/global-kb/cursor-config" (where the
// canonical cursor-tools symlinks point).
// ClaudeHome is "$HOME/.claude".
//
// SkillsOnly / AgentsOnly / NoMCP / NoHooks gate the four mirror
// categories independently.
//
// DryRun forwards to the Applier; nothing on disk changes.
type Options struct {
	CursorHome     string
	CursorGlobalKB string
	ClaudeHome     string

	SkillsOnly bool
	AgentsOnly bool
	NoMCP      bool
	NoHooks    bool
	NoSkills   bool
	NoAgents   bool
	DryRun     bool

	// Out is where the orchestrator writes a one-line summary per
	// action. Tests inject a bytes.Buffer; the CLI passes os.Stdout.
	Out io.Writer
}

// Run is the canonical entrypoint exposed to both the cursor-tools
// subcommand and the standalone binary. It assembles a fresh-FS
// Source, builds the Plan, samples the existing sink targets to
// classify SKIP/BACKUP correctly, applies the plan, and writes a
// human-readable summary to opts.Out.
//
// It returns the slice of executed Actions and any non-nil error.
// Errors short-circuit the per-action loop only at the planner level
// (e.g. unreadable cursor catalog); applier-level errors are recorded
// in the returned Action slice and the first error is returned so the
// CLI can exit non-zero without losing the rest of the action log.
func Run(opts Options) ([]Action, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.ClaudeHome == "" {
		return nil, fmt.Errorf("ClaudeHome is required")
	}

	hooksFile := filepath.Join(opts.CursorHome, "hooks.json")
	mcpFile := filepath.Join(opts.CursorHome, "mcp.json")
	src := NewFSSource(hooksFile, mcpFile)

	skillsDir := filepath.Join(opts.ClaudeHome, "skills")
	agentsDir := filepath.Join(opts.ClaudeHome, "agents")
	hooksTarget := filepath.Join(opts.ClaudeHome, "hooks.json")
	mcpTarget := filepath.Join(opts.ClaudeHome, "mcp.json")
	existing := SampleExisting(skillsDir, agentsDir, hooksTarget, mcpTarget)

	planner := Planner{ClaudeRoot: opts.ClaudeHome, ExistingTargets: existing}
	plan := planner.Build(srcBoundary{src, opts})

	// Safety: if the operator already directory-symlinked
	// <ClaudeHome>/skills or <ClaudeHome>/agents into the source repo,
	// per-file replication would resolve through the symlink and back up
	// the source files themselves. Detect this and replace the per-file
	// plan with a single SKIP action so the operator gets a clear log
	// line ("already directory-symlinked") instead of a destructive run.
	if rel, ok := dirSymlinkResolves(skillsDir); ok {
		plan.Skills = []Action{{
			Op:     OpSkip,
			Source: rel,
			Target: skillsDir,
			Reason: "skills/ is a directory symlink; per-file mirror skipped",
		}}
	}
	if rel, ok := dirSymlinkResolves(agentsDir); ok {
		plan.Agents = []Action{{
			Op:     OpSkip,
			Source: rel,
			Target: agentsDir,
			Reason: "agents/ is a directory symlink; per-file mirror skipped",
		}}
	}

	if opts.SkillsOnly {
		plan.Agents = nil
		plan.Hooks = nil
		plan.MCP = nil
	}
	if opts.AgentsOnly {
		plan.Skills = nil
		plan.Hooks = nil
		plan.MCP = nil
	}
	if opts.NoMCP {
		plan.MCP = nil
	}
	if opts.NoHooks {
		plan.Hooks = nil
	}
	if opts.NoSkills {
		plan.Skills = nil
	}
	if opts.NoAgents {
		plan.Agents = nil
	}

	var filtered []byte
	if len(plan.MCP) > 0 {
		raw, err := src.MCPRaw()
		if err != nil {
			return nil, fmt.Errorf("mcp read: %w", err)
		}
		filtered, err = FilterMCP(raw)
		if err != nil {
			return nil, fmt.Errorf("mcp filter: %w", err)
		}
	}
	app := &Applier{
		DryRun:      opts.DryRun,
		FilteredMCP: filtered,
	}

	out, err := app.Apply(plan)
	for _, a := range out {
		fmt.Fprintf(opts.Out, "%-8s %s -> %s  %s\n", a.Op, a.Source, a.Target, a.Reason)
	}
	return out, err
}

// srcBoundary wraps the FSSource to override the cursor catalogue
// roots so the orchestrator doesn't depend on the planner's relative-
// path heuristic. We rebind the planner's first call to Skills() to a
// list that uses the operator-supplied paths.
type srcBoundary struct {
	inner *FSSource
	opts  Options
}

func (b srcBoundary) Skills(_ []string) ([]SkillEntry, error) {
	return b.inner.Skills([]string{
		filepath.Join(b.opts.CursorGlobalKB, "skills"),
		filepath.Join(b.opts.CursorHome, "skills-cursor"),
	})
}
func (b srcBoundary) SkillCollisions() []SkillEntry { return b.inner.SkillCollisions() }
func (b srcBoundary) Agents(_ string) ([]AgentEntry, error) {
	return b.inner.Agents(filepath.Join(b.opts.CursorGlobalKB, "agents"))
}
func (b srcBoundary) HooksPath() string       { return b.inner.HooksPath() }
func (b srcBoundary) MCPRaw() ([]byte, error) { return b.inner.MCPRaw() }

// dirSymlinkResolves returns the symlink destination if path itself is
// a symlink (Lstat reports ModeSymlink). The boolean is false in every
// other case -- file does not exist, regular dir, regular file. Used by
// the orchestrator to detect operator-level directory symlinks and
// suppress the per-file plan that would otherwise back up source files.
func dirSymlinkResolves(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", false
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}
	dest, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	return dest, true
}
