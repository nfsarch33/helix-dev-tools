package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var onboardVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify all system components are installed and healthy",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()

		checks := []struct {
			Name   string
			Check  func() bool
			Fix    string
		}{
			{"helix-dev-tools binary", func() bool { return checkHelixBinary(filepath.Join(home, "bin")) }, "make install-helixon  # installs helix-dev-tools + cursor-tools symlink"},
			{"runx binary", func() bool { return fileExists(filepath.Join(home, "runs", "runx")) }, "cd ~/runs/runx-src && go build -o ~/runs/runx ."},
			{"mem0-mcp-go binary", func() bool { return fileExists(filepath.Join(home, "runs", "mem0-mcp-go")) }, "cd ~/Code/personal/mem0-mcp-go && go build -o ~/runs/mem0-mcp-go ./cmd/mem0-mcp-go/"},
			{"sprintboard-mcp binary", func() bool { return fileExists(filepath.Join(home, "runs", "sprintboard-mcp")) }, "go build -o ~/runs/sprintboard-mcp ./cmd/sprintboard-mcp/"},
			{"runx config", func() bool { return fileExists(filepath.Join(home, ".config", "runx", "config.yaml")) }, "cp ~/Code/global-kb/cursor-config/runx-config-template.yaml ~/.config/runx/config.yaml"},
			{"sprintboard db", func() bool { return fileExists(filepath.Join(home, ".config", "helix-dev-tools", "sprintboard.db")) }, "cursor-tools sprint create --id test --name test"},
			{"Go version", func() bool { _, err := exec.LookPath("go"); return err == nil }, "brew install go or install gvm"},
			{"git identity", func() bool {
				out, _ := exec.Command("git", "config", "user.email").Output()
				return len(out) > 0
			}, "git config --global user.email <email>"},
		}

		fmt.Printf("%-30s %s\n", "Component", "Status")
		fmt.Println("---")

		allPass := true
		for _, c := range checks {
			status := "PASS"
			if !c.Check() {
				status = "FAIL"
				allPass = false
			}
			fmt.Printf("%-30s %s\n", c.Name, status)
			if status == "FAIL" {
				fmt.Printf("  Fix: %s\n", c.Fix)
			}
		}

		if allPass {
			fmt.Println("\nAll components verified.")
		} else {
			fmt.Println("\nSome components need attention.")
			return fmt.Errorf("verification failed")
		}
		return nil
	},
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// checkHelixBinary returns true when binDir contains either helix-dev-tools
// (canonical name) or cursor-tools (backward-compat symlink or binary).
// This supports the 30-day burn-in where both names coexist.
func checkHelixBinary(binDir string) bool {
	return fileExists(filepath.Join(binDir, "helix-dev-tools")) ||
		fileExists(filepath.Join(binDir, "cursor-tools"))
}

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "System onboarding and verification",
}

func init() {
	onboardCmd.AddCommand(onboardVerifyCmd)
}
