// Package replicate mirrors the Cursor capability layer (skills, agents,
// hooks, MCP config) into a Claude Code home directory.
//
// The package follows ports + adapters: Source describes how to read the
// Cursor side, Sink describes how to write the Claude Code side. The
// Planner is pure (no IO) and produces a deterministic Plan from a
// Source snapshot. The Applier is the only piece that touches the host
// filesystem. This split keeps unit tests fast and host-FS-free.
//
// The default invocation in production is via the
// `replicate-cursor-to-claude-code` cursor-tools subcommand; the
// stand-alone binary at cmd/replicate-cursor-to-claude-code wraps the
// same internal entrypoint for parity with the v257 W1 D2-3 plan path.
package replicate

// OpKind enumerates every action the Applier can take.
//
// The operator-facing rendering is the upper-case string ("SYMLINK",
// "SKIP", ...). Tests assert on these constants, never on free-form
// text, so we never tie tests to log wording.
type OpKind string

const (
	// OpSymlink: create a fresh symlink at Target -> Source.
	OpSymlink OpKind = "SYMLINK"
	// OpSkip: target already points at the right source. No-op.
	OpSkip OpKind = "SKIP"
	// OpBackup: a non-symlink file/dir exists at Target. Move it to
	// "<Target>.bak.<UTC>" before symlinking. Operator override of an
	// installed skill is preserved as a backup.
	OpBackup OpKind = "BACKUP"
	// OpRewrite: write a new file at Target (used for filtered MCP).
	OpRewrite OpKind = "REWRITE"
	// OpError: planner or applier surfaced a problem; recorded so the
	// CLI exit code can reflect partial failure without aborting.
	OpError OpKind = "ERROR"
)

// Action is a single planned (or applied) mirror step.
//
// Source is the absolute path on the Cursor side. Target is the
// absolute path on the Claude Code side. Reason carries any extra
// detail (typically a backup-filename or a planner-rejected reason).
// Both Source and Target are always absolute so the operator can
// inspect what would happen without retracing relative paths.
type Action struct {
	Op     OpKind
	Source string
	Target string
	Reason string
}

// Plan is the full output of the Planner: a deterministic, ordered
// list of Actions per category. Skills and Agents are sorted by Target
// so the dry-run output is stable across runs (important because we
// commit the dry-run text in evidence reports).
type Plan struct {
	Skills []Action
	Agents []Action
	Hooks  []Action
	MCP    []Action
}

// SkillEntry is one Cursor skill the Source returned. Name is the
// directory basename (used as the symlink leaf) and SourceDir is the
// absolute path the symlink will point at.
type SkillEntry struct {
	Name      string
	SourceDir string
}

// AgentEntry is one Cursor agent. Name is the file basename (e.g.
// "go-architect.md"); SourceFile is the absolute path of the .md.
type AgentEntry struct {
	Name       string
	SourceFile string
}

// Source describes how to read the Cursor-side capability layer.
//
// Implementations are: a filesystem-backed Source (production) and an
// in-memory Source (tests). All paths returned are absolute.
type Source interface {
	// Skills walks each provided root and returns every immediate child
	// directory that contains a SKILL.md file. The order of roots is
	// honoured: the first occurrence of a skill name wins, later
	// duplicates are recorded as Reason="duplicate" SkillEntries (not
	// returned, see SkillCollisions).
	Skills(roots []string) ([]SkillEntry, error)

	// SkillCollisions returns the duplicate-name skills the Skills()
	// pass discarded, so the operator-facing CLI can warn that a skill
	// in the second root was shadowed by the first.
	SkillCollisions() []SkillEntry

	// Agents lists every "*.md" file directly under root.
	Agents(root string) ([]AgentEntry, error)

	// HooksPath returns the absolute path of the cursor hooks.json file
	// the Source observed. Empty string if not found.
	HooksPath() string

	// MCPRaw returns the raw bytes of the cursor mcp.json file. Empty
	// slice + nil error if the file is missing — that is a valid state
	// when the operator has not configured Cursor MCP yet.
	MCPRaw() ([]byte, error)
}
