package dockershell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	GHImage         = "ghcr.io/cli/cli:latest"
	ProfilePersonal = "personal"
	ProfileZendesk  = "zendesk"
)

// GHProfile defines a GitHub CLI identity profile for Docker-isolated operations.
type GHProfile struct {
	Name       string
	Login      string
	Email      string
	TokenEnvID string
	SSHAlias   string
}

// DefaultProfiles returns the canonical personal/work profile set.
func DefaultProfiles() map[string]GHProfile {
	return map[string]GHProfile{
		ProfilePersonal: {
			Name:       "personal",
			Login:      "nfsarch33",
			Email:      "jaslian@gmail.com",
			TokenEnvID: "GH_PERSONAL_TOKEN",
			SSHAlias:   "agtc",
		},
		ProfileZendesk: {
			Name:       "zendesk",
			Login:      "jlianzendesk",
			Email:      "jason.lian@zendesk.com",
			TokenEnvID: "GH_ZENDESK_TOKEN",
			SSHAlias:   "",
		},
	}
}

// GHIsolationConfig holds settings for a Docker-isolated gh CLI invocation.
type GHIsolationConfig struct {
	Profile   GHProfile
	RepoPath  string
	Token     string
	NetworkOn bool
	ExtraEnv  map[string]string
}

// ValidateHostEnv checks if the current host environment has poisoned tokens
// that could leak into a child gh process. Returns the list of poisoned keys found.
func ValidateHostEnv() []string {
	var found []string
	for _, key := range PoisonedTokenKeys {
		if os.Getenv(key) != "" {
			found = append(found, key)
		}
	}
	return found
}

// IsHostEnvPoisoned returns true if any gh-related token is set in the host env.
func IsHostEnvPoisoned() bool {
	return len(ValidateHostEnv()) > 0
}

// BuildGHRunArgs constructs docker run arguments for an isolated gh CLI invocation.
// The container gets ONLY the explicitly provided token (never inherited from host).
func (c *GHIsolationConfig) BuildGHRunArgs(ghArgs ...string) []string {
	args := []string{"run", "--rm"}

	if c.NetworkOn {
		args = append(args, "--network=host")
	}

	args = append(args, "-w", "/work")

	if c.RepoPath != "" {
		args = append(args, "-v", c.RepoPath+":/work")
	}

	args = append(args, "-e", "GH_TOKEN="+c.Token)
	args = append(args, "-e", "GIT_AUTHOR_NAME="+c.Profile.Login)
	args = append(args, "-e", "GIT_AUTHOR_EMAIL="+c.Profile.Email)
	args = append(args, "-e", "GIT_COMMITTER_NAME="+c.Profile.Login)
	args = append(args, "-e", "GIT_COMMITTER_EMAIL="+c.Profile.Email)

	for k, v := range c.ExtraEnv {
		args = append(args, "-e", k+"="+v)
	}

	args = append(args, GHImage)
	args = append(args, ghArgs...)

	return args
}

// RunGHIsolated executes a gh command inside a Docker container with identity isolation.
// Returns combined stdout+stderr output and any execution error.
func RunGHIsolated(cfg *GHIsolationConfig, ghArgs ...string) (string, error) {
	if cfg.Token == "" {
		return "", fmt.Errorf("gh isolation: token must not be empty for profile %q", cfg.Profile.Name)
	}

	dockerArgs := cfg.BuildGHRunArgs(ghArgs...)
	cmd := exec.Command("docker", dockerArgs...)
	cmd.Env = []string{}

	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// ResolveTokenFromKeychain attempts to read the gh token for the given profile
// from the macOS Keychain. Returns empty string if not found.
func ResolveTokenFromKeychain(profile GHProfile) string {
	serviceName := fmt.Sprintf("gh-identity-%s", profile.Name)
	cmd := exec.Command("/usr/bin/security", "find-generic-password", "-s", serviceName, "-w")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// PersonalGHConfig creates a GHIsolationConfig for personal repos.
// Token is resolved from Keychain or the GH_PERSONAL_TOKEN env var.
func PersonalGHConfig(repoPath string) (*GHIsolationConfig, error) {
	profiles := DefaultProfiles()
	personal := profiles[ProfilePersonal]

	token := ResolveTokenFromKeychain(personal)
	if token == "" {
		token = os.Getenv(personal.TokenEnvID)
	}
	if token == "" {
		return nil, fmt.Errorf("no token available for personal profile (checked keychain service gh-identity-personal and env %s)", personal.TokenEnvID)
	}

	absPath, _ := filepath.Abs(repoPath)
	return &GHIsolationConfig{
		Profile:   personal,
		RepoPath:  absPath,
		Token:     token,
		NetworkOn: true,
	}, nil
}
