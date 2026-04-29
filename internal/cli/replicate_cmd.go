package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/replicate"
)

var (
	replicateDryRun     bool
	replicateSkillsOnly bool
	replicateAgentsOnly bool
	replicateNoMCP      bool
	replicateNoHooks    bool
	replicateNoSkills   bool
	replicateNoAgents   bool
)

var replicateCmd = &cobra.Command{
	Use:   "replicate-cursor-to-claude-code",
	Short: "Mirror the Cursor capability layer (skills, agents, hooks, MCP) into ~/.claude/",
	Long: `Replicate the Cursor capability layer into the Claude Code home directory.

Skills + agents are exposed as symlinks (so a single edit to the source file
flows to both Cursor and Claude Code). hooks.json is symlinked verbatim.
mcp.json is rewritten with cursor-only entries (e.g. "test-mcp", disabled
servers) filtered out.

Idempotent: re-running this command does nothing when targets already point
at the right source. Conflicting non-symlink files at the sink are renamed
to "<target>.bak.<UTC>" before being replaced, so an operator override of an
installed skill is preserved as a backup, not silently clobbered.

See: Code/global-kb/sop/cursor-claude-code-offload-policy.md`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		opts := replicate.Options{
			CursorHome:     filepath.Join(home, ".cursor"),
			CursorGlobalKB: filepath.Join(home, "Code", "global-kb", "cursor-config"),
			ClaudeHome:     filepath.Join(home, ".claude"),
			SkillsOnly:     replicateSkillsOnly,
			AgentsOnly:     replicateAgentsOnly,
			NoMCP:          replicateNoMCP,
			NoHooks:        replicateNoHooks,
			NoSkills:       replicateNoSkills,
			NoAgents:       replicateNoAgents,
			DryRun:         replicateDryRun,
			Out:            cmd.OutOrStdout(),
		}
		_, err = replicate.Run(opts)
		return err
	},
}

func init() {
	replicateCmd.Flags().BoolVar(&replicateDryRun, "dry-run", false, "Print actions but do not touch disk")
	replicateCmd.Flags().BoolVar(&replicateSkillsOnly, "skills-only", false, "Mirror skills only")
	replicateCmd.Flags().BoolVar(&replicateAgentsOnly, "agents-only", false, "Mirror agents only")
	replicateCmd.Flags().BoolVar(&replicateNoMCP, "no-mcp", false, "Skip MCP config rewrite")
	replicateCmd.Flags().BoolVar(&replicateNoHooks, "no-hooks", false, "Skip hooks.json symlink")
	replicateCmd.Flags().BoolVar(&replicateNoSkills, "no-skills", false, "Skip skills mirror (use when ~/.claude/skills is already directory-symlinked)")
	replicateCmd.Flags().BoolVar(&replicateNoAgents, "no-agents", false, "Skip agents mirror (use when ~/.claude/agents is already directory-symlinked)")
}
