// Command replicate-cursor-to-claude-code mirrors the Cursor capability
// layer (skills, agents, hooks, MCP config) into the Claude Code home
// directory.
//
// Default invocation:
//
//	~/bin/replicate-cursor-to-claude-code            # full mirror
//	~/bin/replicate-cursor-to-claude-code --dry-run  # plan only
//	~/bin/replicate-cursor-to-claude-code --skills-only
//	~/bin/replicate-cursor-to-claude-code --agents-only
//	~/bin/replicate-cursor-to-claude-code --no-mcp
//	~/bin/replicate-cursor-to-claude-code --no-hooks
//
// The same logic is also exposed as a cursor-tools subcommand
// (`cursor-tools replicate-cursor-to-claude-code`); both share the
// internal/replicate package so behaviour is identical.
//
// Acceptance gate references: see
// `Code/global-kb/sop/cursor-claude-code-offload-policy.md` § 8.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nfsarch33/helix-dev-tools/internal/replicate"
)

func main() {
	dry := flag.Bool("dry-run", false, "print actions but do not touch disk")
	skillsOnly := flag.Bool("skills-only", false, "mirror skills only")
	agentsOnly := flag.Bool("agents-only", false, "mirror agents only")
	noMCP := flag.Bool("no-mcp", false, "skip MCP config rewrite")
	noHooks := flag.Bool("no-hooks", false, "skip hooks.json symlink")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve home: %v\n", err)
		os.Exit(1)
	}
	opts := replicate.Options{
		CursorHome:     filepath.Join(home, ".cursor"),
		CursorGlobalKB: filepath.Join(home, "Code", "global-kb", "cursor-config"),
		ClaudeHome:     filepath.Join(home, ".claude"),
		SkillsOnly:     *skillsOnly,
		AgentsOnly:     *agentsOnly,
		NoMCP:          *noMCP,
		NoHooks:        *noHooks,
		DryRun:         *dry,
		Out:            os.Stdout,
	}
	if _, err := replicate.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "replicate: %v\n", err)
		os.Exit(1)
	}
}
