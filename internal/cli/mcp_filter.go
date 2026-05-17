package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfsarch33/helix-dev-tools/internal/mcpfilter"
	"github.com/spf13/cobra"
)

var mcpFilterCmd = &cobra.Command{
	Use:   "mcp-filter",
	Short: "Generate scoped MCP configs to reduce token exposure",
	Long:  "Apply task profiles to filter MCP servers, reducing schema tokens by 40-80%.",
}

var mcpFilterApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a profile to generate filtered mcp.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		inputPath, _ := cmd.Flags().GetString("input")
		outputPath, _ := cmd.Flags().GetString("output")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		profile, ok := mcpfilter.GetProfile(profileName)
		if !ok {
			return fmt.Errorf("unknown profile: %s (available: %s)", profileName, availableProfiles())
		}

		cfg, err := mcpfilter.LoadMCPConfig(inputPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		filtered, result := mcpfilter.ApplyProfile(cfg, profile)

		if dryRun {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		if outputPath == "" {
			outputPath = filepath.Join(filepath.Dir(inputPath), fmt.Sprintf("mcp-%s.json", profileName))
		}

		if err := mcpfilter.WriteMCPConfig(filtered, outputPath); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Printf("profile=%s in=%d out=%d reduction=%.1f%% output=%s\n",
			result.Profile, result.TotalIn, result.TotalOut, result.ReductionPc, outputPath)
		return nil
	},
}

var mcpFilterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available MCP filter profiles",
	Run: func(cmd *cobra.Command, args []string) {
		profiles := mcpfilter.ListProfiles()
		for _, p := range profiles {
			fmt.Printf("%-14s %s (%d includes)\n", p.Name, p.Description, len(p.Include))
		}
	},
}

var mcpFilterAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze an MCP config and show per-profile reduction",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, _ := cmd.Flags().GetString("input")
		cfg, err := mcpfilter.LoadMCPConfig(inputPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		fmt.Printf("Total MCP servers: %d\n\n", len(cfg.MCPServers))
		fmt.Printf("%-14s %5s %5s %8s\n", "Profile", "Keep", "Drop", "Reduce")
		fmt.Printf("%-14s %5s %5s %8s\n", "-------", "----", "----", "------")

		profiles := mcpfilter.ListProfiles()
		for _, p := range profiles {
			_, result := mcpfilter.ApplyProfile(cfg, p)
			fmt.Printf("%-14s %5d %5d %7.1f%%\n",
				result.Profile, result.TotalOut, result.TotalIn-result.TotalOut, result.ReductionPc)
		}
		return nil
	},
}

func init() {
	mcpFilterApplyCmd.Flags().String("profile", "", "Profile to apply (required)")
	mcpFilterApplyCmd.Flags().String("input", defaultMCPPath(), "Input mcp.json path")
	mcpFilterApplyCmd.Flags().String("output", "", "Output path (default: mcp-<profile>.json in same dir)")
	mcpFilterApplyCmd.Flags().Bool("dry-run", false, "Show result without writing")
	_ = mcpFilterApplyCmd.MarkFlagRequired("profile")

	mcpFilterAnalyzeCmd.Flags().String("input", defaultMCPPath(), "Input mcp.json path")

	mcpFilterCmd.AddCommand(mcpFilterApplyCmd)
	mcpFilterCmd.AddCommand(mcpFilterListCmd)
	mcpFilterCmd.AddCommand(mcpFilterAnalyzeCmd)
}

func defaultMCPPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor", "mcp.json")
}

func availableProfiles() string {
	profiles := mcpfilter.ListProfiles()
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return strings.Join(names, ", ")
}
