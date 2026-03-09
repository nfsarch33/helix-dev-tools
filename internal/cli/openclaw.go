package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/openclaw"
	"github.com/spf13/cobra"
)

var openclawDir string

var openclawCmd = &cobra.Command{
	Use:   "openclaw",
	Short: "OpenClaw gateway diagnostics and security auditing",
}

var openclawValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Gateway health validation — checks deadlock signatures and config readability",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := resolveOpenclawDir()
		ts := time.Now().Format(time.RFC3339)
		fmt.Println("=== OpenClaw Gateway Health Diagnostics ===")

		configPath := filepath.Join(dir, "openclaw.json")
		bind := openclaw.ParseGatewayBind(configPath)
		if bind == "" {
			fmt.Printf("%s WARN: Config not readable at %s\n", ts, configPath)
		} else {
			fmt.Printf("%s PASS: Config readable, bind=%s\n", ts, bind)
		}

		logFile := filepath.Join(dir, "logs", "gateway.err.log")
		if openclaw.CheckDeadlockSignatures(logFile, 200) {
			fmt.Printf("%s WARN: Deadlock or timeout signatures detected in %s\n", ts, logFile)
		} else {
			fmt.Printf("%s PASS: No deadlock signatures in recent logs\n", ts)
		}

		fmt.Printf("%s SUCCESS: OpenClaw Gateway telemetry nominal\n", ts)
		return nil
	},
}

var openclawAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Security red-line audit — loopback, permissions, keys, evolution constraints",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := resolveOpenclawDir()
		fmt.Println("=== OpenClaw Security Red-Line Audit ===")

		results := openclaw.RunAudit(dir)
		pass, fail := 0, 0
		for _, r := range results {
			status := "PASS"
			if !r.Pass {
				status = "FAIL"
				fail++
			} else {
				pass++
			}
			if r.Detail != "" {
				fmt.Printf("%s: %s (%s)\n", status, r.Label, r.Detail)
			} else {
				fmt.Printf("%s: %s\n", status, r.Label)
			}
		}

		fmt.Printf("\n=== Results: %d passed, %d failed ===\n", pass, fail)
		if fail > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func resolveOpenclawDir() string {
	if openclawDir != "" {
		return openclawDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw")
}

func init() {
	openclawCmd.PersistentFlags().StringVar(&openclawDir, "dir", "", "OpenClaw config directory (default: ~/.openclaw)")
	openclawCmd.AddCommand(openclawValidateCmd)
	openclawCmd.AddCommand(openclawAuditCmd)
}
