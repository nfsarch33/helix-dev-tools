package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
	"github.com/nfsarch33/cursor-tools/internal/patterns"
)

var sanitizeReadExit = os.Exit

var sanitizeReadCmd = &cobra.Command{
	Use:   "sanitize-read",
	Short: "beforeReadFile: block secret file reads",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSanitizeRead(os.Stdin, os.Stdout)
	},
}

type sanitizeReadHandler struct {
	log         *logger.Logger
	metricsPath string
}

func (h *sanitizeReadHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	start := time.Now()
	if input.FilePath == "" {
		return hookio.Allow(), nil
	}

	basename := filepath.Base(input.FilePath)

	record := func(action, detail string) {
		if h.metricsPath != "" {
			memoryLayer, memoryOp, memoryResult := metrics.InferMemoryContextFromReadPath(detail)
			_ = metrics.Record(h.metricsPath, metrics.Event{
				Hook:         "sanitize-read",
				Action:       action,
				Category:     "tool",
				LatencyMs:    time.Since(start).Milliseconds(),
				Detail:       detail,
				MemoryLayer:  memoryLayer,
				MemoryOp:     memoryOp,
				MemoryResult: memoryResult,
			})
		}
	}

	for _, blocked := range patterns.BlockedFilenames {
		if basename == blocked {
			h.log.Log(fmt.Sprintf("BLOCKED file=%q match=%q", input.FilePath, blocked))
			record("deny", basename)
			return hookio.Deny(
				fmt.Sprintf("BLOCKED: '%s' likely contains secrets", basename),
				fmt.Sprintf("File '%s' was blocked by sanitize-read because it likely contains secrets. Never read secret files.", basename),
			), nil
		}
	}

	if patterns.ContainsAny(input.FilePath, patterns.BlockedDirs) {
		h.log.Log(fmt.Sprintf("BLOCKED file=%q dir=secrets", input.FilePath))
		record("deny", input.FilePath)
		return hookio.Deny(
			fmt.Sprintf("BLOCKED: path contains secrets directory"),
			fmt.Sprintf("Path '%s' is in a secrets directory and was blocked. Do not access secret directories.", input.FilePath),
		), nil
	}

	for _, ext := range patterns.BlockedExtensions {
		if strings.HasSuffix(strings.ToLower(basename), ext) {
			h.log.Log(fmt.Sprintf("BLOCKED file=%q ext=%q", input.FilePath, ext))
			record("deny", basename)
			return hookio.Deny(
				fmt.Sprintf("BLOCKED: '%s' is a key/certificate file", basename),
				"Key and certificate files are blocked by sanitize-read.",
			), nil
		}
	}

	record("allow", input.FilePath)

	// Record skill file reads separately so they don't inflate material skill usage.
	if basename == "SKILL.md" && strings.Contains(input.FilePath, "/skills/") {
		skillName := extractSkillName(input.FilePath)
		if skillName != "" && h.metricsPath != "" {
			_ = metrics.Record(h.metricsPath, metrics.Event{
				Hook:      "skill-activate",
				Action:    "read",
				Category:  "skill",
				LatencyMs: 0,
				Detail:    skillName,
			})
		}
	}

	return hookio.Allow(), nil
}

// extractSkillName pulls the skill directory name from a SKILL.md path.
// e.g. "/Users/x/.cursor/skills/rust-mastery/SKILL.md" -> "rust-mastery"
func extractSkillName(filePath string) string {
	dir := filepath.Dir(filePath)
	return filepath.Base(dir)
}

func runSanitizeRead(stdin *os.File, stdout *os.File) error {
	paths := config.DefaultPaths()
	handler := &sanitizeReadHandler{
		log:         logger.New(paths.LogFile("sanitize-read")),
		metricsPath: paths.MetricsFile(),
	}

	input, err := hookio.ReadInput(stdin)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}

	resp, err := handler.Handle(context.Background(), input)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}

	_ = hookio.WriteResponse(stdout, resp)
	if resp.Permission == "deny" {
		sanitizeReadExit(2)
	}
	return nil
}
