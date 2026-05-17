package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nfsarch33/helix-dev-tools/internal/docsync"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

var (
	docsyncRepo               string
	docsyncRequirePublicFiles bool
	docsCheckConfig           string
	docsCheckRepoAliases      []string
)

var docsyncCmd = &cobra.Command{
	Use:   "docsync",
	Short: "Check and repair repository documentation drift",
}

var docsyncCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check one repository for documentation drift",
	RunE: func(cmd *cobra.Command, args []string) error {
		report, err := docsync.AuditRepoWithOptions(docsyncRepo, docsyncOptions())
		if err != nil {
			return err
		}
		printDocsyncReport(cmd, report)
		if !report.OK() {
			return fmt.Errorf("docsync: %d finding(s)", len(report.Findings))
		}
		return nil
	},
}

var docsyncFixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Repair deterministic documentation drift",
	RunE: func(cmd *cobra.Command, args []string) error {
		report, err := docsync.FixRepo(docsyncRepo)
		if err != nil {
			return err
		}
		printDocsyncReport(cmd, report)
		if !report.OK() {
			return fmt.Errorf("docsync: %d finding(s) remain", len(report.Findings))
		}
		return nil
	},
}

var docsyncReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print a documentation drift report",
	RunE: func(cmd *cobra.Command, args []string) error {
		report, err := docsync.AuditRepoWithOptions(docsyncRepo, docsyncOptions())
		if err != nil {
			return err
		}
		printDocsyncReport(cmd, report)
		return nil
	},
}

var docsCheckCmd = &cobra.Command{
	Use:   "docs-check",
	Short: "Compatibility wrapper for docsync check",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(docsCheckRepoAliases) == 0 {
			report, err := docsync.AuditRepoWithOptions(docsyncRepo, docsyncOptions())
			if err != nil {
				return err
			}
			printDocsyncReport(cmd, report)
			if !report.OK() {
				return fmt.Errorf("docs-check: one or more checks failed")
			}
			return nil
		}
		return runDocsCheckAliases(cmd)
	},
}

func init() {
	docsyncCmd.PersistentFlags().StringVar(&docsyncRepo, "repo", ".", "repository root")
	docsyncCmd.PersistentFlags().BoolVar(&docsyncRequirePublicFiles, "public", false, "require public-repo files such as LICENSE and CONTRIBUTING.md")
	docsyncCmd.AddCommand(docsyncCheckCmd, docsyncFixCmd, docsyncReportCmd)
	docsCheckCmd.Flags().StringVar(&docsyncRepo, "repo", ".", "repository root")
	docsCheckCmd.Flags().StringVar(&docsCheckConfig, "config", defaultRunxConfigPath(), "runx config path")
	docsCheckCmd.Flags().BoolVar(&docsyncRequirePublicFiles, "public", false, "require public-repo files such as LICENSE and CONTRIBUTING.md")
	docsCheckCmd.Flags().Bool("global-kb-only", false, "compatibility flag; check the selected repo")
	docsCheckCmd.Flags().Bool("fleet", false, "compatibility flag; fleet mode is handled by runx")
	docsCheckCmd.Flags().StringArrayVar(&docsCheckRepoAliases, "repo-alias", nil, "runx repo alias to include; repeatable")
}

func docsyncOptions() docsync.Options {
	return docsync.Options{RequirePublicFiles: docsyncRequirePublicFiles}
}

func runDocsCheckAliases(cmd *cobra.Command) error {
	ok := true
	for _, alias := range docsCheckRepoAliases {
		root, err := resolveRunxRepoPath(docsCheckConfig, alias)
		if err != nil {
			return err
		}
		report, err := docsync.AuditRepoWithOptions(root, docsyncOptions())
		if err != nil {
			return err
		}
		printDocsyncAliasReport(cmd, alias, report)
		ok = ok && report.OK()
	}
	if !ok {
		return fmt.Errorf("docs-check: one or more checks failed")
	}
	return nil
}

func printDocsyncReport(cmd *cobra.Command, report docsync.Report) {
	out := cmd.OutOrStdout()
	if report.OK() {
		fmt.Fprintln(out, "[PASS] documentation drift check")
	} else {
		fmt.Fprintln(out, "[FAIL] documentation drift check")
	}
	for _, fixed := range report.Fixed {
		fmt.Fprintf(out, "[FIXED] %s\n", fixed)
	}
	for _, finding := range report.Findings {
		fmt.Fprintf(out, "[%s] %s %s\n", finding.Code, finding.Path, finding.Message)
	}
	if len(report.Findings) == 0 && len(report.Fixed) == 0 {
		fmt.Fprintf(out, "[OK] %s\n", displayRepo())
	}
}

func printDocsyncAliasReport(cmd *cobra.Command, alias string, report docsync.Report) {
	out := cmd.OutOrStdout()
	if report.OK() {
		fmt.Fprintf(out, "[PASS] %s\n", alias)
	} else {
		fmt.Fprintf(out, "[FAIL] %s\n", alias)
	}
	for _, finding := range report.Findings {
		fmt.Fprintf(out, "[%s] %s %s\n", finding.Code, finding.Path, finding.Message)
	}
}

func displayRepo() string {
	if _, err := os.Getwd(); err == nil {
		return "."
	}
	return "selected repo"
}

func defaultRunxConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "runx", "config.yaml")
}

func resolveRunxRepoPath(configPath, alias string) (string, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("docs-check: read runx config: %w", err)
	}
	var cfg struct {
		Repos map[string]struct {
			Path string `yaml:"path"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("docs-check: parse runx config: %w", err)
	}
	repo, ok := cfg.Repos[alias]
	if !ok || repo.Path == "" {
		return "", fmt.Errorf("docs-check: unknown repo alias %q", alias)
	}
	return expandRepoPath(repo.Path), nil
}

func expandRepoPath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if len(path) > 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return os.ExpandEnv(path)
}
