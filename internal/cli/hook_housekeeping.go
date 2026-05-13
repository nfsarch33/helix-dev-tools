package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/coordination"
	"github.com/nfsarch33/cursor-tools/internal/health"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/lockfile"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
	"github.com/nfsarch33/cursor-tools/internal/outcomes"
)

var housekeepingCmd = &cobra.Command{
	Use:   "housekeeping",
	Short: "stop: log rotation, git sync, promote learnings on session end",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHousekeeping(os.Stdin, os.Stdout)
	},
}

type housekeepingHandler struct {
	log            *logger.Logger
	paths          config.Paths
	metricsPath    string
	outcomeEmitter outcomes.Emitter
}

func (h *housekeepingHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	started := time.Now()
	defer func() {
		if input == nil {
			return
		}
		status := strings.TrimSpace(input.Status)
		latencyMs := time.Since(started).Milliseconds()
		if h.metricsPath != "" {
			_ = metrics.Record(h.metricsPath, metrics.Event{
				Hook:      "housekeeping",
				Action:    status,
				Category:  "housekeeping",
				Detail:    status,
				LatencyMs: latencyMs,
			})
		}
		recordHookOutcome(h.outcomeEmitter, hookOutcomeParams{
			hookName:  "housekeeping",
			action:    status,
			category:  "housekeeping",
			latencyMs: latencyMs,
			detail:    status,
		})
	}()

	lock := lockfile.NewDirLock(h.paths.LockDir("housekeeping"))
	if err := lock.Acquire(); err != nil {
		h.log.LogEntry(logger.Entry{
			Level:   "warn",
			Message: "housekeeping skipped",
			Hook:    "housekeeping",
			Result:  "skip",
			Fields: map[string]any{
				"reason": "another housekeeping instance running",
			},
		})
		return hookio.Empty(), nil
	}
	defer lock.Release()

	h.rotateAllLogs()
	h.maybeFleetPreflight()
	h.log.LogEntry(logger.Entry{
		Level:   "info",
		Message: "stop event received",
		Hook:    "housekeeping",
		Result:  strings.TrimSpace(input.Status),
		Fields: map[string]any{
			"status": strings.TrimSpace(input.Status),
		},
	})

	if input.Status == "completed" || input.Status == "aborted" {
		h.runResourceProbe()
		h.runSessionHandoff()
		h.runSyncCounts()
		h.runPromoteLearnings()
		h.cleanCoordinationSignals()
		h.syncRepo()
	} else {
		h.pullRepo()
		h.pullCoordinationSignals()
	}

	return hookio.Empty(), nil
}

// fleetPreflightHTTPGet is replaceable for tests (e.g. stub that never hits the network).
var fleetPreflightHTTPGet = fleetPreflightHTTPGetDefault

func fleetPreflightHTTPGetDefault(ctx context.Context, client *http.Client, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// maybeFleetPreflight runs optional non-blocking DRL/Prometheus HTTP probes when
// CURSOR_TOOLS_FLEET_PREFLIGHT=1. Logs warnings on failure; never blocks the hook.
func (h *housekeepingHandler) maybeFleetPreflight() {
	if strings.TrimSpace(os.Getenv("CURSOR_TOOLS_FLEET_PREFLIGHT")) != "1" {
		return
	}
	drl := strings.TrimSpace(os.Getenv("FLEET_DRL_HEALTH_URL"))
	prom := strings.TrimSpace(os.Getenv("FLEET_PROM_HEALTH_URL"))
	ctx := context.Background()
	res := health.RunFleetPreflight(ctx, health.FleetPreflightOptions{
		DRLHealthURL:  drl,
		PromHealthURL: prom,
		HTTPGet: func(ctx context.Context, c *http.Client, u string) (int, error) {
			return fleetPreflightHTTPGet(ctx, c, u)
		},
	})
	for _, p := range res.Probes {
		if p.Err != nil {
			h.log.LogEntry(logger.Entry{
				Level:   "warn",
				Message: fmt.Sprintf("fleet preflight: %s unreachable (%s)", p.Label, p.Err.Error()),
				Hook:    "housekeeping",
				Result:  "fleet-preflight-warn",
				Fields: map[string]any{
					"url":   p.URL,
					"label": p.Label,
				},
			})
			continue
		}
		if p.Status >= 400 {
			h.log.LogEntry(logger.Entry{
				Level:   "warn",
				Message: fmt.Sprintf("fleet preflight: %s HTTP %d", p.Label, p.Status),
				Hook:    "housekeeping",
				Result:  "fleet-preflight-warn",
				Fields: map[string]any{
					"url":    p.URL,
					"status": p.Status,
				},
			})
			continue
		}
		h.log.Log(fmt.Sprintf("fleet preflight: ok %s (%s)", p.Label, p.URL))
	}
}

func (h *housekeepingHandler) rotateAllLogs() {
	logger.RotateAll(h.paths.HooksDir, []string{
		"housekeeping.log",
		"checks.log",
		"mcp-audit.log",
		"guard-shell.log",
		"sanitize-read.log",
		"post-edit.log",
	})
	_ = metrics.RotateFile(h.metricsPath, 512000)
}

func (h *housekeepingHandler) runSessionHandoff() {
	if out, err := runSelfCommandOutput(2*time.Minute, h.paths, "session-handoff"); err != nil {
		h.log.Log(fmt.Sprintf("session-handoff error: %s", string(out)))
	} else {
		h.log.Log("session-handoff: ok")
	}
}

func (h *housekeepingHandler) runResourceProbe() {
	if out, err := runSelfCommandOutput(30*time.Second, h.paths, "resource-probe-once"); err != nil {
		h.log.Log(fmt.Sprintf("resource-probe-once error: %s", string(out)))
	} else {
		h.log.Log("resource-probe-once: ok")
	}
}

func (h *housekeepingHandler) runSyncCounts() {
	if out, err := runSelfCommandOutput(2*time.Minute, h.paths, "sync-counts", "--apply"); err != nil {
		h.log.Log(fmt.Sprintf("sync-counts error: %s", string(out)))
	}
}

func (h *housekeepingHandler) runPromoteLearnings() {
	workspaceDir := os.Getenv("CURSOR_WORKSPACE")
	if workspaceDir == "" || !filepath.IsAbs(workspaceDir) {
		workspaceDir, _ = os.Getwd()
	}
	workspaceDir = filepath.Clean(workspaceDir)
	learningsDir := workspaceDir + "/.learnings"
	if isDir(learningsDir) {
		_, _ = runSelfCommandOutput(2*time.Minute, h.paths, "promote", "--workspace", workspaceDir)
		h.log.Log(fmt.Sprintf("promoted learnings from %s", workspaceDir))
	} else {
		_, _ = runSelfCommandOutput(2*time.Minute, h.paths, "promote")
	}
}

// cleanCoordinationSignals is replaceable for testing.
var cleanCoordinationSignalsFn = defaultCleanCoordinationSignals

func defaultCleanCoordinationSignals(p config.Paths, log *logger.Logger) {
	client, err := newCoordinationClient(p)
	if err != nil {
		log.Log(fmt.Sprintf("coordination cleanup skipped: %v", err))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deleted, err := client.CleanStaleSignals(ctx, 48*time.Hour)
	if err != nil {
		log.Log(fmt.Sprintf("coordination cleanup error: %v", err))
		return
	}
	if deleted > 0 {
		log.Log(fmt.Sprintf("coordination cleanup: removed %d stale signal(s)", deleted))
	}
}

func (h *housekeepingHandler) cleanCoordinationSignals() {
	cleanCoordinationSignalsFn(h.paths, h.log)
}

// pullCoordinationSignalsFn is replaceable for testing.
var pullCoordinationSignalsFn = defaultPullCoordinationSignals

func defaultPullCoordinationSignals(p config.Paths, log *logger.Logger) {
	client, err := newCoordinationClient(p)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	signals, err := client.ListSignals(ctx)
	if err != nil {
		log.Log(fmt.Sprintf("coordination signal pull skipped: %v", err))
		return
	}
	machine := coordination.LocalMachine()
	pending := coordination.FilterPendingTasks(signals, machine)
	if len(pending) > 0 {
		log.LogEntry(logger.Entry{
			Level:   "warn",
			Message: fmt.Sprintf("coordination: %d pending task(s) from other machines", len(pending)),
			Hook:    "housekeeping",
			Result:  "pending-tasks",
			Fields: map[string]any{
				"machine":       machine,
				"pending_count": len(pending),
				"total_signals": len(signals),
			},
		})
	} else {
		log.Log(fmt.Sprintf("coordination: %d signal(s), 0 pending tasks for %s", len(signals), machine))
	}
}

func (h *housekeepingHandler) pullCoordinationSignals() {
	pullCoordinationSignalsFn(h.paths, h.log)
}

// SignalCleanupMaxAge controls how old a non-completed signal must be before
// automatic cleanup. Exposed for testing. Default 48h.
var SignalCleanupMaxAge = 48 * time.Hour

// CleanCoordinationSignals runs the cleanup directly with an explicit client.
// Useful for testing without Mem0 credential resolution.
func CleanCoordinationSignals(ctx context.Context, client *coordination.Client, maxAge time.Duration) (int, error) {
	return client.CleanStaleSignals(ctx, maxAge)
}

func (h *housekeepingHandler) syncRepo() {
	repoPath := h.paths.GlobalKB
	if !isDir(repoPath + "/.git") {
		h.log.Log(fmt.Sprintf("WARN: unified-memory not found at %s", repoPath))
		return
	}
	h.setSSHCommand()
	ensureGitSyncConfig(repoPath)

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

	if err := safeRebase(repoPath); err != nil {
		h.log.Log(fmt.Sprintf("WARN: initial rebase failed: %v", err))
	}

	result := pushWithRetry(repoPath, 3)
	writePushState(h.paths.HooksDir, result)
	if result.Err != nil {
		h.log.Log(fmt.Sprintf("WARN: push failed after %d attempt(s): %v", result.Attempts, result.Err))
	} else {
		h.log.Log(fmt.Sprintf("synced: unified-memory (%d attempt(s))", result.Attempts))
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
	keyPath := h.paths.SSHKeyPath()
	if _, err := os.Stat(keyPath); err == nil {
		os.Setenv("GIT_SSH_COMMAND", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", keyPath))
	}
}

func hasChanges(repoPath string) bool {
	out, err := runCommandOutput(30*time.Second, "git", "-C", repoPath, "status", "--porcelain")
	return err == nil && len(out) > 0
}

// changedFileSummary returns a compact body line describing what changed,
// grouped by top-level directory (e.g. "cursor-config/", "global-memories/").
// Appended to auto commit messages so WSL↔macOS syncs are searchable.
func changedFileSummary(repoPath string) string {
	out, err := runCommandOutput(30*time.Second, "git", "-C", repoPath, "diff", "--cached", "--name-only")
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
	_, err := runCommandOutput(2*time.Minute, "git", fullArgs...)
	return err
}

// gitCmdOutput runs a git command and returns combined stdout+stderr.
func gitCmdOutput(repoPath string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", repoPath}, args...)
	out, err := runCommandOutput(2*time.Minute, "git", fullArgs...)
	return strings.TrimSpace(string(out)), err
}

// safeRebase pulls with rebase and auto-aborts on conflict, preventing a
// half-rebased working tree. Returns nil when rebase succeeds or there is
// nothing to rebase.
func safeRebase(repoPath string) error {
	if err := gitCmd(repoPath, "pull", "--rebase", "origin", "main"); err == nil {
		return nil
	}

	status, _ := gitCmdOutput(repoPath, "status")
	if strings.Contains(status, "rebase in progress") {
		conflicted, _ := gitCmdOutput(repoPath, "diff", "--name-only", "--diff-filter=U")
		_ = gitCmd(repoPath, "rebase", "--abort")
		if conflicted != "" {
			return fmt.Errorf("rebase conflict in: %s", conflicted)
		}
		return fmt.Errorf("rebase conflict (aborted)")
	}

	return fmt.Errorf("pull --rebase failed (offline or non-fast-forward)")
}

type pushResult struct {
	Attempts    int
	Err         error
	Conflicting string
}

// pushWithRetry attempts git push up to maxRetries times with exponential
// backoff. Between retries it re-pulls with safeRebase to incorporate any
// remote changes that caused the push rejection.
func pushWithRetry(repoPath string, maxRetries int) pushResult {
	var lastConflict string
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := gitCmd(repoPath, "push", "origin", "main"); err == nil {
			return pushResult{Attempts: attempt}
		}

		if attempt >= maxRetries {
			break
		}

		delay := time.Duration(attempt*2) * time.Second
		time.Sleep(delay)

		if err := safeRebase(repoPath); err != nil {
			lastConflict = err.Error()
			return pushResult{
				Attempts:    attempt,
				Err:         fmt.Errorf("rebase conflict on retry %d: %w", attempt, err),
				Conflicting: lastConflict,
			}
		}
	}
	return pushResult{
		Attempts:    maxRetries,
		Err:         fmt.Errorf("push rejected after %d attempts", maxRetries),
		Conflicting: lastConflict,
	}
}

// ensureGitSyncConfig sets up rerere and the "ours" merge driver in the
// repo-local git config. Idempotent -- safe to call on every sync.
func ensureGitSyncConfig(repoPath string) {
	_ = gitCmd(repoPath, "config", "--local", "rerere.enabled", "true")
	_ = gitCmd(repoPath, "config", "--local", "rerere.autoupdate", "true")
	_ = gitCmd(repoPath, "config", "--local", "merge.ours.driver", "true")
}

// writePushState persists a small state file so doctor/health-check can
// report on the last sync outcome.
func writePushState(hooksDir string, r pushResult) {
	stateFile := filepath.Join(hooksDir, "last-push-result.txt")
	_ = os.MkdirAll(hooksDir, 0o755)

	status := "success"
	if r.Err != nil {
		status = "failed"
	}
	content := fmt.Sprintf("timestamp: %s\nresult: %s\nattempts: %d\n",
		time.Now().UTC().Format(time.RFC3339), status, r.Attempts)
	if r.Conflicting != "" {
		content += "conflicting: " + r.Conflicting + "\n"
	}
	_ = os.WriteFile(stateFile, []byte(content), 0o644) // #nosec G306 -- personal state file
}

func runHousekeeping(stdin *os.File, stdout *os.File) error {
	paths := config.DefaultPaths()
	handler := &housekeepingHandler{
		log:            logger.New(paths.LogFile("housekeeping")),
		paths:          paths,
		metricsPath:    paths.MetricsFile(),
		outcomeEmitter: hookOutcomeEmitter(paths),
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
