package replicate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Applier writes a Plan to the filesystem.
//
// The Applier is the only component that touches the host. It is
// idempotent: re-applying the same plan when targets already point at
// the right source produces SKIP for every action and does not write
// anything. Apply is safe to interrupt; partial application leaves the
// filesystem in a consistent state because each action is one of:
//
//   - rename(target, target.bak.<UTC>)
//   - symlink(source -> target)
//   - write(target_tmp); rename(target_tmp -> target)
//
// All three are atomic at the directory-entry level on POSIX.
type Applier struct {
	// DryRun: if true, no filesystem mutations happen; the returned
	// Actions still report the operation that *would* have been taken.
	DryRun bool

	// FilteredMCP is the bytes the MCP rewrite step writes to the sink
	// for OpRewrite actions. The CLI computes this from FilterMCP() and
	// passes it through; tests can stub it directly.
	FilteredMCP []byte

	// Now is injected for deterministic backup-suffix generation in
	// tests. Defaults to time.Now if nil.
	Now func() time.Time
}

// Apply executes the plan and returns the Actions actually performed,
// in plan order. Each Action's Op may differ from the planner's
// classification when the disk state changed between planning and
// applying (e.g. a target appeared or disappeared). The Reason field
// is updated to reflect the applied behaviour.
func (a *Applier) Apply(plan Plan) ([]Action, error) {
	all := append([]Action{}, plan.Skills...)
	all = append(all, plan.Agents...)
	all = append(all, plan.Hooks...)
	all = append(all, plan.MCP...)

	out := make([]Action, 0, len(all))
	var firstErr error
	for _, act := range all {
		applied, err := a.applyOne(act)
		out = append(out, applied)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return out, firstErr
}

func (a *Applier) applyOne(act Action) (Action, error) {
	switch act.Op {
	case OpError:
		return act, errors.New(act.Reason)
	case OpRewrite:
		return a.rewriteFile(act)
	case OpSkip, OpSymlink, OpBackup:
		return a.symlinkOrBackup(act)
	default:
		return Action{
			Op:     OpError,
			Source: act.Source,
			Target: act.Target,
			Reason: fmt.Sprintf("unsupported op %q", act.Op),
		}, fmt.Errorf("unsupported op %q", act.Op)
	}
}

// symlinkOrBackup re-checks the disk state at apply time. If the
// target already points at the right source, it returns SKIP. If it
// occupies the path with a non-symlink or a wrong symlink, it backs
// up the target before symlinking. If the target is missing, it
// symlinks directly.
//
// In DryRun mode no filesystem mutations happen, including parent
// directory creation -- the contract for `--dry-run` is "show what
// would happen without touching disk", and creating a previously-
// absent claude/skills/ directory would be a visible side effect
// the operator did not consent to.
func (a *Applier) symlinkOrBackup(act Action) (Action, error) {
	if !a.DryRun {
		if err := os.MkdirAll(filepath.Dir(act.Target), 0o755); err != nil {
			return Action{
				Op:     OpError,
				Source: act.Source,
				Target: act.Target,
				Reason: fmt.Sprintf("mkdir parent: %v", err),
			}, err
		}
	}

	info, err := os.Lstat(act.Target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Action{
			Op:     OpError,
			Source: act.Source,
			Target: act.Target,
			Reason: fmt.Sprintf("lstat: %v", err),
		}, err
	}

	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, rerr := os.Readlink(act.Target)
			if rerr == nil && resolved == act.Source {
				return Action{
					Op:     OpSkip,
					Source: act.Source,
					Target: act.Target,
					Reason: "already linked to source",
				}, nil
			}
		}
		// Backup needed.
		bak := act.Target + ".bak." + a.now().UTC().Format("20060102T150405Z")
		if !a.DryRun {
			if err := os.Rename(act.Target, bak); err != nil {
				return Action{
					Op:     OpError,
					Source: act.Source,
					Target: act.Target,
					Reason: fmt.Sprintf("backup rename: %v", err),
				}, err
			}
		}
		if !a.DryRun {
			if err := os.Symlink(act.Source, act.Target); err != nil {
				return Action{
					Op:     OpError,
					Source: act.Source,
					Target: act.Target,
					Reason: fmt.Sprintf("post-backup symlink: %v", err),
				}, err
			}
		}
		return Action{
			Op:     OpBackup,
			Source: act.Source,
			Target: act.Target,
			Reason: "backed up to " + bak,
		}, nil
	}

	// No existing target.
	if !a.DryRun {
		if err := os.Symlink(act.Source, act.Target); err != nil {
			return Action{
				Op:     OpError,
				Source: act.Source,
				Target: act.Target,
				Reason: fmt.Sprintf("symlink: %v", err),
			}, err
		}
	}
	return Action{
		Op:     OpSymlink,
		Source: act.Source,
		Target: act.Target,
		Reason: "fresh symlink",
	}, nil
}

// rewriteFile writes a.FilteredMCP atomically to act.Target via
// "<target>.tmp + rename".
//
// Idempotency: if the existing file at Target is byte-identical to
// FilteredMCP we return OpSkip instead of touching disk. This keeps
// re-runs side-effect free and lets the dry-run output stay stable.
func (a *Applier) rewriteFile(act Action) (Action, error) {
	if a.FilteredMCP == nil {
		return Action{
			Op:     OpError,
			Source: act.Source,
			Target: act.Target,
			Reason: "rewrite requested but FilteredMCP is empty",
		}, errors.New("FilteredMCP empty")
	}
	if existing, err := os.ReadFile(act.Target); err == nil && bytes.Equal(existing, a.FilteredMCP) {
		return Action{
			Op:     OpSkip,
			Source: act.Source,
			Target: act.Target,
			Reason: "filtered MCP already matches",
		}, nil
	}
	if a.DryRun {
		return Action{
			Op:     OpRewrite,
			Source: act.Source,
			Target: act.Target,
			Reason: "would write filtered MCP (dry-run)",
		}, nil
	}
	if err := os.MkdirAll(filepath.Dir(act.Target), 0o755); err != nil {
		return Action{
			Op:     OpError,
			Source: act.Source,
			Target: act.Target,
			Reason: fmt.Sprintf("mkdir parent: %v", err),
		}, err
	}
	tmp := act.Target + ".tmp"
	if err := os.WriteFile(tmp, a.FilteredMCP, 0o600); err != nil {
		return Action{
			Op:     OpError,
			Source: act.Source,
			Target: act.Target,
			Reason: fmt.Sprintf("write tmp: %v", err),
		}, err
	}
	if err := os.Rename(tmp, act.Target); err != nil {
		_ = os.Remove(tmp) // best effort cleanup of the partial write
		return Action{
			Op:     OpError,
			Source: act.Source,
			Target: act.Target,
			Reason: fmt.Sprintf("rename tmp -> target: %v", err),
		}, err
	}
	return Action{
		Op:     OpRewrite,
		Source: act.Source,
		Target: act.Target,
		Reason: "filtered MCP written",
	}, nil
}

func (a *Applier) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// SampleExisting walks the sink and produces an ExistingTargets
// snapshot the planner can consume. The set covers every immediate
// child of skillsDir + agentsDir, plus the hooks.json + mcp.json
// fixtures at the sink root. Missing dirs are silently treated as
// empty.
func SampleExisting(skillsDir, agentsDir, hooksFile, mcpFile string) ExistingTargets {
	out := ExistingTargets{LinkResolves: map[string]string{}}
	for _, dir := range []string{skillsDir, agentsDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			full := filepath.Join(dir, e.Name())
			info, err := os.Lstat(full)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				if dest, rerr := os.Readlink(full); rerr == nil {
					out.LinkResolves[full] = dest
					continue
				}
			}
			out.LinkResolves[full] = ""
		}
	}
	for _, p := range []string{hooksFile, mcpFile} {
		if p == "" {
			continue
		}
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if dest, rerr := os.Readlink(p); rerr == nil {
				out.LinkResolves[p] = dest
				continue
			}
		}
		out.LinkResolves[p] = ""
	}
	return out
}
