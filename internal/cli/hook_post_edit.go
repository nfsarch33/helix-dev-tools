package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var postEditCmd = &cobra.Command{
	Use:   "post-edit",
	Short: "afterFileEdit: format files, sync counts, promote local learnings",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPostEdit(os.Stdin, os.Stdout)
	},
}

type postEditHandler struct {
	log   *logger.Logger
	paths config.Paths
}

func (h *postEditHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	start := time.Now()
	if input.FilePath == "" {
		return hookio.Empty(), nil
	}

	h.formatFile(input.FilePath)
	h.syncCountsIfNeeded(input.FilePath)
	h.promoteLearningsIfNeeded(input.FilePath)

	_ = metrics.Record(h.paths.MetricsFile(), metrics.Event{
		Hook:      "post-edit",
		Action:    "format",
		Category:  "tool",
		LatencyMs: time.Since(start).Milliseconds(),
		Detail:    filepath.Base(input.FilePath),
	})

	return hookio.Empty(), nil
}

func (h *postEditHandler) formatFile(filePath string) {
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
	switch ext {
	case "go":
		h.runFormatter("gofmt", "-w", filePath)
	case "dart":
		h.runFormatter("dart", "format", filePath)
	case "py":
		h.runFormatter("ruff", "format", filePath)
	case "json":
		h.formatJSON(filePath)
	case "graphql":
		h.runGraphQLCodegen(filePath)
	}
}

func (h *postEditHandler) runFormatter(name string, args ...string) {
	if _, err := exec.LookPath(name); err != nil {
		return
	}
	if _, err := runCommandOutput(2*time.Minute, name, args...); err == nil {
		h.log.LogEntry(logger.Entry{
			Level:   "info",
			Message: "formatter completed",
			Hook:    "post-edit",
			Result:  "format",
			Fields: map[string]any{
				"formatter": name,
				"path":      args[len(args)-1],
			},
		})
	}
}

func (h *postEditHandler) formatJSON(filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	// Use encoding/json to re-indent
	var buf strings.Builder
	if err := jsonReformat(&buf, data); err != nil {
		return
	}
	_ = os.WriteFile(filePath, []byte(buf.String()+"\n"), 0o644)
	h.log.LogEntry(logger.Entry{
		Level:   "info",
		Message: "formatter completed",
		Hook:    "post-edit",
		Result:  "format",
		Fields: map[string]any{
			"formatter": "json",
			"path":      filePath,
		},
	})
}

func (h *postEditHandler) runGraphQLCodegen(filePath string) {
	dir := filepath.Dir(filePath)
	repoRoot := findGitRoot(dir)
	if repoRoot == "" {
		return
	}
	makefile := filepath.Join(repoRoot, "Makefile")
	data, err := os.ReadFile(makefile)
	if err != nil {
		return
	}
	if strings.Contains(string(data), "schema-pdg") {
		cmd := exec.Command("make", "schema-pdg")
		cmd.Dir = repoRoot
		if err := cmd.Run(); err == nil {
			h.log.LogEntry(logger.Entry{
				Level:   "info",
				Message: "graphql codegen completed",
				Hook:    "post-edit",
				Result:  "codegen",
				Fields: map[string]any{
					"path": filePath,
					"repo": repoRoot,
					"task": "schema-pdg",
				},
			})
		}
	}
}

func (h *postEditHandler) syncCountsIfNeeded(filePath string) {
	triggers := []string{"SKILL.md", "daily-startup-prompt.md", "skills-index.md", "00-index/SKILL.md", "mcp-index-and-selection-sop.md"}
	for _, t := range triggers {
		if strings.HasSuffix(filePath, t) {
			h.runSyncCounts()
			return
		}
	}
}

func (h *postEditHandler) runSyncCounts() {
	selfBin, err := os.Executable()
	if err != nil {
		return
	}
	_, _ = runCommandOutput(2*time.Minute, selfBin, "sync-counts", "--apply")
}

type learningsAction int

const (
	learningsNone   learningsAction = iota
	learningsLocal                  // workspace .learnings/ edit
	learningsGlobal                 // global learnings edit (via ~/memo or ~/Code/global-kb)
)

func classifyLearningsPath(filePath string) (learningsAction, string) {
	if strings.Contains(filePath, "/.learnings/") {
		workspaceDir := filePath[:strings.Index(filePath, "/.learnings/")]
		return learningsLocal, workspaceDir
	}
	if strings.Contains(filePath, "/memo/learnings/") ||
		strings.Contains(filePath, "/global-kb/learnings/") {
		return learningsGlobal, ""
	}
	return learningsNone, ""
}

func (h *postEditHandler) promoteLearningsIfNeeded(filePath string) {
	selfBin, err := os.Executable()
	if err != nil {
		return
	}

	action, wsDir := classifyLearningsPath(filePath)
	switch action {
	case learningsLocal:
		if isDir(filepath.Join(wsDir, ".learnings")) {
			_, _ = runCommandOutput(2*time.Minute, selfBin, "promote", "--workspace", wsDir)
			h.log.LogEntry(logger.Entry{
				Level:   "info",
				Message: "learnings promoted",
				Hook:    "post-edit",
				Result:  "promote",
				Fields: map[string]any{
					"workspace": wsDir,
				},
			})
		}
	case learningsGlobal:
		h.log.LogEntry(logger.Entry{
			Level:   "info",
			Message: "global learnings promotion skipped",
			Hook:    "post-edit",
			Result:  "skip",
			Fields: map[string]any{
				"reason": "mem0-first routing",
			},
		})
	}
}

func findGitRoot(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func jsonReformat(buf *strings.Builder, data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	return enc.Encode(raw)
}

func runPostEdit(stdin *os.File, stdout *os.File) error {
	paths := config.DefaultPaths()
	handler := &postEditHandler{
		log:   logger.New(paths.LogFile("post-edit")),
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
