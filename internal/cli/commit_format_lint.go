package cli

import (
	"regexp"
	"strings"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
)

// strictConventionalCommit is the v318-3 pre-flight regex. It is
// strictly tighter than `conventionalFormat` in `githook_commit_msg.go`:
//
//   - The type list is enumerated (no arbitrary lowercase tokens).
//   - The scope, if present, must be a single `[a-z0-9_-]+` token —
//     no commas, no spaces, no other separators.
//   - The bare "<type>: <subject>" form (no parens) is also valid.
//
// The post-staged commit-msg hook keeps its own broader regex; this
// one fronts the IDE-hook surface so multi-scope rejections are
// surfaced BEFORE staging or message-typing burns operator effort.
var strictConventionalCommit = regexp.MustCompile(
	`^(feat|fix|docs|refactor|test|chore|perf|build|ci|style|revert)(\([a-z0-9_-]+\))?: \S.*$`,
)

// commitMsgBypassPrefixes mirrors the bypasses in `runCommitMsg` so the
// pre-flight surface and the post-staged hook agree on what is exempt.
var commitMsgBypassPrefixes = []string{
	"Merge ",
	"auto: ",
}

// IsConventionalCommitInvalid returns true when the supplied commit
// message subject line MUST be rejected by the v318-3 pre-flight
// linter.
//
// The function inspects only the FIRST line (everything before the
// first newline). Bypass prefixes (Merge / auto:) always return false.
//
// Rules enforced:
//
//   - type ∈ {feat, fix, docs, refactor, test, chore, perf, build, ci,
//     style, revert}.
//   - Single scope only: `[a-z0-9_-]+`. No commas, no spaces.
//   - Subject must be non-empty.
//   - Type must be lowercase.
//   - Colon-space separator is required between header and subject.
func IsConventionalCommitInvalid(msg string) bool {
	first := strings.SplitN(msg, "\n", 2)[0]
	first = strings.TrimRight(first, " \t")
	if first == "" {
		return true
	}
	for _, prefix := range commitMsgBypassPrefixes {
		if strings.HasPrefix(first, prefix) {
			return false
		}
	}
	if strictConventionalCommit.MatchString(first) {
		return false
	}
	return true
}

// extractCommitMessageFromShell pulls the `-m "..."` or `-m '...'`
// payload out of a `git commit` shell invocation, including pipelines
// like `git add -A && git commit -m "..."`. Returns ("", false) when:
//
//   - the command does not contain `git commit`
//   - the command uses `-F`, `-c`, or no `-m` (we only short-circuit
//     when the message is on argv).
//
// The matcher is intentionally permissive on quoting: it accepts a
// double-quoted or single-quoted body, with the closing quote being
// the same kind as the opener.
//
// We deliberately do NOT try to handle escaped quotes inside the
// message — the post-staged commit-msg hook is the authoritative
// linter; this surface is a fast pre-flight that catches the common
// `git commit -m "foo(a,b): bar"` mistake.
var commitMessageInShell = regexp.MustCompile(
	`(?s)git\s+commit\s+(?:[^"'\s][^\s]*\s+)*-m\s+(?:"([^"]*)"|'([^']*)')`,
)

func extractCommitMessageFromShell(cmd string) (string, bool) {
	if cmd == "" {
		return "", false
	}
	if !strings.Contains(cmd, "git commit") {
		return "", false
	}
	m := commitMessageInShell.FindStringSubmatch(cmd)
	if m == nil {
		return "", false
	}
	if m[1] != "" {
		return m[1], true
	}
	if m[2] != "" {
		return m[2], true
	}
	return "", false
}

// commitFormatLintDeny is the IDE-hook surface used by guard-shell.
// Returns a deny response when the supplied shell command embeds a
// `git commit -m "<msg>"` whose subject violates the strict
// conventional-commit rules. nil means "this surface had nothing to
// add; let the rest of the guard-shell pipeline run".
func commitFormatLintDeny(cmd string) *hookio.Response {
	msg, ok := extractCommitMessageFromShell(cmd)
	if !ok {
		return nil
	}
	if !IsConventionalCommitInvalid(msg) {
		return nil
	}
	user := "BLOCKED: commit message fails the strict conventional-commit pre-flight.\n" +
		"  Got: " + truncateCommitMsgForDisplay(msg, 120) + "\n" +
		"  Use a SINGLE scope only (e.g. feat(api): foo). Allowed types: " +
		"feat, fix, docs, refactor, test, chore, perf, build, ci, style, revert."
	agent := "Reject and rewrite the commit message with one scope segment. " +
		"Multi-scope (comma-separated) commits are rejected by the post-staged commit-msg hook anyway; " +
		"this pre-flight saves the staging round trip. " +
		"If this is a real Merge or auto: commit, the linter exempts those prefixes."
	return hookio.Deny(user, agent)
}

func truncateCommitMsgForDisplay(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
