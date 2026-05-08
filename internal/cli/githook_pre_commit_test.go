// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// withFakeGitConfig replaces gitConfigGetter for the duration of the
// test and restores it on cleanup. The fake returns whatever value the
// caller stored under the key, defaulting to "" when missing.
func withFakeGitConfig(t *testing.T, values map[string]string, errFor map[string]error) {
	t.Helper()
	prev := gitConfigGetter
	gitConfigGetter = func(key string) (string, error) {
		if errFor != nil {
			if e, ok := errFor[key]; ok && e != nil {
				return "", e
			}
		}
		if values == nil {
			return "", nil
		}
		return values[key], nil
	}
	t.Cleanup(func() { gitConfigGetter = prev })
}

// withFakeExit replaces preCommitExit + preCommitStderr so tests can
// observe the exit code and stderr message without killing the test
// binary.
func withFakeExit(t *testing.T) (codePtr *int, stderr *bytes.Buffer) {
	t.Helper()
	prevExit := preCommitExit
	prevStderr := preCommitStderr
	code := 0
	codePtr = &code
	stderr = &bytes.Buffer{}
	preCommitExit = func(c int) { code = c }
	preCommitStderr = stderr
	t.Cleanup(func() {
		preCommitExit = prevExit
		preCommitStderr = prevStderr
	})
	return codePtr, stderr
}

func TestPreCommit_BlocksZendeskEmail(t *testing.T) {
	withFakeGitConfig(t, map[string]string{
		"user.email": "jlianzendesk@zendesk.com",
	}, nil)
	code, stderr := withFakeExit(t)

	if err := runPreCommit(nil, nil); err != nil {
		t.Fatalf("runPreCommit returned error: %v", err)
	}
	if *code != 1 {
		t.Fatalf("expected exit 1, got %d", *code)
	}
	msg := stderr.String()
	if !strings.Contains(msg, "Zendesk identity") {
		t.Errorf("stderr %q missing 'Zendesk identity'", msg)
	}
	if !strings.Contains(msg, "jaslian@gmail.com") {
		t.Errorf("stderr %q missing remediation guidance", msg)
	}
}

func TestPreCommit_BlocksAnyZendeskDomain(t *testing.T) {
	withFakeGitConfig(t, map[string]string{
		"user.email": "j.lian@zendesk.com",
	}, nil)
	code, _ := withFakeExit(t)

	_ = runPreCommit(nil, nil)
	if *code != 1 {
		t.Fatalf("expected exit 1 for j.lian@zendesk.com, got %d", *code)
	}
}

func TestPreCommit_AllowsPersonalEmail(t *testing.T) {
	withFakeGitConfig(t, map[string]string{
		"user.email": "jaslian@gmail.com",
	}, nil)
	code, stderr := withFakeExit(t)

	if err := runPreCommit(nil, nil); err != nil {
		t.Fatalf("runPreCommit returned error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("expected exit 0 for personal email, got %d", *code)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty on success, got %q", stderr.String())
	}
}

func TestPreCommit_BlocksEmptyEmail(t *testing.T) {
	withFakeGitConfig(t, map[string]string{
		"user.email": "",
	}, nil)
	code, stderr := withFakeExit(t)

	_ = runPreCommit(nil, nil)
	if *code != 1 {
		t.Fatalf("expected exit 1 for empty email, got %d", *code)
	}
	if !strings.Contains(stderr.String(), "user.email is empty") {
		t.Errorf("stderr %q missing empty-email message", stderr.String())
	}
}

func TestPreCommit_RespectsAllowOptOut(t *testing.T) {
	withFakeGitConfig(t, map[string]string{
		"user.email":                 "jlianzendesk@zendesk.com",
		"hooks.allowZendeskIdentity": "true",
	}, nil)
	code, stderr := withFakeExit(t)

	if err := runPreCommit(nil, nil); err != nil {
		t.Fatalf("runPreCommit returned error: %v", err)
	}
	if *code != 0 {
		t.Fatalf("opt-out should allow commit, got exit %d", *code)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty under opt-out, got %q", stderr.String())
	}
}

func TestPreCommit_OptOutCaseInsensitive(t *testing.T) {
	withFakeGitConfig(t, map[string]string{
		"user.email":                 "jlianzendesk@zendesk.com",
		"hooks.allowZendeskIdentity": "TRUE",
	}, nil)
	code, _ := withFakeExit(t)

	_ = runPreCommit(nil, nil)
	if *code != 0 {
		t.Fatalf("TRUE opt-out should allow commit, got exit %d", *code)
	}
}

func TestPreCommit_GitConfigErrorAborts(t *testing.T) {
	withFakeGitConfig(t,
		map[string]string{},
		map[string]error{"user.email": errors.New("git crashed")},
	)
	code, stderr := withFakeExit(t)

	_ = runPreCommit(nil, nil)
	if *code != 1 {
		t.Fatalf("expected exit 1 on git config error, got %d", *code)
	}
	if !strings.Contains(stderr.String(), "git crashed") {
		t.Errorf("stderr %q should surface underlying git error", stderr.String())
	}
}
