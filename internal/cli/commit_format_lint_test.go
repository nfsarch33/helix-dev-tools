// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/patterns"
)

// TestIsConventionalCommitInvalid_MultiScope is the RED gate for v318-3:
// a comma-separated multi-scope commit message must be flagged INVALID
// before it ever reaches the post-staged commit-msg hook. This is the
// regression carried forward from the v317 retro: three multi-scope
// rejections happened only at commit-msg time, after the operator had
// already staged and typed the message.
func TestIsConventionalCommitInvalid_MultiScope(t *testing.T) {
	if !IsConventionalCommitInvalid("feat(scope1,scope2): foo") {
		t.Fatalf("expected feat(scope1,scope2): foo to be flagged INVALID")
	}
}

func TestIsConventionalCommitInvalid_TableDriven(t *testing.T) {
	t.Helper()

	cases := []struct {
		name string
		msg  string
		want bool // true = INVALID
	}{
		// Valid single-scope conventional commits.
		{name: "single-scope feat", msg: "feat(api): add endpoint", want: false},
		{name: "single-scope fix", msg: "fix(githook): correct exit code", want: false},
		{name: "single-scope chore", msg: "chore(ci): update workflow", want: false},
		{name: "single-scope docs", msg: "docs(readme): clarify install", want: false},
		{name: "single-scope refactor", msg: "refactor(cli): extract helper", want: false},
		{name: "single-scope test", msg: "test(patterns): cover regex", want: false},
		{name: "single-scope perf", msg: "perf(loader): cache parse", want: false},
		{name: "single-scope build", msg: "build(deps): bump cobra", want: false},
		{name: "single-scope style", msg: "style(format): gofmt", want: false},
		{name: "single-scope revert", msg: "revert(cli): undo bad change", want: false},
		{name: "no-scope feat", msg: "feat: add new capability", want: false},
		{name: "no-scope fix", msg: "fix: hotfix typo", want: false},
		{name: "scope with hyphen", msg: "feat(commit-msg): tighten regex", want: false},
		{name: "scope with underscore", msg: "feat(api_v2): rollout", want: false},
		{name: "scope with digits", msg: "feat(v318): something", want: false},

		// INVALID — the v318-3 surface.
		{name: "multi-scope comma", msg: "feat(scope1,scope2): foo", want: true},
		{name: "multi-scope comma+space", msg: "feat(rules, skills): foo", want: true},
		{name: "multi-scope three", msg: "feat(a,b,c): foo", want: true},
		{name: "multi-scope mvc real-world", msg: "feat(rules,skills): refresh", want: true},
		// INVALID — generic conventional violations.
		{name: "unknown type", msg: "bogus(scope): foo", want: true},
		{name: "missing subject", msg: "feat(api): ", want: true},
		{name: "missing colon", msg: "feat(api) add foo", want: true},
		{name: "uppercase type", msg: "Feat(api): foo", want: true},
		{name: "scope with space", msg: "feat(my scope): foo", want: true},

		// Bypass prefixes — must NOT be flagged INVALID even if they
		// look unconventional.
		{name: "merge bypass", msg: "Merge pull request #42", want: false},
		{name: "auto bypass", msg: "auto: refresh sentrux baseline", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsConventionalCommitInvalid(tc.msg)
			if got != tc.want {
				t.Fatalf("IsConventionalCommitInvalid(%q) = %v, want %v", tc.msg, got, tc.want)
			}
		})
	}
}

// TestExtractCommitMessageFromShell verifies the shell-command parsing
// surface that lets guard-shell intercept `git commit -m "..."` BEFORE
// the post-staged commit-msg hook runs.
func TestExtractCommitMessageFromShell(t *testing.T) {
	t.Helper()

	cases := []struct {
		name    string
		cmd     string
		wantMsg string
		wantOk  bool
	}{
		{
			name:    "double-quoted single scope",
			cmd:     `git commit -m "feat(api): add endpoint"`,
			wantMsg: "feat(api): add endpoint",
			wantOk:  true,
		},
		{
			name:    "single-quoted multi scope",
			cmd:     `git commit -m 'feat(rules,skills): refresh'`,
			wantMsg: "feat(rules,skills): refresh",
			wantOk:  true,
		},
		{
			name:    "compound: && git commit",
			cmd:     `git add -A && git commit -m "fix(cli): wire flag"`,
			wantMsg: "fix(cli): wire flag",
			wantOk:  true,
		},
		{
			name:    "no commit",
			cmd:     "git status",
			wantMsg: "",
			wantOk:  false,
		},
		{
			name:    "commit without -m",
			cmd:     "git commit",
			wantMsg: "",
			wantOk:  false,
		},
		{
			name:    "commit with -F file",
			cmd:     "git commit -F .git/COMMIT_EDITMSG",
			wantMsg: "",
			wantOk:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMsg, gotOk := extractCommitMessageFromShell(tc.cmd)
			if gotMsg != tc.wantMsg || gotOk != tc.wantOk {
				t.Fatalf(
					"extractCommitMessageFromShell(%q) = (%q, %v), want (%q, %v)",
					tc.cmd, gotMsg, gotOk, tc.wantMsg, tc.wantOk,
				)
			}
		})
	}
}

// TestGuardShell_DeniesMultiScopeCommit is the integration end of the
// v318-3 surface. With a clean identity (so the existing identity gate
// does not deny first), a `git commit -m "feat(a,b): foo"` invocation
// MUST be denied with a "use single scope" remediation hint.
func TestGuardShell_DeniesMultiScopeCommit(t *testing.T) {
	tmp := t.TempDir()
	m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
	if err != nil {
		t.Fatalf("compile matcher: %v", err)
	}
	h := &guardShellHandler{
		matcher:     m,
		log:         logger.New(filepath.Join(tmp, "ide-hook.log")),
		metricsPath: filepath.Join(tmp, "metrics.jsonl"),
	}

	prev := identityGateStateProviderForTest
	identityGateStateProviderForTest = func() identityGateState {
		return identityGateState{
			RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
			GitEmail:  "jaslian@gmail.com",
			Env:       map[string]string{},
		}
	}
	defer func() { identityGateStateProviderForTest = prev }()

	resp, err := h.Handle(context.Background(), &hookio.Input{
		Command: `git commit -m "feat(rules,skills): refresh hot context"`,
	})
	if err != nil {
		t.Fatalf("Handle returned err: %v", err)
	}
	if resp.Permission != "deny" {
		t.Fatalf("expected deny for multi-scope commit, got %s (user=%q agent=%q)",
			resp.Permission, resp.UserMessage, resp.AgentMessage)
	}
	combined := resp.UserMessage + " " + resp.AgentMessage
	if want := "single scope"; !strings.Contains(strings.ToLower(combined), want) {
		t.Fatalf("expected remediation hint to mention %q, got user=%q agent=%q",
			want, resp.UserMessage, resp.AgentMessage)
	}
}

// TestGuardShell_AllowsSingleScopeCommit confirms the linter does not
// over-deny: a clean single-scope `git commit -m "..."` MUST allow.
func TestGuardShell_AllowsSingleScopeCommit(t *testing.T) {
	tmp := t.TempDir()
	m, err := patterns.NewMatcher(patterns.ShellDenyPatterns, patterns.ShellWarnPatterns)
	if err != nil {
		t.Fatalf("compile matcher: %v", err)
	}
	h := &guardShellHandler{
		matcher:     m,
		log:         logger.New(filepath.Join(tmp, "ide-hook.log")),
		metricsPath: filepath.Join(tmp, "metrics.jsonl"),
	}

	prev := identityGateStateProviderForTest
	identityGateStateProviderForTest = func() identityGateState {
		return identityGateState{
			RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
			GitEmail:  "jaslian@gmail.com",
			Env:       map[string]string{},
		}
	}
	defer func() { identityGateStateProviderForTest = prev }()

	resp, err := h.Handle(context.Background(), &hookio.Input{
		Command: `git commit -m "feat(githook): add v318-3 linter"`,
	})
	if err != nil {
		t.Fatalf("Handle returned err: %v", err)
	}
	if resp.Permission != "allow" {
		t.Fatalf("expected allow for single-scope commit, got %s (user=%q agent=%q)",
			resp.Permission, resp.UserMessage, resp.AgentMessage)
	}
}
