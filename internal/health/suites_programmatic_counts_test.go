package health

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
)

func TestProgrammaticCountsAcceptsFiveHookRoutes(t *testing.T) {
	p := programmaticCountsFixture(t)

	routes := []string{
		"post-edit",
		"guard-mcp",
		"sanitize-read",
		"guard-shell",
		"housekeeping",
	}
	var hooks strings.Builder
	for _, route := range routes {
		fmt.Fprintf(&hooks, `{"command":"~/bin/cursor-tools hook %s"}`+"\n", route)
	}
	writeFile(t, filepath.Join(p.CursorConfigDir(), "hooks.json"), hooks.String())

	suite := suiteProgrammaticCounts(p)

	var hookRoutes *Result
	for i := range suite.Results {
		if strings.Contains(suite.Results[i].Name, "hooks.json has") &&
			strings.Contains(suite.Results[i].Name, "Go routes") {
			hookRoutes = &suite.Results[i]
			break
		}
	}
	if hookRoutes == nil {
		t.Fatalf("missing hooks.json Go route count assertion in %+v", suite.Results)
	}
	if !hookRoutes.Passed {
		t.Fatalf("five hook routes should pass count verification, got detail %q", hookRoutes.Detail)
	}
}

func programmaticCountsFixture(t *testing.T) config.Paths {
	t.Helper()

	base := t.TempDir()
	p := config.Paths{
		Home:            base,
		GlobalKB:        filepath.Join(base, "global-kb"),
		SkillsDir:       filepath.Join(base, ".cursor", "skills"),
		AgentsDir:       filepath.Join(base, ".claude", "agents"),
		AgentsSkillsDir: filepath.Join(base, ".agents", "skills"),
		CommandsDir:     filepath.Join(base, ".cursor", "commands"),
	}

	for _, dir := range []string{
		p.CursorConfigDir(),
		p.SkillsDir,
		p.AgentsSkillsDir,
		p.AgentsDir,
		p.CommandsDir,
		filepath.Join(p.SkillsDir, "go"),
		filepath.Join(p.AgentsSkillsDir, "testing"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeFile(t, filepath.Join(p.SkillsDir, "go", "SKILL.md"), "# go\n")
	writeFile(t, filepath.Join(p.AgentsSkillsDir, "testing", "SKILL.md"), "# testing\n")

	for i := 1; i <= 6; i++ {
		writeFile(t, filepath.Join(p.AgentsDir, fmt.Sprintf("agent-%d.md", i)), "agent\n")
		writeFile(t, filepath.Join(p.CommandsDir, fmt.Sprintf("command-%d.md", i)), "command\n")
	}

	return p
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
