// runx-public-repo-gate: allow-file personal_path_id — identity gate detects literal personal-stack identifiers, so the strings must remain in source

// Package cli — git pre-commit handler.
//
// This hook enforces the v254 sprint cross-cutting policy:
//
//	"All personal repos MUST be committed with the nfsarch33 GitHub
//	identity (jaslian@gmail.com). The Zendesk work identity
//	(work-identity@company.example) MUST NEVER land on a personal repo."
//
// Mechanism:
//
//   - Inspect `git config user.email` (the value `git commit` will use
//     for this commit).
//   - If the email looks like a Zendesk work email (matches the
//     compiled-in deny pattern, case-insensitive), abort the commit
//     with a friendly remediation message.
//   - Allow opt-out per repository with `git config hooks.allowZendeskIdentity true`
//     so work clones (~/Code/secure-auth-platform etc.) keep working.
//   - Reject empty `user.email` because git would otherwise
//     fall back to system defaults that often include the work
//     identity.
//
// The hook is intentionally narrow: it does NOT scan staged content
// for Zendesk strings (research notes legitimately mention them). It
// only gates the commit *author*, which is the actual policy boundary.
package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var preCommitExit = os.Exit
var preCommitStderr io.Writer = os.Stderr

var preCommitCmd = &cobra.Command{
	Use:   "pre-commit",
	Short: "Block commits authored by the Zendesk work identity on personal repos",
	Args:  cobra.NoArgs,
	RunE:  runPreCommit,
}

// denyZendeskIdentity matches any email containing the literal
// substring "zendesk" (case-insensitive). The work account is
// work-identity@company.example but staff emails like j.lian@zendesk.com
// would be just as wrong on a personal repo, so we match the domain
// substring rather than a specific local-part.
var denyZendeskIdentity = regexp.MustCompile(`(?i)zendesk`)

// gitConfigGetter is the seam tests use to fake `git config user.email`
// without forking. The real implementation just shells out.
var gitConfigGetter = realGitConfig

func realGitConfig(key string) (string, error) {
	out, err := exec.Command("git", "config", "--get", key).Output()
	if err != nil {
		// `git config --get` returns exit 1 when the key is unset;
		// callers treat that as "empty string, no error".
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runPreCommit(_ *cobra.Command, _ []string) error {
	allow, _ := gitConfigGetter("hooks.allowZendeskIdentity")
	if strings.EqualFold(strings.TrimSpace(allow), "true") {
		return nil
	}

	email, err := gitConfigGetter("user.email")
	if err != nil {
		fmt.Fprintf(preCommitStderr, "ERROR: cannot read git config user.email: %v\n", err)
		preCommitExit(1)
		return nil
	}
	email = strings.TrimSpace(email)
	if email == "" {
		fmt.Fprint(preCommitStderr, "ERROR: git config user.email is empty.\n"+
			"Personal repos require: git config user.email \"jaslian@gmail.com\"\n"+
			"To opt-out (work clones): git config hooks.allowZendeskIdentity true\n")
		preCommitExit(1)
		return nil
	}

	if denyZendeskIdentity.MatchString(email) {
		fmt.Fprintf(preCommitStderr,
			"ERROR: refusing to commit with Zendesk identity %q on a personal repo.\n"+
				"Personal repos must use the nfsarch33 GitHub identity:\n"+
				"  git config user.email \"jaslian@gmail.com\"\n"+
				"  git config user.name  \"Jason Lian\"\n"+
				"To opt-out (intentional work clone): git config hooks.allowZendeskIdentity true\n",
			email)
		preCommitExit(1)
		return nil
	}

	return nil
}
