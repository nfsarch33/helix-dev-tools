package cli

import (
	"context"
	"encoding/json"
	"fmt"
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
	Short: "afterFileEdit: format files, sync counts, promote learnings",
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
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err == nil {
		h.log.Log(fmt.Sprintf("FORMAT: %s path=%s", name, args[len(args)-1]))
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
	h.log.Log(fmt.Sprintf("FORMAT: json path=%s", filePath))
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
			h.log.Log(fmt.Sprintf("CODEGEN: schema-pdg path=%s repo=%s", filePath, repoRoot))
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
	cmd := exec.Command(selfBin, "sync-counts", "--apply")
	_ = cmd.Run()
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
			cmd := exec.Command(selfBin, "promote", "--workspace", wsDir)
			_ = cmd.Run()
			h.log.Log(fmt.Sprintf("PROMOTE: learnings from %s", wsDir))
		}
	case learningsGlobal:
		cmd := exec.Command(selfBin, "promote")
		_ = cmd.Run()
		h.log.Log("PROMOTE: consolidated L1/L2 digests")
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
