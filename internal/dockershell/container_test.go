package dockershell_test

import (
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/dockershell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContainerConfig_Defaults(t *testing.T) {
	cfg := dockershell.NewContainerConfig("global-kb")
	assert.Equal(t, "global-kb", cfg.RepoAlias)
	assert.Equal(t, "alpine/git:latest", cfg.Image)
	assert.False(t, cfg.NetworkEnabled)
	assert.Empty(t, cfg.EnvVars)
}

func TestContainerConfig_ScrubsTokens(t *testing.T) {
	cfg := dockershell.NewContainerConfig("global-kb")
	cfg.EnvVars = map[string]string{
		"GH_TOKEN":                 "ghp_secret",
		"GITHUB_TOKEN":             "ghp_secret2",
		"GITHUB_API_TOKEN":         "ghp_secret3",
		"GH_ENTERPRISE_TOKEN":      "ghp_secret4",
		"HOMEBREW_GITHUB_API_TOKEN": "ghp_secret5",
		"GIT_AUTHOR_NAME":          "Test User",
		"GIT_AUTHOR_EMAIL":         "test@example.com",
	}

	scrubbed := cfg.ScrubTokenEnv()
	for _, key := range dockershell.PoisonedTokenKeys {
		_, exists := scrubbed[key]
		assert.False(t, exists, "token key %s should be scrubbed", key)
	}
	assert.Equal(t, "Test User", scrubbed["GIT_AUTHOR_NAME"])
	assert.Equal(t, "test@example.com", scrubbed["GIT_AUTHOR_EMAIL"])
}

func TestContainerConfig_BuildMounts(t *testing.T) {
	cfg := dockershell.NewContainerConfig("global-kb")
	cfg.RepoPath = "/home/user/repos/global-kb"
	cfg.SSHKeyPath = "/home/user/.ssh/id_ed25519"
	cfg.SSHConfigPath = "/home/user/.ssh/config"

	mounts := cfg.BuildMounts()
	require.Len(t, mounts, 3)

	assert.Equal(t, "/home/user/repos/global-kb", mounts[0].Source)
	assert.Equal(t, "/work", mounts[0].Target)
	assert.False(t, mounts[0].ReadOnly)

	assert.Equal(t, "/home/user/.ssh/id_ed25519", mounts[1].Source)
	assert.Equal(t, "/root/.ssh/id_ed25519", mounts[1].Target)
	assert.True(t, mounts[1].ReadOnly)

	assert.Equal(t, "/home/user/.ssh/config", mounts[2].Source)
	assert.Equal(t, "/root/.ssh/config", mounts[2].Target)
	assert.True(t, mounts[2].ReadOnly)
}

func TestContainerConfig_BuildRunArgs(t *testing.T) {
	cfg := dockershell.NewContainerConfig("global-kb")
	cfg.RepoPath = "/home/user/repos/global-kb"
	cfg.SSHKeyPath = "/home/user/.ssh/id_ed25519"
	cfg.SSHConfigPath = "/home/user/.ssh/config"
	cfg.Identity = dockershell.Identity{
		Name:  "Test User",
		Email: "test@example.com",
	}

	args := cfg.BuildRunArgs("git", "push", "origin", "main")
	assert.Contains(t, args, "--rm")
	assert.Contains(t, args, "--network=none")
	assert.Contains(t, args, "-w")
	assert.Contains(t, args, "/work")
	assert.Contains(t, args, "alpine/git:latest")
	assert.Contains(t, args, "git")
	assert.Contains(t, args, "push")
}

func TestContainerConfig_IdentityEnv(t *testing.T) {
	cfg := dockershell.NewContainerConfig("global-kb")
	cfg.Identity = dockershell.Identity{
		Name:  "Test User",
		Email: "test@example.com",
	}

	env := cfg.IdentityEnv()
	assert.Equal(t, "Test User", env["GIT_AUTHOR_NAME"])
	assert.Equal(t, "Test User", env["GIT_COMMITTER_NAME"])
	assert.Equal(t, "test@example.com", env["GIT_AUTHOR_EMAIL"])
	assert.Equal(t, "test@example.com", env["GIT_COMMITTER_EMAIL"])
}

func TestContainerConfig_NetworkDisabledByDefault(t *testing.T) {
	cfg := dockershell.NewContainerConfig("global-kb")
	args := cfg.BuildRunArgs("git", "status")
	assert.Contains(t, args, "--network=none")
}

func TestContainerConfig_NetworkEnabledForPush(t *testing.T) {
	cfg := dockershell.NewContainerConfig("global-kb")
	cfg.NetworkEnabled = true
	args := cfg.BuildRunArgs("git", "push")
	found := false
	for _, a := range args {
		if a == "--network=none" {
			found = true
		}
	}
	assert.False(t, found, "network=none should not be present when NetworkEnabled=true")
}

func TestPoisonedTokenKeys_Complete(t *testing.T) {
	expected := []string{
		"GH_TOKEN",
		"GITHUB_TOKEN",
		"GITHUB_API_TOKEN",
		"GH_ENTERPRISE_TOKEN",
		"GITHUB_ENTERPRISE_TOKEN",
		"HOMEBREW_GITHUB_API_TOKEN",
		"VENDIR_GITHUB_API_TOKEN",
		"SAVED_GITHUB_TOKEN",
	}
	assert.Equal(t, expected, dockershell.PoisonedTokenKeys)
}
