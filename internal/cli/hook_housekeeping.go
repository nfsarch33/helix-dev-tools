package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/lockfile"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var housekeepingCmd = &cobra.Command{
	Use:   "housekeeping",
	Short: "stop: log rotation, git sync, promote learnings on session end",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHousekeeping(os.Stdin, os.Stdout)
	},
}

type housekeepingHandler struct {
	log         *logger.Logger
	paths       config.Paths
	metricsPath string
}

func (h *housekeepingHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	started := time.Now()
	defer func() {
		if h.metricsPath == "" || input == nil {
			return
		}
		_ = metrics.Record(h.metricsPath, metrics.Event{
			Hook:      "housekeeping",
			Action:    strings.TrimSpace(input.Status),
			Category:  "housekeeping",
			Detail:    strings.TrimSpace(input.Status),
			LatencyMs: time.Since(started).Milliseconds(),
		})
	}()

	lock := lockfile.NewDirLock(h.paths.LockDir("housekeeping"))
	if err := lock.Acquire(); err != nil {
		h.log.Log("SKIPPED: another housekeeping instance running")
		return hookio.Empty(), nil
	}
	defer lock.Release()

	h.rotateAllLogs()
	h.log.Log(fmt.Sprintf("stop event: status=%s", input.Status))

	if input.Status == "completed" || input.Status == "aborted" {
		h.runSyncCounts()
		h.runPromoteLearnings()
		h.syncRepo()
	} else {
		h.pullRepo()
	}

	return hookio.Empty(), nil
}

func (h *housekeepingHandler) rotateAllLogs() {
	logger.RotateAll(h.paths.HooksDir, []string{
		"housekeeping.log",
		"mcp-audit.log",
		"guard-shell.log",
		"sanitize-read.log",
		"post-edit.log",
		"metrics.jsonl",
	})
}

func (h *housekeepingHandler) runSyncCounts() {
	selfBin, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(selfBin, "sync-counts", "--apply")
	if out, err := cmd.CombinedOutput(); err != nil {
		h.log.Log(fmt.Sprintf("sync-counts error: %s", string(out)))
	}
}

func (h *housekeepingHandler) runPromoteLearnings() {
	selfBin, err := os.Executable()
	if err != nil {
		return
	}
	workspaceDir := os.Getenv("CURSOR_WORKSPACE")
	if workspaceDir == "" || !filepath.IsAbs(workspaceDir) {
		workspaceDir, _ = os.Getwd()
	}
	workspaceDir = filepath.Clean(workspaceDir)
	learningsDir := workspaceDir + "/.learnings"
	if isDir(learningsDir) {
		cmd := exec.Command(selfBin, "promote", "--workspace", workspaceDir) // #nosec G702 -- workspaceDir validated as absolute + cleaned
		_ = cmd.Run()
		h.log.Log(fmt.Sprintf("promoted learnings from %s", workspaceDir))
	} else {
		cmd := exec.Command(selfBin, "promote")
		_ = cmd.Run()
	}
}

func (h *housekeepingHandler) syncRepo() {
	repoPath := h.paths.GlobalKB
	if !isDir(repoPath + "/.git") {
		h.log.Log(fmt.Sprintf("WARN: unified-memory not found at %s", repoPath))
		return
	}
	h.setSSHCommand()

	if hasChanges(repoPath) {
		gitCmd(repoPath, "add", "-A")
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		summary := changedFileSummary(repoPath)
		commitMsg := fmt.Sprintf("auto: session sync [%s]%s", hostname, summary)
		gitCmd(repoPath, "commit", "-m", commitMsg)
		h.log.Log("committed: unified-memory")
	}

	if err := gitCmd(repoPath, "pull", "--rebase", "origin", "main"); err != nil {
		_ = gitCmd(repoPath, "pull", "--ff-only", "origin", "main")
	}

	if err := gitCmd(repoPath, "push"); err != nil {
		h.log.Log("WARN: push failed for unified-memory")
	} else {
		h.log.Log("synced: unified-memory")
	}
}

func (h *housekeepingHandler) pullRepo() {
	repoPath := h.paths.GlobalKB
	if !isDir(repoPath + "/.git") {
		return
	}
	h.setSSHCommand()
	gitCmd(repoPath, "fetch", "origin", "--quiet")
	gitCmd(repoPath, "merge", "--ff-only", "origin/main")
	h.log.Log("pulled: unified-memory")
}

func (h *housekeepingHandler) setSSHCommand() {
	keyPath := h.paths.Home + "/.ssh/agtc"
	if _, err := os.Stat(keyPath); err == nil {
		os.Setenv("GIT_SSH_COMMAND", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", keyPath))
	}
}

func hasChanges(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	out, err := cmd.Output()
	return err == nil && len(out) > 0
}

// changedFileSummary returns a compact body line describing what changed,
// grouped by top-level directory (e.g. "cursor-config/", "global-memories/").
// Appended to auto commit messages so WSL↔macOS syncs are searchable.
func changedFileSummary(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "diff", "--cached", "--name-only")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "/", 2)
		seen[parts[0]] = true
	}
	if len(seen) == 0 {
		return ""
	}
	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	// stable sort
	for i := 0; i < len(dirs)-1; i++ {
		for j := i + 1; j < len(dirs); j++ {
			if dirs[j] < dirs[i] {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}
	return "\n\nChanged: " + strings.Join(dirs, ", ")
}

func gitCmd(repoPath string, args ...string) error {
	fullArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func runHousekeeping(stdin *os.File, stdout *os.File) error {
	paths := config.DefaultPaths()
	handler := &housekeepingHandler{
		log:         logger.New(paths.LogFile("housekeeping")),
		paths:       paths,
		metricsPath: paths.MetricsFile(),
	}

	input, err := hookio.ReadInput(stdin)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Empty())
		return nil
	}

	resp, _ := handler.Handle(context.Background(), input)
	_ = hookio.WriteResponse(stdout, resp)
	return nil
}
