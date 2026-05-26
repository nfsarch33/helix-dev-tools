package cli

import (
	"os"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/dashboard"
)

func TestParseDashboardProjectIDs(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"", nil},
		{"1", []int{1}},
		{"1,2,3", []int{1, 2, 3}},
		{" 10 , 20 ", []int{10, 20}},
		{"abc", nil},
		{"1,abc,3", []int{1, 3}},
	}
	for _, tt := range tests {
		got := parseDashboardProjectIDs(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseDashboardProjectIDs(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseDashboardProjectIDs(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestEnvFallback(t *testing.T) {
	old := dashboardEnvFn
	defer func() { dashboardEnvFn = old }()

	dashboardEnvFn = func(key string) string {
		if key == "SET_VAR" {
			return "value"
		}
		return ""
	}

	if got := envFallback("SET_VAR", "default"); got != "value" {
		t.Errorf("envFallback(SET_VAR) = %q, want value", got)
	}
	if got := envFallback("UNSET_VAR", "default"); got != "default" {
		t.Errorf("envFallback(UNSET_VAR) = %q, want default", got)
	}
}

func TestBuildDashboardFetchersDefault(t *testing.T) {
	old := dashboardEnvFn
	defer func() { dashboardEnvFn = old }()
	dashboardEnvFn = func(string) string { return "" }

	savedURL := dashboardEngramURL
	dashboardEngramURL = "http://test-engram:8281"
	defer func() { dashboardEngramURL = savedURL }()

	fetchers := buildDashboardFetchers()

	if len(fetchers) < 4 {
		t.Fatalf("expected at least 4 fetchers, got %d", len(fetchers))
	}

	names := make(map[string]bool)
	for _, f := range fetchers {
		names[f.Name()] = true
	}
	for _, want := range []string{"argocd", "sprintboard", "engram", "agentrace"} {
		if !names[want] {
			t.Errorf("missing fetcher %q in default set", want)
		}
	}
}

func TestBuildDashboardFetchersWithGitLab(t *testing.T) {
	old := dashboardEnvFn
	defer func() { dashboardEnvFn = old }()
	dashboardEnvFn = func(key string) string {
		switch key {
		case "GITLAB_PROJECTS":
			return "1,2"
		default:
			return ""
		}
	}

	fetchers := buildDashboardFetchers()

	hasGitLab := false
	for _, f := range fetchers {
		if f.Name() == "gitlab" {
			hasGitLab = true
			break
		}
	}
	if !hasGitLab {
		t.Error("expected gitlab fetcher when GITLAB_PROJECTS set")
	}
}

func TestDashboardServeCommandRegistered(t *testing.T) {
	if dashboardServeCmd.Use != "dashboard-serve" {
		t.Fatalf("Use = %q, want dashboard-serve", dashboardServeCmd.Use)
	}

	portFlag := dashboardServeCmd.Flags().Lookup("port")
	if portFlag == nil {
		t.Fatal("--port flag not registered")
	}
	if portFlag.DefValue != "9095" {
		t.Errorf("--port default = %q, want 9095", portFlag.DefValue)
	}
}

func TestDashboardServeFlagDefaults(t *testing.T) {
	manifest := dashboardServeCmd.Flags().Lookup("manifest")
	if manifest == nil {
		t.Fatal("--manifest flag not registered")
	}
	engram := dashboardServeCmd.Flags().Lookup("engram-url")
	if engram == nil {
		t.Fatal("--engram-url flag not registered")
	}
	if engram.DefValue != "http://localhost:8281" {
		t.Errorf("--engram-url default = %q, want http://localhost:8281", engram.DefValue)
	}
}

func TestEngramFetcherMatchesFlag(t *testing.T) {
	old := dashboardEnvFn
	defer func() { dashboardEnvFn = old }()
	dashboardEnvFn = func(string) string { return "" }

	savedURL := dashboardEngramURL
	dashboardEngramURL = "http://custom-engram:9999"
	defer func() { dashboardEngramURL = savedURL }()

	fetchers := buildDashboardFetchers()
	for _, f := range fetchers {
		if ef, ok := f.(*dashboard.EngramFetcher); ok {
			if ef.BaseURL != "http://custom-engram:9999" {
				t.Errorf("EngramFetcher.BaseURL = %q, want custom URL", ef.BaseURL)
			}
			return
		}
	}
	t.Fatal("no EngramFetcher found")
}

func TestBuildDashboardFetchersNoGitLabWithoutEnv(t *testing.T) {
	old := dashboardEnvFn
	defer func() { dashboardEnvFn = old }()
	dashboardEnvFn = func(string) string { return "" }

	fetchers := buildDashboardFetchers()
	for _, f := range fetchers {
		if f.Name() == "gitlab" {
			t.Error("gitlab fetcher should not be present without GITLAB_PROJECTS")
		}
	}
}

func TestDashboardServeEnvFnDefault(t *testing.T) {
	if dashboardEnvFn == nil {
		t.Fatal("dashboardEnvFn should default to os.Getenv")
	}
	_ = os.Getenv("PATH")
}
