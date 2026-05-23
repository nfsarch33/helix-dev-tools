package cli

import (
	"io"
	"strings"
	"testing"
)

func withFakeSentruxGate(t *testing.T, enabled bool, findings []string) {
	t.Helper()
	prevEnabled := sentruxGateEnabledGetter
	prevEval := sentruxGateEvaluator
	sentruxGateEnabledGetter = func() bool { return enabled }
	sentruxGateEvaluator = func() []string { return findings }
	t.Cleanup(func() {
		sentruxGateEnabledGetter = prevEnabled
		sentruxGateEvaluator = prevEval
	})
}

// TestPrePush_SentruxGate_DisabledByDefault_v8900: when the local repo
// has not opted in via `git config hooks.sentruxGate true`, the gate
// must not run and the push proceeds.
func TestPrePush_SentruxGate_DisabledByDefault_v8900(t *testing.T) {
	withFakeAllowMainPush(t, true)
	withFakeIdentityGate(t, func(_ identityGateState) []string { return nil }, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-tools.git",
		GitEmail:  "jaslian@gmail.com",
	})
	withFakePublicRepoGate(t, func(_ string) []string { return nil })
	withFakeRebrandGate(t, func(_ string) []string { return nil })
	called := false
	prevEnabled := sentruxGateEnabledGetter
	prevEval := sentruxGateEvaluator
	sentruxGateEnabledGetter = func() bool { return false }
	sentruxGateEvaluator = func() []string {
		called = true
		return []string{"would have failed"}
	}
	t.Cleanup(func() {
		sentruxGateEnabledGetter = prevEnabled
		sentruxGateEvaluator = prevEval
	})
	code, _ := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/x abc refs/heads/feature/x def\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	_ = runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/cursor-tools.git"})
	if *code != 0 {
		t.Fatalf("expected exit 0 when sentrux gate disabled, got %d", *code)
	}
	if called {
		t.Fatalf("sentrux evaluator should not run when disabled")
	}
}

// TestPrePush_SentruxGate_EnabledFails_v8900: an enabled gate that
// returns findings blocks the push and prints remediation.
func TestPrePush_SentruxGate_EnabledFails_v8900(t *testing.T) {
	withFakeAllowMainPush(t, true)
	withFakeIdentityGate(t, func(_ identityGateState) []string { return nil }, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-tools.git",
		GitEmail:  "jaslian@gmail.com",
	})
	withFakePublicRepoGate(t, func(_ string) []string { return nil })
	withFakeRebrandGate(t, func(_ string) []string { return nil })
	withFakeSentruxGate(t, true, []string{
		"sentrux structural gate failed:",
		"Quality 7100 -> 6500",
	})
	code, stderr := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/x abc refs/heads/feature/x def\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	_ = runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/cursor-tools.git"})
	if *code != 1 {
		t.Fatalf("expected exit 1 on sentrux gate failure, got %d", *code)
	}
	if !strings.Contains(stderr.String(), "sentrux-gate FAILED") {
		t.Fatalf("expected sentrux failure header in stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "git config hooks.sentruxGate false") {
		t.Fatalf("expected disable instructions in stderr, got: %q", stderr.String())
	}
}

// TestPrePush_SentruxGate_EnabledPasses_v8900: an enabled gate with no
// findings allows the push.
func TestPrePush_SentruxGate_EnabledPasses_v8900(t *testing.T) {
	withFakeAllowMainPush(t, true)
	withFakeIdentityGate(t, func(_ identityGateState) []string { return nil }, identityGateState{
		RemoteURL: "git@github-agtc:nfsarch33/cursor-tools.git",
		GitEmail:  "jaslian@gmail.com",
	})
	withFakePublicRepoGate(t, func(_ string) []string { return nil })
	withFakeRebrandGate(t, func(_ string) []string { return nil })
	withFakeSentruxGate(t, true, nil)
	code, _ := withCapturedPrePushExit(t)

	prePushStdin = strings.NewReader("refs/heads/feature/x abc refs/heads/feature/x def\n")
	defer func() { prePushStdin = io.Reader(nil) }()

	_ = runPrePush(nil, []string{"origin", "git@github-agtc:nfsarch33/cursor-tools.git"})
	if *code != 0 {
		t.Fatalf("expected exit 0 when sentrux gate passes, got %d", *code)
	}
}
