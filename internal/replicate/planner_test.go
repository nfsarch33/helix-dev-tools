package replicate

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeSource is a deterministic in-memory Source for unit tests.
type fakeSource struct {
	skillsByRoot map[string][]SkillEntry
	collisions   []SkillEntry
	agentsByRoot map[string][]AgentEntry
	hooksPath    string
	mcpRaw       []byte
	skillsErr    error
	agentsErr    error
	mcpErr       error
}

func (f *fakeSource) Skills(roots []string) ([]SkillEntry, error) {
	if f.skillsErr != nil {
		return nil, f.skillsErr
	}
	seen := map[string]bool{}
	out := []SkillEntry{}
	for _, r := range roots {
		for _, s := range f.skillsByRoot[r] {
			if seen[s.Name] {
				f.collisions = append(f.collisions, s)
				continue
			}
			seen[s.Name] = true
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSource) SkillCollisions() []SkillEntry { return f.collisions }
func (f *fakeSource) Agents(root string) ([]AgentEntry, error) {
	if f.agentsErr != nil {
		return nil, f.agentsErr
	}
	return f.agentsByRoot[root], nil
}
func (f *fakeSource) HooksPath() string       { return f.hooksPath }
func (f *fakeSource) MCPRaw() ([]byte, error) { return f.mcpRaw, f.mcpErr }

// expectedRoot for the planner's claudeRoot under unit tests; the
// planner derives the cursor-skills root by joining "..", ".cursor",
// "skills", so we match that derivation when we set up the fake.
const fakeClaudeRoot = "/home/u/.claude"

func cursorSkillRoots(claudeRoot string) (primary, secondary string) {
	primary = filepath.Join(claudeRoot, "..", ".cursor", "skills")
	secondary = filepath.Join(claudeRoot, "..", ".cursor", "skills-cursor")
	return
}

func cursorAgentRoot(claudeRoot string) string {
	return filepath.Join(claudeRoot, "..", "Code", "global-kb", "cursor-config", "agents")
}

func TestPlanner_BuildEmptySource(t *testing.T) {
	t.Parallel()
	p := Planner{ClaudeRoot: fakeClaudeRoot}
	plan := p.Build(&fakeSource{})
	if len(plan.Skills) != 0 || len(plan.Agents) != 0 || len(plan.Hooks) != 0 || len(plan.MCP) != 0 {
		t.Fatalf("empty source must yield empty plan; got %+v", plan)
	}
}

func TestPlanner_BuildAllSymlinkOnFreshTarget(t *testing.T) {
	t.Parallel()
	primary, _ := cursorSkillRoots(fakeClaudeRoot)
	src := &fakeSource{
		skillsByRoot: map[string][]SkillEntry{
			primary: {
				{Name: "alpha", SourceDir: "/src/skills/alpha"},
				{Name: "bravo", SourceDir: "/src/skills/bravo"},
			},
		},
		agentsByRoot: map[string][]AgentEntry{
			cursorAgentRoot(fakeClaudeRoot): {
				{Name: "go-architect.md", SourceFile: "/src/agents/go-architect.md"},
			},
		},
		hooksPath: "/src/hooks.json",
		mcpRaw:    []byte(`{"mcpServers":{}}`),
	}
	p := Planner{ClaudeRoot: fakeClaudeRoot}
	plan := p.Build(src)

	wantSkills := []Action{
		{Op: OpSymlink, Source: "/src/skills/alpha", Target: filepath.Join(fakeClaudeRoot, "skills", "alpha")},
		{Op: OpSymlink, Source: "/src/skills/bravo", Target: filepath.Join(fakeClaudeRoot, "skills", "bravo")},
	}
	if !reflect.DeepEqual(plan.Skills, wantSkills) {
		t.Fatalf("skills mismatch:\nwant=%v\ngot =%v", wantSkills, plan.Skills)
	}
	if len(plan.Agents) != 1 || plan.Agents[0].Op != OpSymlink {
		t.Fatalf("expected single symlink for agent, got %+v", plan.Agents)
	}
	if len(plan.Hooks) != 1 || plan.Hooks[0].Op != OpSymlink || plan.Hooks[0].Target != filepath.Join(fakeClaudeRoot, "hooks.json") {
		t.Fatalf("expected hooks symlink, got %+v", plan.Hooks)
	}
	if len(plan.MCP) != 1 || plan.MCP[0].Op != OpRewrite {
		t.Fatalf("expected mcp rewrite, got %+v", plan.MCP)
	}
}

func TestPlanner_SkipWhenLinkAlreadyCorrect(t *testing.T) {
	t.Parallel()
	primary, _ := cursorSkillRoots(fakeClaudeRoot)
	src := &fakeSource{
		skillsByRoot: map[string][]SkillEntry{
			primary: {{Name: "alpha", SourceDir: "/src/skills/alpha"}},
		},
	}
	tgt := filepath.Join(fakeClaudeRoot, "skills", "alpha")
	p := Planner{
		ClaudeRoot: fakeClaudeRoot,
		ExistingTargets: ExistingTargets{
			LinkResolves: map[string]string{tgt: "/src/skills/alpha"},
		},
	}
	plan := p.Build(src)
	if len(plan.Skills) != 1 || plan.Skills[0].Op != OpSkip {
		t.Fatalf("expected SKIP, got %+v", plan.Skills)
	}
}

func TestPlanner_BackupWhenNonSymlinkPresent(t *testing.T) {
	t.Parallel()
	primary, _ := cursorSkillRoots(fakeClaudeRoot)
	src := &fakeSource{
		skillsByRoot: map[string][]SkillEntry{
			primary: {{Name: "alpha", SourceDir: "/src/skills/alpha"}},
		},
	}
	tgt := filepath.Join(fakeClaudeRoot, "skills", "alpha")
	p := Planner{
		ClaudeRoot: fakeClaudeRoot,
		ExistingTargets: ExistingTargets{
			LinkResolves: map[string]string{tgt: ""},
		},
	}
	plan := p.Build(src)
	if len(plan.Skills) != 1 || plan.Skills[0].Op != OpBackup {
		t.Fatalf("expected BACKUP for non-symlink occupant, got %+v", plan.Skills)
	}
	if !strings.Contains(plan.Skills[0].Reason, "non-symlink") {
		t.Fatalf("BACKUP reason should mention non-symlink occupant, got %q", plan.Skills[0].Reason)
	}
}

func TestPlanner_BackupWhenSymlinkPointsElsewhere(t *testing.T) {
	t.Parallel()
	primary, _ := cursorSkillRoots(fakeClaudeRoot)
	src := &fakeSource{
		skillsByRoot: map[string][]SkillEntry{
			primary: {{Name: "alpha", SourceDir: "/src/skills/alpha"}},
		},
	}
	tgt := filepath.Join(fakeClaudeRoot, "skills", "alpha")
	p := Planner{
		ClaudeRoot: fakeClaudeRoot,
		ExistingTargets: ExistingTargets{
			LinkResolves: map[string]string{tgt: "/some/other/path"},
		},
	}
	plan := p.Build(src)
	if len(plan.Skills) != 1 || plan.Skills[0].Op != OpBackup {
		t.Fatalf("expected BACKUP for divergent symlink, got %+v", plan.Skills)
	}
	if !strings.Contains(plan.Skills[0].Reason, "/some/other/path") {
		t.Fatalf("BACKUP reason should include divergent target, got %q", plan.Skills[0].Reason)
	}
}

func TestPlanner_OrderIsDeterministic(t *testing.T) {
	t.Parallel()
	primary, _ := cursorSkillRoots(fakeClaudeRoot)
	src := &fakeSource{
		skillsByRoot: map[string][]SkillEntry{
			primary: {
				{Name: "zulu", SourceDir: "/src/skills/zulu"},
				{Name: "bravo", SourceDir: "/src/skills/bravo"},
				{Name: "alpha", SourceDir: "/src/skills/alpha"},
			},
		},
	}
	p := Planner{ClaudeRoot: fakeClaudeRoot}
	plan := p.Build(src)
	if len(plan.Skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(plan.Skills))
	}
	for i := 0; i < len(plan.Skills)-1; i++ {
		if plan.Skills[i].Target >= plan.Skills[i+1].Target {
			t.Fatalf("skills not sorted by Target: %v", plan.Skills)
		}
	}
}

func TestPlanner_SourceErrorBubblesAsErrorAction(t *testing.T) {
	t.Parallel()
	src := &fakeSource{skillsErr: errors.New("listing failed")}
	p := Planner{ClaudeRoot: fakeClaudeRoot}
	plan := p.Build(src)
	if len(plan.Skills) != 1 || plan.Skills[0].Op != OpError {
		t.Fatalf("expected single OpError on Source.Skills error, got %+v", plan.Skills)
	}
	if !strings.Contains(plan.Skills[0].Reason, "listing failed") {
		t.Fatalf("OpError reason missing underlying error, got %q", plan.Skills[0].Reason)
	}
}

func TestPlanner_MCPMissingIsNoop(t *testing.T) {
	t.Parallel()
	src := &fakeSource{}
	p := Planner{ClaudeRoot: fakeClaudeRoot}
	plan := p.Build(src)
	if len(plan.MCP) != 0 {
		t.Fatalf("missing MCPRaw must yield no-op MCP plan, got %+v", plan.MCP)
	}
}

func TestPlanner_MCPErrorBubblesAsErrorAction(t *testing.T) {
	t.Parallel()
	src := &fakeSource{mcpErr: errors.New("read failed")}
	p := Planner{ClaudeRoot: fakeClaudeRoot}
	plan := p.Build(src)
	if len(plan.MCP) != 1 || plan.MCP[0].Op != OpError {
		t.Fatalf("expected single OpError on MCPRaw error, got %+v", plan.MCP)
	}
}

func TestPlanner_HooksMissingIsNoop(t *testing.T) {
	t.Parallel()
	src := &fakeSource{} // hooksPath = ""
	p := Planner{ClaudeRoot: fakeClaudeRoot}
	plan := p.Build(src)
	if len(plan.Hooks) != 0 {
		t.Fatalf("missing hooks must yield no-op plan, got %+v", plan.Hooks)
	}
}
