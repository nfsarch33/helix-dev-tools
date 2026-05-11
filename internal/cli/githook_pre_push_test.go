// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func withFakeIdentityGate(t *testing.T, fn func(identityGateState) []string, state identityGateState) {
	t.Helper()
	prevFn := identityGateEvaluator
	prevState := identityGateGatherer
	identityGateEvaluator = fn
	identityGateGatherer = func() identityGateState { return state }
	t.Cleanup(func() {
		identityGateEvaluator = prevFn
		identityGateGatherer = prevState
	})
}

func withFakeAllowMainPush(t *testing.T, allow bool) {
	t.Helper()
	prev := allowMainPushGetter
	allowMainPushGetter = func() bool { return allow }
	t.Cleanup(func() { allowMainPushGetter = prev })
}

func withCapturedPrePushExit(t *testing.T) (*int, *bytes.Buffer) {
	t.Helper()
	prevExit := prePushExit
	prevStderr := prePushStderr
	code := 0
	stderr := &bytes.Buffer{}
	prePushExit = func(c int) { code = c }
	prePushStderr = stderr
	t.Cleanup(func() {
		prePushExit = prevExit
		prePushStderr = prevStderr
	})
	return &code, stderr
}

// withFakePublicRepoGate replaces the public-repo-gate evaluator for
// the duration of a test. Returns nil (no findings) by default.
func withFakePublicRepoGate(t *testing.T, fn func(remoteURL string) []string) {
	t.Helper()
	prev := publicRepoGateEvaluator
	publicRepoGateEvaluator = fn
	t.Cleanup(func() { publicRepoGateEvaluator = prev })
}

func withFakeAncestorChecker(t *testing.T, fn func(remoteSHA, localSHA string) bool) {
	t.Helper()
	prev := isAncestorChecker
	isAncestorChecker = fn
	t.Cleanup(func() { isAncestorChecker = prev })
}

func TestPrePush_BlocksPoisonedGitHubTokenOnPersonalRemote(t *testing.T) {
	withFakeAllowMainPush(t, false)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{"GITHUB_TOKEN": "ghp_work"},
	})
	code, stderr := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/foo local-sha refs/heads/feature/foo remote-sha\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	_ = runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/cursor-global-kb.git"})
	if *code != 1 {
		t.Fatalf("want exit 1 for poisoned token, got %d", *code)
	}
	if !strings.Contains(stderr.String(), "GITHUB_TOKEN") {
		t.Errorf("stderr should mention GITHUB_TOKEN: %q", stderr.String())
	}
}

func TestPrePush_AllowsCleanIdentityFeatureBranch(t *testing.T) {
	withFakeAllowMainPush(t, false)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	code, _ := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/foo local-sha refs/heads/feature/foo remote-sha\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	if err := runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/cursor-global-kb.git"}); err != nil {
		t.Fatalf("clean identity should not return error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("want exit 0 for clean identity, got %d", *code)
	}
}

func TestPrePush_DoesNotGateZendeskWorkClone(t *testing.T) {
	withFakeAllowMainPush(t, false)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github.com:zendesk/secure-auth-platform.git",
		GitEmail:  "jlianzendesk@zendesk.com",
		Env:       map[string]string{"GITHUB_TOKEN": "ghp_work"},
	})
	withFakePublicRepoGate(t, func(string) []string { return nil })
	code, _ := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/foo local-sha refs/heads/feature/foo remote-sha\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	if err := runPrePush(nil, []string{"origin", "git@github.com:zendesk/secure-auth-platform.git"}); err != nil {
		t.Fatalf("work clone push must not error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("work clone must not be blocked, got exit %d", *code)
	}
}

func TestPrePush_BlocksZendeskNonFastForwardWithFullSHAs(t *testing.T) {
	withFakeAllowMainPush(t, true)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github.com:zendesk/secure-auth-platform.git",
		GitEmail:  "jlianzendesk@zendesk.com",
		Env:       map[string]string{},
	})
	withFakeAncestorChecker(t, func(_, _ string) bool { return false })
	withFakePublicRepoGate(t, func(string) []string { return nil })
	code, stderr := withCapturedPrePushExit(t)

	line := "refs/heads/feature/foo " +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa refs/heads/feature/foo " +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"
	prePushStdin = strings.NewReader(line)
	defer func() { prePushStdin = io.Reader(nil) }()

	_ = runPrePush(nil, []string{"origin", "git@github.com:zendesk/secure-auth-platform.git"})
	if *code != 1 {
		t.Fatalf("want exit 1 for zendesk non-fast-forward, got %d", *code)
	}
	if !strings.Contains(stderr.String(), "non-fast-forward refused") {
		t.Errorf("stderr should mention refusal: %q", stderr.String())
	}
}

func TestPrePush_AllowsZendeskFastForwardWithFullSHAs(t *testing.T) {
	withFakeAllowMainPush(t, true)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github.com:zendesk/secure-auth-platform.git",
		GitEmail:  "jlianzendesk@zendesk.com",
		Env:       map[string]string{},
	})
	withFakeAncestorChecker(t, func(_, _ string) bool { return true })
	withFakePublicRepoGate(t, func(string) []string { return nil })
	code, _ := withCapturedPrePushExit(t)

	line := "refs/heads/feature/foo " +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa refs/heads/feature/foo " +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"
	prePushStdin = strings.NewReader(line)
	defer func() { prePushStdin = io.Reader(nil) }()

	if err := runPrePush(nil, []string{"origin", "git@github.com:zendesk/secure-auth-platform.git"}); err != nil {
		t.Fatalf("zendesk fast-forward push must not error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("want exit 0, got %d", *code)
	}
}

func TestPrePush_AllowsZendeskNewBranchAllZeroRemoteSHA(t *testing.T) {
	withFakeAllowMainPush(t, true)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github.com:zendesk/secure-auth-platform.git",
		GitEmail:  "jlianzendesk@zendesk.com",
		Env:       map[string]string{},
	})
	withFakeAncestorChecker(t, func(_, _ string) bool {
		t.Fatal("merge-base probe should not run for all-zero remote sha")
		return false
	})
	withFakePublicRepoGate(t, func(string) []string { return nil })
	code, _ := withCapturedPrePushExit(t)

	line := "refs/heads/feature/foo " +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa refs/heads/feature/foo " +
		"0000000000000000000000000000000000000000\n"
	prePushStdin = strings.NewReader(line)
	defer func() { prePushStdin = io.Reader(nil) }()

	if err := runPrePush(nil, []string{"origin", "git@github.com:zendesk/secure-auth-platform.git"}); err != nil {
		t.Fatalf("zendesk new-branch push must not error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("want exit 0, got %d", *code)
	}
}

// v317-1 RED test: a planted leak finding from runx public-repo-gate
// must abort the push with exit 1 and emit the runx remediation hint.
func TestPrePush_BlocksPublicRepoLeakFinding(t *testing.T) {
	withFakeAllowMainPush(t, false)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/ironclaw-mcp.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	withFakePublicRepoGate(t, func(remoteURL string) []string {
		return []string{
			"public-repo-gate failed for nfsarch33/ironclaw-mcp (alias=ironclaw-mcp):",
			"internal/foo.go:42 fleet_alias_2 win1",
		}
	})
	code, stderr := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/foo local-sha refs/heads/feature/foo remote-sha\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	_ = runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/ironclaw-mcp.git"})
	if *code != 1 {
		t.Fatalf("public-repo-gate finding must block push, got exit %d", *code)
	}
	if !strings.Contains(stderr.String(), "public-repo-gate") {
		t.Errorf("stderr should mention public-repo-gate: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "win1") {
		t.Errorf("stderr should surface the finding body: %q", stderr.String())
	}
}

// v317-1 GREEN test: a clean public-repo with no findings must allow
// the push to continue through to the main-branch check.
func TestPrePush_AllowsCleanPublicRepo(t *testing.T) {
	withFakeAllowMainPush(t, false)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/ironclaw-mcp.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	gateInvoked := false
	withFakePublicRepoGate(t, func(remoteURL string) []string {
		gateInvoked = true
		return nil
	})
	code, _ := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/foo local-sha refs/heads/feature/foo remote-sha\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	if err := runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/ironclaw-mcp.git"}); err != nil {
		t.Fatalf("clean public repo push should not error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("clean public repo push should not be blocked, got exit %d", *code)
	}
	if !gateInvoked {
		t.Errorf("public-repo-gate must be invoked for public remote; was skipped")
	}
}

// v317-1 SKIP test: private personal repos must not invoke the gate.
// The default evaluator returns nil for any URL whose repo segment is
// not in publicRepoGitHubNames; this test guarantees runPrePush only
// passes the URL through and does not invent findings.
func TestPrePush_SkipsGateForPrivatePersonalRepo(t *testing.T) {
	withFakeAllowMainPush(t, false)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	gateInvoked := false
	withFakePublicRepoGate(t, func(remoteURL string) []string {
		gateInvoked = true
		// real default returns nil for non-public; mirror that.
		if _, ok := publicRepoGitHubNames[publicRepoNameFromURLDefault(remoteURL)]; !ok {
			return nil
		}
		return []string{"unexpected"}
	})
	code, _ := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/foo local-sha refs/heads/feature/foo remote-sha\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	if err := runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/cursor-global-kb.git"}); err != nil {
		t.Fatalf("private repo push should not error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("private repo push should not be blocked, got exit %d", *code)
	}
	if !gateInvoked {
		t.Errorf("evaluator must still be called so it can decide; was skipped entirely")
	}
}

// v317-1: URL parser unit tests for publicRepoNameFromURLDefault.
func TestPublicRepoNameFromURLDefault_TableDriven(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{name: "ssh agtc personal", url: "git@github-agtc:nfsarch33/ironclaw-mcp.git", want: "ironclaw-mcp"},
		{name: "ssh agtc no .git", url: "git@github-agtc:nfsarch33/ironclaw-mcp", want: "ironclaw-mcp"},
		{name: "https personal", url: "https://github.com/nfsarch33/cursor-tools.git", want: "cursor-tools"},
		{name: "zendesk work clone", url: "git@github.com:zendesk/secure-auth-platform.git", want: ""},
		{name: "empty", url: "", want: ""},
		{name: "personal but not github", url: "git@gitlab.com:nfsarch33/foo.git", want: "foo"}, // matches owner segment regardless of host
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := publicRepoNameFromURLDefault(tc.url)
			if got != tc.want {
				t.Errorf("publicRepoNameFromURLDefault(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestResolvePublicRepoAlias(t *testing.T) {
	t.Parallel()
	if got := resolvePublicRepoAlias("llm-cluster-router"); got != "router" {
		t.Errorf("resolvePublicRepoAlias(llm-cluster-router) = %q, want router", got)
	}
	if got := resolvePublicRepoAlias("ironclaw-mcp"); got != "ironclaw-mcp" {
		t.Errorf("resolvePublicRepoAlias(ironclaw-mcp) = %q, want identity", got)
	}
}

func TestPrePush_StillBlocksPushToMainOnPersonalRemote(t *testing.T) {
	withFakeAllowMainPush(t, false)
	withFakeIdentityGate(t, evaluateIdentityGateStrict, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-global-kb.git",
		GitEmail:  "jaslian@gmail.com",
		Env:       map[string]string{},
	})
	code, stderr := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/main local-sha refs/heads/main remote-sha\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	_ = runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/cursor-global-kb.git"})
	if *code != 1 {
		t.Fatalf("push to main must be blocked even with clean identity: %d", *code)
	}
	if !strings.Contains(stderr.String(), "main") {
		t.Errorf("stderr should mention main: %q", stderr.String())
	}
}
