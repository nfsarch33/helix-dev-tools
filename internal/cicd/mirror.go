package cicd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Mirror pushes a local repository to a GitLab remote.
type Mirror struct {
	GitBin string
}

// NewMirror creates a git mirror helper.
func NewMirror() *Mirror {
	return &Mirror{GitBin: "git"}
}

// MirrorResult captures the outcome of a mirror operation.
type MirrorResult struct {
	RepoPath  string
	RemoteURL string
	Ref       string
	Success   bool
	Output    string
	Error     string
}

// Push mirrors a local ref to the remote URL.
func (m *Mirror) Push(ctx context.Context, repoPath, remoteURL, ref string) MirrorResult {
	if ref == "" {
		ref = "main"
	}

	cmd := exec.CommandContext(ctx, m.GitBin, "-C", repoPath, "push", remoteURL, ref)
	out, err := cmd.CombinedOutput()

	result := MirrorResult{
		RepoPath:  repoPath,
		RemoteURL: remoteURL,
		Ref:       ref,
		Output:    strings.TrimSpace(string(out)),
	}

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("%v: %s", err, out)
	} else {
		result.Success = true
	}
	return result
}

// AddRemote adds a GitLab remote to the local repo if not present.
func (m *Mirror) AddRemote(ctx context.Context, repoPath, remoteName, remoteURL string) error {
	checkCmd := exec.CommandContext(ctx, m.GitBin, "-C", repoPath, "remote", "get-url", remoteName)
	if err := checkCmd.Run(); err == nil {
		return nil
	}
	addCmd := exec.CommandContext(ctx, m.GitBin, "-C", repoPath, "remote", "add", remoteName, remoteURL)
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add remote: %v: %s", err, out)
	}
	return nil
}
