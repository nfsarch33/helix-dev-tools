package replicate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FSSource is the production Source: it reads the Cursor capability
// layer from the host filesystem.
//
// All paths it returns to the planner are absolute. The planner is
// already path-agnostic (it only cares about basenames + source paths
// when emitting actions), so FSSource never has to know what the
// claudeRoot is.
type FSSource struct {
	// HooksFile is the absolute path of the Cursor hooks.json (often a
	// symlink under ~/.cursor/hooks.json that resolves into
	// ~/Code/global-kb/cursor-config/hooks.json). If empty or missing
	// on disk, HooksPath returns "" (planner treats this as no-op).
	HooksFile string
	// MCPFile is the absolute path of the Cursor mcp.json (typically
	// ~/.cursor/mcp.json). If missing on disk, MCPRaw returns nil.
	MCPFile string

	collisions []SkillEntry
}

// NewFSSource is the canonical constructor. Empty hooks/mcp paths are
// allowed; missing files at scan time are treated as "not configured".
func NewFSSource(hooksFile, mcpFile string) *FSSource {
	return &FSSource{HooksFile: hooksFile, MCPFile: mcpFile}
}

// Skills walks each root and returns every immediate child directory
// containing a SKILL.md. Roots are honoured in order; the first
// occurrence of a skill name wins.
func (s *FSSource) Skills(roots []string) ([]SkillEntry, error) {
	seen := map[string]bool{}
	out := []SkillEntry{}
	s.collisions = nil
	for _, root := range roots {
		entries, err := readDir(root)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read skill root %q: %w", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			abs := filepath.Join(root, name)
			skillFile := filepath.Join(abs, "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue // not a skill
			}
			entry := SkillEntry{Name: name, SourceDir: abs}
			if seen[name] {
				s.collisions = append(s.collisions, entry)
				continue
			}
			seen[name] = true
			out = append(out, entry)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// SkillCollisions returns the duplicate-name skills the most recent
// Skills() invocation discarded.
func (s *FSSource) SkillCollisions() []SkillEntry { return s.collisions }

// Agents returns every "*.md" file directly under root.
func (s *FSSource) Agents(root string) ([]AgentEntry, error) {
	entries, err := readDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents root %q: %w", root, err)
	}
	out := []AgentEntry{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, AgentEntry{
			Name:       e.Name(),
			SourceFile: filepath.Join(root, e.Name()),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// HooksPath returns the configured cursor hooks file if it exists on
// disk. Empty otherwise. We resolve the symlink (if any) so the Sink
// records the durable target rather than the cursor-side alias.
func (s *FSSource) HooksPath() string {
	if s.HooksFile == "" {
		return ""
	}
	if _, err := os.Stat(s.HooksFile); err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(s.HooksFile)
	if err != nil {
		return s.HooksFile
	}
	return resolved
}

// MCPRaw returns the raw bytes of the cursor mcp.json file. Missing
// file => empty slice + nil error.
func (s *FSSource) MCPRaw() ([]byte, error) {
	if s.MCPFile == "" {
		return nil, nil
	}
	b, err := os.ReadFile(s.MCPFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return b, nil
}

// readDir is os.ReadDir wrapped to satisfy the test seam. It is a var
// so tests can stub it without exposing it on the struct (the tests
// in this package use the in-memory fakeSource for unit tests; the
// FSSource is only exercised in integration tests against a temp dir).
var readDir = func(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}
