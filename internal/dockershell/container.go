package dockershell

// PoisonedTokenKeys lists environment variable names that must be scrubbed
// before any personal-identity git/gh operation. Mirrors the canonical list
// in runx/internal/envscrub and cursor-tools strict identity gate.
var PoisonedTokenKeys = []string{
	"GH_TOKEN",
	"GITHUB_TOKEN",
	"GITHUB_API_TOKEN",
	"GH_ENTERPRISE_TOKEN",
	"GITHUB_ENTERPRISE_TOKEN",
	"HOMEBREW_GITHUB_API_TOKEN",
	"VENDIR_GITHUB_API_TOKEN",
	"_SAVED_GITHUB_TOKEN",
}

// Identity holds git author/committer identity for container operations.
type Identity struct {
	Name  string
	Email string
}

// Mount represents a bind mount for the Docker container.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// ContainerConfig holds the configuration for an ephemeral Docker container
// that provides identity-isolated git/gh operations. The container inherits
// NO host environment variables by default, preventing ZD PAT injection.
type ContainerConfig struct {
	RepoAlias      string
	RepoPath       string
	Image          string
	SSHKeyPath     string
	SSHConfigPath  string
	Identity       Identity
	NetworkEnabled bool
	EnvVars        map[string]string
}

// NewContainerConfig creates a default container configuration for a repo alias.
func NewContainerConfig(repoAlias string) *ContainerConfig {
	return &ContainerConfig{
		RepoAlias:      repoAlias,
		Image:          "alpine/git:latest",
		NetworkEnabled: false,
		EnvVars:        make(map[string]string),
	}
}

// ScrubTokenEnv returns a copy of EnvVars with all poisoned token keys removed.
func (c *ContainerConfig) ScrubTokenEnv() map[string]string {
	result := make(map[string]string, len(c.EnvVars))
	poisoned := make(map[string]bool, len(PoisonedTokenKeys))
	for _, key := range PoisonedTokenKeys {
		poisoned[key] = true
	}
	for k, v := range c.EnvVars {
		if !poisoned[k] {
			result[k] = v
		}
	}
	return result
}

// IdentityEnv returns environment variables for git author/committer identity.
func (c *ContainerConfig) IdentityEnv() map[string]string {
	return map[string]string{
		"GIT_AUTHOR_NAME":     c.Identity.Name,
		"GIT_AUTHOR_EMAIL":    c.Identity.Email,
		"GIT_COMMITTER_NAME":  c.Identity.Name,
		"GIT_COMMITTER_EMAIL": c.Identity.Email,
	}
}

// BuildMounts constructs the bind mount list for the container.
func (c *ContainerConfig) BuildMounts() []Mount {
	mounts := []Mount{
		{Source: c.RepoPath, Target: "/work", ReadOnly: false},
	}
	if c.SSHKeyPath != "" {
		mounts = append(mounts, Mount{Source: c.SSHKeyPath, Target: "/root/.ssh/id_ed25519", ReadOnly: true})
	}
	if c.SSHConfigPath != "" {
		mounts = append(mounts, Mount{Source: c.SSHConfigPath, Target: "/root/.ssh/config", ReadOnly: true})
	}
	return mounts
}

// BuildRunArgs constructs the docker run arguments for an ephemeral container.
// The resulting args array is suitable for exec.Command("docker", args...).
func (c *ContainerConfig) BuildRunArgs(command ...string) []string {
	args := []string{"run", "--rm"}

	if !c.NetworkEnabled {
		args = append(args, "--network=none")
	}

	args = append(args, "-w", "/work")

	for _, m := range c.BuildMounts() {
		mountOpt := m.Source + ":" + m.Target
		if m.ReadOnly {
			mountOpt += ":ro"
		}
		args = append(args, "-v", mountOpt)
	}

	env := c.IdentityEnv()
	for k, v := range env {
		args = append(args, "-e", k+"="+v)
	}

	args = append(args, c.Image)
	args = append(args, command...)

	return args
}
