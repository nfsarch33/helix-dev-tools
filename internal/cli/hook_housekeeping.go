package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/lockfile"
	"github.com/nfsarch33/cursor-tools/internal/logger"
)

var housekeepingCmd = &cobra.Command{
	Use:   "housekeeping",
	Short: "stop: log rotation, git sync, promote learnings on session end",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHousekeeping(os.Stdin, os.Stdout)
	},
}

type housekeepingHandler struct {
	log   *logger.Logger
	paths config.Paths
}

func (h *housekeepingHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
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
	if workspaceDir == "" {
		workspaceDir, _ = os.Getwd()
	}
	learningsDir := workspaceDir + "/.learnings"
	if isDir(learningsDir) {
		cmd := exec.Command(selfBin, "promote", "--workspace", workspaceDir)
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
		commitMsg := fmt.Sprintf("auto: session sync [%s]", hostname)
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
		log:   logger.New(paths.LogFile("housekeeping")),
		paths: paths,
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
