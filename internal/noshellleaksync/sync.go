// Package noshellleaksync verifies and resyncs the canonical no-shell-leak
// Cursor rule across all per-repo mirror locations. It is invoked from the
// `cursor-tools hook guard-no-shell-leak-sync` subcommand at session start
// (Cursor `beforeReadAgent` event) so that any repo opened in the editor has
// the most recent rule before the agent begins issuing tool calls.
//
// v299 D6: 14 mirror targets across personal repos. The canonical source of
// truth lives at $HOME/Code/global-kb/cursor-config/rules/no-shell-leak.mdc.
// Each mirror is keyed by its repo alias path; the syncer never touches
// public mirror repos (ironclaw, openclaw, hermes, etc.) or dormant repos.
package noshellleaksync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// CanonicalRulePath returns the relative path (from $HOME) to the canonical
// no-shell-leak rule file. Used by the resolver and by tests.
const CanonicalRulePath = "Code/global-kb/cursor-config/rules/no-shell-leak.mdc"

// MirrorRelativePath is the per-repo location of the mirrored rule.
const MirrorRelativePath = ".cursor/rules/no-shell-leak.mdc"

// MirrorRepoPaths returns the 14 personal-repo directories (relative to
// $HOME) that should each carry a synchronized copy of the no-shell-leak
// rule. Listed in deterministic order so test diffs are stable.
//
// Public mirror repos (ironclaw, openclaw, hermes, gstack, autoresearch,
// openclaw-rl, windows-mcp, chromedp, Auto-claude-code-research-in-sleep,
// skills, academic-research-skills) are intentionally excluded -- we do not
// own them and must not push private rule content upstream.
//
// Dormant repos (cursor-memory-bank archive, cylrl-auto-agent, invoice-automation,
// wp-wc-ci-cd, k8s-exam-prep, coinbase-vwap-calculation, dungeons-and-lava,
// pbbautoscan) are also excluded -- no active development means no agent
// session, so no mirror needed.
func MirrorRepoPaths() []string {
	return []string{
		"agentic-ai-research",
		"ai-agent-business-stack",
		"Code/global-kb",
		"Code/pdf-mcp-server",
		"Code/secure-auth-platform",
		"cursor-tools",
		"hermes-agent",
		"ironclaw",
		"ironclaw-mcp",
		"ironclaw-ops",
		"linkedin-mcp-server",
		"memo",
		"openclaw-mission-control",
		"upwork-mcp",
	}
}

// FileSHA256 returns the lower-case hex SHA-256 digest of the file at path.
// It is exported for use in tests and by callers that want to surface the
// digest without re-implementing the helper.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is constructed from a deterministic mirror table, not user input.
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SyncResult describes the outcome of resolving and resyncing a single
// mirror target.
type SyncResult struct {
	RepoPath        string // path relative to $HOME (e.g. "memo")
	MirrorPath      string // absolute path to the mirror file
	CanonicalSHA256 string
	MirrorSHA256    string
	Action          Action
	Note            string
}

// Action enumerates the possible per-mirror outcomes.
type Action string

const (
	// ActionInSync means the mirror already matched the canonical SHA-256.
	ActionInSync Action = "in_sync"
	// ActionResynced means the mirror was rewritten with the canonical bytes.
	ActionResynced Action = "resynced"
	// ActionMirrorMissing means the per-repo .cursor/rules/ directory does
	// not exist (e.g. repo never had Cursor rules) and the syncer skipped it.
	// This is *not* a failure -- it is the safe default for repos that opt
	// out of the rule mirror system.
	ActionMirrorMissing Action = "mirror_missing"
	// ActionRepoMissing means the repo directory itself is not present on
	// this machine (e.g. WSL-only repo), and the syncer skipped it.
	ActionRepoMissing Action = "repo_missing"
	// ActionError means the syncer attempted to read or write but failed.
	ActionError Action = "error"
)

// Syncer encapsulates the read/write operations needed to resync mirrors.
// It is parameterised on $HOME (so tests can supply a temp directory) and
// on the canonical/mirror path constants (so tests can override them).
type Syncer struct {
	HomeDir           string
	CanonicalRel      string
	MirrorRel         string
	Repos             []string
	WriteOnDrift      bool
	CreateMissingDirs bool
}

// NewSyncer returns a Syncer wired with the production defaults.
func NewSyncer(homeDir string) *Syncer {
	return &Syncer{
		HomeDir:           homeDir,
		CanonicalRel:      CanonicalRulePath,
		MirrorRel:         MirrorRelativePath,
		Repos:             MirrorRepoPaths(),
		WriteOnDrift:      true,
		CreateMissingDirs: false,
	}
}

// Run resolves the canonical rule, then walks each mirror target in
// MirrorRepoPaths order, returning a per-target SyncResult. The slice is
// always sorted by RepoPath so callers can render stable summaries.
func (s *Syncer) Run() ([]SyncResult, error) {
	canonicalAbs := filepath.Join(s.HomeDir, s.CanonicalRel)
	canonicalSHA, err := FileSHA256(canonicalAbs)
	if err != nil {
		return nil, fmt.Errorf("read canonical %s: %w", canonicalAbs, err)
	}
	canonicalBytes, err := os.ReadFile(canonicalAbs) //nolint:gosec // canonicalAbs derived from controlled relative path.
	if err != nil {
		return nil, fmt.Errorf("read canonical bytes %s: %w", canonicalAbs, err)
	}

	results := make([]SyncResult, 0, len(s.Repos))
	for _, repo := range s.Repos {
		repoAbs := filepath.Join(s.HomeDir, repo)
		mirrorAbs := filepath.Join(repoAbs, s.MirrorRel)
		res := SyncResult{
			RepoPath:        repo,
			MirrorPath:      mirrorAbs,
			CanonicalSHA256: canonicalSHA,
		}

		if _, err := os.Stat(repoAbs); err != nil {
			if os.IsNotExist(err) {
				res.Action = ActionRepoMissing
				res.Note = fmt.Sprintf("repo dir not present at %s", repoAbs)
				results = append(results, res)
				continue
			}
			res.Action = ActionError
			res.Note = err.Error()
			results = append(results, res)
			continue
		}

		mirrorBytes, mirrorErr := os.ReadFile(mirrorAbs) //nolint:gosec // mirrorAbs derived from controlled relative path.
		if mirrorErr != nil {
			if os.IsNotExist(mirrorErr) {
				res.Action = ActionMirrorMissing
				res.Note = fmt.Sprintf("mirror file absent at %s (opted out of rule sync)", mirrorAbs)
				results = append(results, res)
				continue
			}
			res.Action = ActionError
			res.Note = mirrorErr.Error()
			results = append(results, res)
			continue
		}

		mirrorSHAbytes := sha256.Sum256(mirrorBytes)
		res.MirrorSHA256 = hex.EncodeToString(mirrorSHAbytes[:])

		if res.MirrorSHA256 == canonicalSHA {
			res.Action = ActionInSync
			results = append(results, res)
			continue
		}

		if !s.WriteOnDrift {
			res.Action = ActionError
			res.Note = "drift detected (write disabled)"
			results = append(results, res)
			continue
		}

		if err := os.WriteFile(mirrorAbs, canonicalBytes, 0o644); err != nil {
			res.Action = ActionError
			res.Note = fmt.Sprintf("write %s: %v", mirrorAbs, err)
			results = append(results, res)
			continue
		}
		res.Action = ActionResynced
		res.Note = fmt.Sprintf("rewrote %d bytes", len(canonicalBytes))
		results = append(results, res)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].RepoPath < results[j].RepoPath
	})
	return results, nil
}
