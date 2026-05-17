package mcpfilter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/mcpfilter"
)

func sampleConfig() *mcpfilter.MCPConfig {
	return &mcpfilter.MCPConfig{
		MCPServers: map[string]mcpfilter.MCPServer{
			"user-mem0":                 {Command: "mem0-mcp-go"},
			"user-context-mode":         {Command: "context-mode"},
			"user-time":                 {Command: "time-server"},
			"user-exa":                  {Command: "exa"},
			"user-tavily-mcp":           {Command: "tavily"},
			"user-perplexity-ask":       {Command: "perplexity"},
			"user-duckduckgo":           {Command: "ddg"},
			"user-google-scholar":       {Command: "gscholar"},
			"user-fetch":                {Command: "fetch"},
			"user-pdf-handler":          {Command: "pdf-handler"},
			"user-context7":             {Command: "context7"},
			"user-github-official":      {Command: "github"},
			"user-git-mcp-server":       {Command: "git"},
			"user-sentrux":              {Command: "sentrux"},
			"cursor-ide-browser":        {Command: "browser"},
			"user-playwright":           {Command: "playwright"},
			"user-chrome-devtools":      {Command: "devtools"},
			"user-linkedin-mcp":         {Command: "linkedin"},
			"user-upwork-mcp":           {Command: "upwork"},
			"user-ironclaw":             {Command: "ironclaw"},
			"user-plantuml":             {Command: "plantuml"},
			"user-mermaid":              {Command: "mermaid"},
			"user-word-document-server": {Command: "word"},
			"cursor-app-control":        {Command: "app-control"},
			"user-atlassian-jira":       {Command: "jira"},
		},
	}
}

func TestApplyProfile_Research(t *testing.T) {
	cfg := sampleConfig()
	profile, ok := mcpfilter.GetProfile("research")
	if !ok {
		t.Fatal("research profile not found")
	}

	filtered, result := mcpfilter.ApplyProfile(cfg, profile)

	if result.TotalIn != 25 {
		t.Errorf("TotalIn = %d, want 25", result.TotalIn)
	}
	if result.TotalOut != 11 {
		t.Errorf("TotalOut = %d, want 11 (research profile includes 11 servers)", result.TotalOut)
	}
	if result.ReductionPc < 50 {
		t.Errorf("reduction %.1f%%, want >=50%%", result.ReductionPc)
	}

	if _, ok := filtered.MCPServers["user-exa"]; !ok {
		t.Error("research profile should include user-exa")
	}
	if _, ok := filtered.MCPServers["user-linkedin-mcp"]; ok {
		t.Error("research profile should not include user-linkedin-mcp")
	}
}

func TestApplyProfile_CodeReview(t *testing.T) {
	cfg := sampleConfig()
	profile, ok := mcpfilter.GetProfile("code-review")
	if !ok {
		t.Fatal("code-review profile not found")
	}

	filtered, result := mcpfilter.ApplyProfile(cfg, profile)

	if result.TotalOut != 6 {
		t.Errorf("TotalOut = %d, want 6", result.TotalOut)
	}
	if result.ReductionPc < 70 {
		t.Errorf("reduction %.1f%%, want >=70%%", result.ReductionPc)
	}

	for _, required := range []string{"user-git-mcp-server", "user-sentrux"} {
		if _, ok := filtered.MCPServers[required]; !ok {
			t.Errorf("code-review profile should include %s", required)
		}
	}
}

func TestApplyProfile_Minimal(t *testing.T) {
	cfg := sampleConfig()
	profile, ok := mcpfilter.GetProfile("minimal")
	if !ok {
		t.Fatal("minimal profile not found")
	}

	_, result := mcpfilter.ApplyProfile(cfg, profile)

	if result.TotalOut != 4 {
		t.Errorf("TotalOut = %d, want 4", result.TotalOut)
	}
	if result.ReductionPc < 80 {
		t.Errorf("reduction %.1f%%, want >=80%%", result.ReductionPc)
	}
}

func TestApplyProfile_WithExclude(t *testing.T) {
	cfg := sampleConfig()
	profile := mcpfilter.ProfileDef{
		Name:    "custom",
		Include: []string{"*"},
		Exclude: []string{"user-ironclaw", "user-atlassian-jira"},
	}

	filtered, result := mcpfilter.ApplyProfile(cfg, profile)

	if result.TotalOut != 23 {
		t.Errorf("TotalOut = %d, want 23 (25 - 2 excluded)", result.TotalOut)
	}
	if _, ok := filtered.MCPServers["user-ironclaw"]; ok {
		t.Error("excluded server should not be in output")
	}
}

func TestApplyProfile_WildcardPrefix(t *testing.T) {
	cfg := sampleConfig()
	profile := mcpfilter.ProfileDef{
		Name:    "cursor-only",
		Include: []string{"cursor-*"},
	}

	filtered, result := mcpfilter.ApplyProfile(cfg, profile)

	if result.TotalOut != 2 {
		t.Errorf("TotalOut = %d, want 2 (cursor-ide-browser + cursor-app-control)", result.TotalOut)
	}
	if _, ok := filtered.MCPServers["cursor-ide-browser"]; !ok {
		t.Error("should include cursor-ide-browser")
	}
	if _, ok := filtered.MCPServers["cursor-app-control"]; !ok {
		t.Error("should include cursor-app-control")
	}
}

func TestWriteAndLoadMCPConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	cfg := sampleConfig()
	if err := mcpfilter.WriteMCPConfig(cfg, path); err != nil {
		t.Fatalf("WriteMCPConfig: %v", err)
	}

	loaded, err := mcpfilter.LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("LoadMCPConfig: %v", err)
	}

	if len(loaded.MCPServers) != len(cfg.MCPServers) {
		t.Errorf("loaded %d servers, want %d", len(loaded.MCPServers), len(cfg.MCPServers))
	}

	data, _ := os.ReadFile(path)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("output is invalid JSON: %v", err)
	}
}

func TestListProfiles(t *testing.T) {
	profiles := mcpfilter.ListProfiles()
	if len(profiles) < 7 {
		t.Errorf("got %d profiles, want >= 7", len(profiles))
	}

	names := make(map[string]bool)
	for _, p := range profiles {
		names[p.Name] = true
	}

	for _, expected := range []string{"research", "code-review", "deployment", "debug", "writing", "job-hunt", "minimal"} {
		if !names[expected] {
			t.Errorf("missing builtin profile: %s", expected)
		}
	}
}

func TestApplyProfile_EmptyConfig(t *testing.T) {
	cfg := &mcpfilter.MCPConfig{MCPServers: map[string]mcpfilter.MCPServer{}}
	profile, _ := mcpfilter.GetProfile("research")

	_, result := mcpfilter.ApplyProfile(cfg, profile)

	if result.TotalIn != 0 {
		t.Errorf("TotalIn = %d, want 0", result.TotalIn)
	}
	if result.TotalOut != 0 {
		t.Errorf("TotalOut = %d, want 0", result.TotalOut)
	}
	if result.ReductionPc != 0 {
		t.Errorf("reduction = %.1f%%, want 0%%", result.ReductionPc)
	}
}
