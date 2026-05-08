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
