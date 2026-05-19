package dockeridentity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Profile struct {
	Name     string
	Email    string
	SSHKey   string
	SSHAlias string
}

type PushConfig struct {
	Profile  Profile
	RepoPath string
	Remote   string
	Ref      string
	Upstream bool
}

func DefaultPersonalProfile() Profile {
	home := os.Getenv("HOME")
	email := os.Getenv("GIT_AUTHOR_EMAIL")
	if email == "" {
		email = "user@example.com"
	}
	sshAlias := os.Getenv("GH_SSH_ALIAS")
	if sshAlias == "" {
		sshAlias = "github.com"
	}
	return Profile{
		Name:     "nfsarch33",
		Email:    email,
		SSHKey:   filepath.Join(home, ".ssh", "agtc"),
		SSHAlias: sshAlias,
	}
}

func BuildDockerArgs(cfg PushConfig) []string {
	knownHosts := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")

	script := fmt.Sprintf(
		`mkdir -p /root/.ssh && cp /tmp/id_key_src /root/.ssh/id_key && chmod 600 /root/.ssh/id_key && `+
			`printf "Host %s\n  HostName github.com\n  User git\n  IdentityFile /root/.ssh/id_key\n  StrictHostKeyChecking accept-new\n" > /root/.ssh/config && `+
			`GIT_SSH_COMMAND="ssh -F /root/.ssh/config" git -c core.hooksPath=/dev/null $GIT_VERB $GIT_FLAGS $GIT_REMOTE $GIT_REF`,
		cfg.Profile.SSHAlias)

	flags := ""
	if cfg.Upstream {
		flags = "-u"
	}

	args := []string{
		"run", "--rm", "--network", "host",
		"-v", cfg.RepoPath + ":/repo",
		"-v", cfg.Profile.SSHKey + ":/tmp/id_key_src:ro",
		"-v", knownHosts + ":/root/.ssh/known_hosts:ro",
		"-w", "/repo",
		"-e", "GIT_VERB=push",
		"-e", "GIT_FLAGS=" + flags,
		"-e", "GIT_REMOTE=" + cfg.Remote,
		"-e", "GIT_REF=" + cfg.Ref,
		"-e", "GIT_AUTHOR_NAME=" + cfg.Profile.Name,
		"-e", "GIT_AUTHOR_EMAIL=" + cfg.Profile.Email,
		"-e", "GIT_COMMITTER_NAME=" + cfg.Profile.Name,
		"-e", "GIT_COMMITTER_EMAIL=" + cfg.Profile.Email,
		"--entrypoint", "/bin/sh",
		"alpine/git:latest",
		"-c", script,
	}
	return args
}

func IsHostEnvPoisoned() bool {
	for _, key := range []string{"GITHUB_TOKEN", "GITHUB_API_TOKEN", "HOMEBREW_GITHUB_API_TOKEN"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func PoisonedKeys() []string {
	var found []string
	for _, key := range []string{"GITHUB_TOKEN", "GITHUB_API_TOKEN", "HOMEBREW_GITHUB_API_TOKEN"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			found = append(found, key)
		}
	}
	return found
}
