// Package cli — git pre-push handler.
//
// This hook layers two policies:
//
//  1. Direct push to main/master is blocked unless the worktree opts in
//     with `git config hooks.allowMainPush true` (legacy behaviour).
//
//  2. On personal repos (nfsarch33/* remotes) the strict identity gate
//     runs and aborts the push if any poisoned GITHUB_TOKEN-style env
//     vars are present, the user.email is empty, or the email is not
//     the personal nfsarch33 identity. The Zendesk work clones are
//     never gated.
//
// Policy 2 is the v257 W2 sprint deliverable and the corresponding
// guidance in `sop/personal-repo-identity.md`.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var prePushExit = os.Exit
var prePushStderr io.Writer = os.Stderr
var prePushStdin io.Reader

var (
	identityGateEvaluator = evaluateIdentityGateStrict
	identityGateGatherer  = gatherIdentityGateState
	allowMainPushGetter   = readAllowMainPush
)

var prePushCmd = &cobra.Command{
	Use:           "pre-push [remote] [url]",
	Short:         "Block direct pushes to main/master and enforce personal-repo identity gate",
	Args:          cobra.RangeArgs(1, 2),
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE:          runPrePush,
}

var protectedBranches = regexp.MustCompile(`^(main|master)$`)

func readAllowMainPush() bool {
	cmd := exec.Command("git", "config", "--bool", "hooks.allowMainPush")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func runPrePush(_ *cobra.Command, _ []string) error {
	if failures := identityGateEvaluator(identityGateGatherer()); len(failures) > 0 {
		fmt.Fprintln(prePushStderr, "ERROR: cursor-tools identity gate FAILED for personal repo push:")
		for _, f := range failures {
			fmt.Fprintln(prePushStderr, "  - "+f)
		}
		fmt.Fprintln(prePushStderr,
			"\nRemediation:\n"+
				"  unset GITHUB_TOKEN GITHUB_API_TOKEN HOMEBREW_GITHUB_API_TOKEN VENDIR_GITHUB_API_TOKEN\n"+
				"  git config user.email jaslian@gmail.com\n"+
				"  git config user.name 'Jason Lian'\n"+
				"  cursor-tools doctor identity --strict   # confirm gate is green\n"+
				"To opt-out (rare): git config hooks.allowMainPush true (only for main-branch protection bypass; identity gate has no opt-out)")
		prePushExit(1)
		return nil
	}

	if !allowMainPushGetter() {
		stdin := prePushStdin
		if stdin == nil {
			stdin = os.Stdin
		}
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 3 {
				continue
			}
			remoteRef := fields[2]
			branch := strings.TrimPrefix(remoteRef, "refs/heads/")
			if protectedBranches.MatchString(branch) {
				fmt.Fprintf(prePushStderr,
					"ERROR: direct push to '%s' is blocked.\n"+
						"Use a feature branch and open a pull request.\n"+
						"To opt-out (personal repos): git config hooks.allowMainPush true\n",
					branch)
				prePushExit(1)
				return nil
			}
		}
	}

	return nil
}
