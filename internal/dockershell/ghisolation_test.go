package dockershell

import (
	"testing"
)

func TestDefaultProfiles(t *testing.T) {
	t.Setenv("GH_PERSONAL_LOGIN", "test-personal")
	t.Setenv("GH_PERSONAL_EMAIL", "test@example.com")
	t.Setenv("GH_ZENDESK_LOGIN", "test-work")
	t.Setenv("GH_ZENDESK_EMAIL", "work@example.com")

	profiles := DefaultProfiles()

	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}

	personal := profiles[ProfilePersonal]
	if personal.Login != "test-personal" {
		t.Errorf("personal login = %q, want test-personal", personal.Login)
	}
	if personal.Email != "test@example.com" {
		t.Errorf("personal email = %q, want test@example.com", personal.Email)
	}

	zd := profiles[ProfileZendesk]
	if zd.Login != "test-work" {
		t.Errorf("zendesk login = %q, want test-work", zd.Login)
	}
}

func TestValidateHostEnv_Clean(t *testing.T) {
	for _, key := range PoisonedTokenKeys {
		t.Setenv(key, "")
	}
	found := ValidateHostEnv()
	if len(found) != 0 {
		t.Errorf("expected no poisoned keys, found %v", found)
	}
}

func TestValidateHostEnv_Poisoned(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test_value")
	t.Setenv("GH_TOKEN", "gho_test_value")

	found := ValidateHostEnv()
	if len(found) < 2 {
		t.Errorf("expected at least 2 poisoned keys, found %d: %v", len(found), found)
	}
}

func TestBuildGHRunArgs_PersonalProfile(t *testing.T) {
	cfg := &GHIsolationConfig{
		Profile: GHProfile{
			Name:  "personal",
			Login: "test-user",
			Email: "test@example.com",
		},
		RepoPath:  "/tmp/test-repo",
		Token:     "ghp_test_token_12345",
		NetworkOn: true,
	}

	args := cfg.BuildGHRunArgs("pr", "list")

	hasNetwork := false
	hasToken := false
	hasWorkdir := false
	hasImage := false
	hasGHCmd := false

	for i, arg := range args {
		switch {
		case arg == "--network=host":
			hasNetwork = true
		case arg == "GH_TOKEN=ghp_test_token_12345":
			hasToken = true
		case arg == "-w" && i+1 < len(args) && args[i+1] == "/work":
			hasWorkdir = true
		case arg == GHImage:
			hasImage = true
		case arg == "pr" && i+1 < len(args) && args[i+1] == "list":
			hasGHCmd = true
		}
	}

	if !hasNetwork {
		t.Error("missing --network=host")
	}
	if !hasToken {
		t.Error("missing GH_TOKEN env injection")
	}
	if !hasWorkdir {
		t.Error("missing -w /work")
	}
	if !hasImage {
		t.Errorf("missing image %s", GHImage)
	}
	if !hasGHCmd {
		t.Error("missing gh pr list command")
	}
}

func TestBuildGHRunArgs_NoHostEnvInheritance(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "poisoned_value")

	cfg := &GHIsolationConfig{
		Profile: GHProfile{
			Name:  "personal",
			Login: "test-user",
			Email: "test@example.com",
		},
		RepoPath:  "/tmp/test-repo",
		Token:     "ghp_clean_personal_token",
		NetworkOn: true,
	}

	args := cfg.BuildGHRunArgs("api", "user")

	for _, arg := range args {
		if arg == "GITHUB_TOKEN=poisoned_value" || arg == "GH_TOKEN=poisoned_value" {
			t.Errorf("container args contain poisoned host token: %s", arg)
		}
	}
}

func TestRunGHIsolated_EmptyToken(t *testing.T) {
	cfg := &GHIsolationConfig{
		Profile: GHProfile{Name: "test"},
		Token:   "",
	}

	_, err := RunGHIsolated(cfg, "api", "user")
	if err == nil {
		t.Error("expected error for empty token")
	}
}
