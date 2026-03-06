package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var prePushCmd = &cobra.Command{
	Use:   "pre-push [remote]",
	Short: "Block direct pushes to main/master",
	Args:  cobra.ExactArgs(1),
	RunE:  runPrePush,
}

var protectedBranches = regexp.MustCompile(`^(main|master)$`)

func runPrePush(_ *cobra.Command, args []string) error {
	allowMainPush := false
	cmd := exec.Command("git", "config", "--bool", "hooks.allowMainPush")
	out, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(out)) == "true" {
		allowMainPush = true
	}

	if allowMainPush {
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		remoteRef := fields[2]
		branch := strings.TrimPrefix(remoteRef, "refs/heads/")
		if protectedBranches.MatchString(branch) {
			fmt.Fprintf(os.Stderr, "ERROR: direct push to '%s' is blocked.\nUse a feature branch and open a pull request.\nTo opt-out (personal repos): git config hooks.allowMainPush true\n", branch)
			os.Exit(1)
		}
	}

	return nil
}
