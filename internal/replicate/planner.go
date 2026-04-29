package replicate

import (
	"path/filepath"
	"sort"
)

// Planner produces a deterministic Plan for mirroring the supplied
// Source into a Claude Code home directory layout rooted at claudeRoot.
//
// claudeRoot is the absolute path of the Claude home (typically
// "$HOME/.claude"). The four sink paths are derived as:
//
//	<claudeRoot>/skills/<skill-name>     -> Source.Skills().SourceDir
//	<claudeRoot>/agents/<agent-name>.md  -> Source.Agents().SourceFile
//	<claudeRoot>/hooks.json              -> Source.HooksPath()
//	<claudeRoot>/mcp.json                -> rewritten MCPRaw()
//
// The Planner is pure: it does not touch the filesystem and does not
// apply the plan. existingTargets is the set of inode-resolved targets
// the Applier already observed at the sink, used to compute SKIP /
// BACKUP. Pass nil when the caller has not yet sampled the sink (the
// planner then conservatively plans SYMLINK for everything; the
// Applier still re-checks at write time).
type Planner struct {
	ClaudeRoot      string
	ExistingTargets ExistingTargets
}

// ExistingTargets is a snapshot of paths that already exist at the
// sink, so the planner can decide SYMLINK vs SKIP vs BACKUP without
// touching disk during planning.
//
// LinkResolves[targetPath] -> the path the existing symlink points at.
//
//	If targetPath does not exist, omit the key entirely.
//	If targetPath exists but is not a symlink, set the value to "" —
//	the planner emits BACKUP in that case.
//	If targetPath is a symlink that resolves to the desired source,
//	the planner emits SKIP.
type ExistingTargets struct {
	LinkResolves map[string]string
}

// has reports whether a path is known to exist at the sink (regardless
// of whether it is a symlink).
func (e ExistingTargets) has(target string) bool {
	if e.LinkResolves == nil {
		return false
	}
	_, ok := e.LinkResolves[target]
	return ok
}

// resolveOf returns the symlink target if known, else "".
func (e ExistingTargets) resolveOf(target string) string {
	if e.LinkResolves == nil {
		return ""
	}
	return e.LinkResolves[target]
}

// Build returns a deterministic Plan derived from the Source snapshot
// and the planner configuration. Build never returns nil; on Source
// errors it surfaces them as Action{Op: OpError}.
func (p Planner) Build(src Source) Plan {
	plan := Plan{}

	// --- Skills -----------------------------------------------------
	skillRoots := []string{
		filepath.Join(p.ClaudeRoot, "..", ".cursor", "skills"),        // primary cursor catalogue (resolved by caller in production)
		filepath.Join(p.ClaudeRoot, "..", ".cursor", "skills-cursor"), // cursor-bundled skill set
	}
	skills, err := src.Skills(skillRoots)
	if err != nil {
		plan.Skills = append(plan.Skills, Action{
			Op:     OpError,
			Source: "(skills source)",
			Reason: err.Error(),
		})
	} else {
		for _, s := range skills {
			target := filepath.Join(p.ClaudeRoot, "skills", s.Name)
			plan.Skills = append(plan.Skills, p.classify(s.SourceDir, target))
		}
		sort.SliceStable(plan.Skills, func(i, j int) bool {
			return plan.Skills[i].Target < plan.Skills[j].Target
		})
	}

	// --- Agents -----------------------------------------------------
	agentRoot := filepath.Join(p.ClaudeRoot, "..", "Code", "global-kb", "cursor-config", "agents")
	agents, err := src.Agents(agentRoot)
	if err != nil {
		plan.Agents = append(plan.Agents, Action{
			Op:     OpError,
			Source: "(agents source)",
			Reason: err.Error(),
		})
	} else {
		for _, a := range agents {
			target := filepath.Join(p.ClaudeRoot, "agents", a.Name)
			plan.Agents = append(plan.Agents, p.classify(a.SourceFile, target))
		}
		sort.SliceStable(plan.Agents, func(i, j int) bool {
			return plan.Agents[i].Target < plan.Agents[j].Target
		})
	}

	// --- Hooks ------------------------------------------------------
	if hooksSrc := src.HooksPath(); hooksSrc != "" {
		target := filepath.Join(p.ClaudeRoot, "hooks.json")
		plan.Hooks = append(plan.Hooks, p.classify(hooksSrc, target))
	}

	// --- MCP --------------------------------------------------------
	mcpRaw, err := src.MCPRaw()
	switch {
	case err != nil:
		plan.MCP = append(plan.MCP, Action{
			Op:     OpError,
			Source: "(mcp source)",
			Reason: err.Error(),
		})
	case len(mcpRaw) == 0:
		// Missing source MCP config is a valid state — operator may not
		// have wired Cursor MCP yet. Planner emits no action.
	default:
		target := filepath.Join(p.ClaudeRoot, "mcp.json")
		plan.MCP = append(plan.MCP, Action{
			Op:     OpRewrite,
			Source: "(filtered cursor mcp.json)",
			Target: target,
			Reason: "filtered cursor mcp.json (drop disabled+test entries)",
		})
	}

	return plan
}

// classify returns SKIP / SYMLINK / BACKUP for the given source-target
// pair, given the planner's ExistingTargets snapshot.
func (p Planner) classify(source, target string) Action {
	if !p.ExistingTargets.has(target) {
		return Action{Op: OpSymlink, Source: source, Target: target}
	}
	resolve := p.ExistingTargets.resolveOf(target)
	if resolve == source {
		return Action{Op: OpSkip, Source: source, Target: target, Reason: "already linked to source"}
	}
	if resolve == "" {
		return Action{
			Op:     OpBackup,
			Source: source,
			Target: target,
			Reason: "non-symlink occupant; will move to <target>.bak.<UTC> before symlinking",
		}
	}
	// Existing symlink, but pointing somewhere else. Treat as BACKUP
	// so we don't quietly clobber an operator-curated alternative.
	return Action{
		Op:     OpBackup,
		Source: source,
		Target: target,
		Reason: "existing symlink points elsewhere (" + resolve + "); will back up before re-linking",
	}
}
