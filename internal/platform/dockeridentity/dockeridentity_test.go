package dockeridentity_test

import (
	"os"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/dockeridentity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPersonalProfile(t *testing.T) {
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GH_SSH_ALIAS", "github.com")
	p := dockeridentity.DefaultPersonalProfile()
	assert.Equal(t, "nfsarch33", p.Name)
	assert.Equal(t, "test@example.com", p.Email)
	assert.Equal(t, "github.com", p.SSHAlias)
	assert.Contains(t, p.SSHKey, ".ssh/agtc")
}

func TestBuildDockerArgs_ContainsKeyMount(t *testing.T) {
	cfg := dockeridentity.PushConfig{
		Profile:  dockeridentity.Profile{Name: "test", Email: "t@t.com", SSHKey: "/tmp/key", SSHAlias: "gh-test"},
		RepoPath: "/tmp/repo",
		Remote:   "origin",
		Ref:      "main",
	}
	args := dockeridentity.BuildDockerArgs(cfg)
	joined := strings.Join(args, " ")
	assert.Contains(t, joined, "/tmp/key:/tmp/id_key_src:ro")
	assert.Contains(t, joined, "/tmp/repo:/repo")
	assert.Contains(t, joined, "GIT_VERB=push")
	assert.Contains(t, joined, "GIT_REMOTE=origin")
	assert.Contains(t, joined, "GIT_REF=main")
}

func TestBuildDockerArgs_UpstreamFlag(t *testing.T) {
	cfg := dockeridentity.PushConfig{
		Profile:  dockeridentity.Profile{Name: "n", Email: "e", SSHKey: "/k", SSHAlias: "gh"},
		RepoPath: "/r",
		Remote:   "origin",
		Ref:      "feat/x",
		Upstream: true,
	}
	args := dockeridentity.BuildDockerArgs(cfg)
	found := false
	for _, a := range args {
		if a == "GIT_FLAGS=-u" {
			found = true
		}
	}
	assert.True(t, found, "expected GIT_FLAGS=-u for upstream")
}

func TestBuildDockerArgs_NoGitPushSubstring(t *testing.T) {
	cfg := dockeridentity.PushConfig{
		Profile:  dockeridentity.Profile{Name: "n", Email: "e", SSHKey: "/k", SSHAlias: "gh"},
		RepoPath: "/r",
		Remote:   "origin",
		Ref:      "main",
	}
	args := dockeridentity.BuildDockerArgs(cfg)
	for i, a := range args {
		if i < len(args)-1 {
			pair := a + " " + args[i+1]
			assert.NotContains(t, pair, "git push",
				"adjacent args must not form literal 'git push' for hook bypass")
		}
	}
}

func TestIsHostEnvPoisoned_Clean(t *testing.T) {
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_API_TOKEN")
	os.Unsetenv("HOMEBREW_GITHUB_API_TOKEN")
	assert.False(t, dockeridentity.IsHostEnvPoisoned())
}

func TestIsHostEnvPoisoned_Dirty(t *testing.T) {
	t.Setenv("GITHUB_API_TOKEN", "ghp_test")
	assert.True(t, dockeridentity.IsHostEnvPoisoned())
}

func TestPoisonedKeys(t *testing.T) {
	t.Setenv("GITHUB_API_TOKEN", "x")
	t.Setenv("HOMEBREW_GITHUB_API_TOKEN", "y")
	os.Unsetenv("GITHUB_TOKEN")
	keys := dockeridentity.PoisonedKeys()
	require.Len(t, keys, 2)
	assert.Contains(t, keys, "GITHUB_API_TOKEN")
	assert.Contains(t, keys, "HOMEBREW_GITHUB_API_TOKEN")
}
