package cli

import (
	"fmt"
	"os"

	"github.com/nfsarch33/helix-dev-tools/internal/importmigrate"
	"github.com/spf13/cobra"
)

var importMigrateCmd = &cobra.Command{
	Use:   "import-migrate",
	Short: "Rewrite Go import paths across a module tree",
	Long: `import-migrate walks all .go files under a directory and replaces every
occurrence of --old-prefix with --new-prefix.

Atomic write semantics: each file is written to a temp file then renamed,
so a partial failure cannot corrupt a source file.

go.mod, vendor/, .git/, and node_modules/ are excluded from rewriting.
Patch go.mod separately after running this command.

Examples:
  # Dry-run to preview changes
  cursor-tools import-migrate \
    --root ~/ai-agent-business-stack/go \
    --old-prefix github.com/nfsarch33/ai-agent-business-stack/go \
    --new-prefix github.com/nfsarch33/helixon-ec/go \
    --dry-run

  # Apply migration
  cursor-tools import-migrate \
    --root ~/ai-agent-business-stack/go \
    --old-prefix github.com/nfsarch33/ai-agent-business-stack/go \
    --new-prefix github.com/nfsarch33/helixon-ec/go`,
	RunE: runImportMigrate,
}

func init() {
	importMigrateCmd.Flags().String("root", ".", "Root directory to migrate")
	importMigrateCmd.Flags().String("old-prefix", "", "Import path prefix to replace (required)")
	importMigrateCmd.Flags().String("new-prefix", "", "Import path prefix to substitute in (required)")
	importMigrateCmd.Flags().Bool("dry-run", false, "Report what would change without writing files")
	importMigrateCmd.Flags().Int("concurrency", 0, "Worker goroutines (0 = GOMAXPROCS)")
	_ = importMigrateCmd.MarkFlagRequired("old-prefix")
	_ = importMigrateCmd.MarkFlagRequired("new-prefix")
}

func runImportMigrate(cmd *cobra.Command, _ []string) error {
	root, _ := cmd.Flags().GetString("root")
	oldPrefix, _ := cmd.Flags().GetString("old-prefix")
	newPrefix, _ := cmd.Flags().GetString("new-prefix")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	concurrency, _ := cmd.Flags().GetInt("concurrency")

	// Expand home dir shorthand.
	if len(root) > 1 && root[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		root = home + root[1:]
	}

	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "[dry-run] no files will be modified")
	}

	result, err := importmigrate.Migrate(importmigrate.Config{
		Root:        root,
		OldPrefix:   oldPrefix,
		NewPrefix:   newPrefix,
		DryRun:      dryRun,
		Concurrency: concurrency,
	})
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), result.Summary())
	if dryRun && result.FilesChanged > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Run without --dry-run to apply changes.")
	}
	return nil
}
