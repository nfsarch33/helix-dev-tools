package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
	"github.com/nfsarch33/helix-dev-tools/internal/outcomes"
	"github.com/nfsarch33/helix-dev-tools/internal/patterns"
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
	log            *logger.Logger
	metricsPath    string
	outcomeEmitter outcomes.Emitter
}

func (h *sanitizeReadHandler) Handle(_ context.Context, input *hookio.Input) (*hookio.Response, error) {
	start := time.Now()
	if input.FilePath == "" {
		return hookio.Allow(), nil
	}

	basename := filepath.Base(input.FilePath)

	record := func(action, detail string) {
		latencyMs := time.Since(start).Milliseconds()
		memoryLayer, memoryOp, memoryResult := metrics.InferMemoryContextFromReadPath(detail)
		if h.metricsPath != "" {
			_ = metrics.Record(h.metricsPath, metrics.Event{
				Hook:         "sanitize-read",
				Action:       action,
				Category:     "tool",
				LatencyMs:    latencyMs,
				Detail:       detail,
				MemoryLayer:  memoryLayer,
				MemoryOp:     memoryOp,
				MemoryResult: memoryResult,
			})
		}
		recordHookOutcome(h.outcomeEmitter, hookOutcomeParams{
			hookName:     "sanitize-read",
			action:       action,
			category:     "tool",
			latencyMs:    latencyMs,
			detail:       detail,
			memoryLayer:  memoryLayer,
			memoryOp:     memoryOp,
			memoryResult: memoryResult,
		})
	}

	for _, blocked := range patterns.BlockedFilenames {
		if basename == blocked {
			h.log.LogEntry(logger.Entry{
				Level:   "warn",
				Message: "file read blocked",
				Hook:    "sanitize-read",
				Result:  "deny",
				Fields: map[string]any{
					"path":  input.FilePath,
					"match": blocked,
				},
			})
			record("deny", basename)
			return hookio.Deny(
				fmt.Sprintf("BLOCKED: '%s' likely contains secrets", basename),
				fmt.Sprintf("File '%s' was blocked by sanitize-read because it likely contains secrets. Never read secret files.", basename),
			), nil
		}
	}

	if patterns.ContainsAny(input.FilePath, patterns.BlockedDirs) {
		h.log.LogEntry(logger.Entry{
			Level:   "warn",
			Message: "file read blocked",
			Hook:    "sanitize-read",
			Result:  "deny",
			Fields: map[string]any{
				"path":   input.FilePath,
				"reason": "blocked directory",
			},
		})
		record("deny", input.FilePath)
		return hookio.Deny(
			"BLOCKED: path contains secrets directory",
			fmt.Sprintf("Path '%s' is in a secrets directory and was blocked. Do not access secret directories.", input.FilePath),
		), nil
	}

	for _, ext := range patterns.BlockedExtensions {
		if strings.HasSuffix(strings.ToLower(basename), ext) {
			h.log.LogEntry(logger.Entry{
				Level:   "warn",
				Message: "file read blocked",
				Hook:    "sanitize-read",
				Result:  "deny",
				Fields: map[string]any{
					"path": input.FilePath,
					"ext":  ext,
				},
			})
			record("deny", basename)
			return hookio.Deny(
				fmt.Sprintf("BLOCKED: '%s' is a key/certificate file", basename),
				"Key and certificate files are blocked by sanitize-read.",
			), nil
		}
	}

	record("allow", input.FilePath)

	if basename == "SKILL.md" && isSkillPath(input.FilePath) {
		skillName := extractSkillName(input.FilePath)
		if skillName != "" {
			if h.metricsPath != "" {
				_ = metrics.Record(h.metricsPath, metrics.Event{
					Hook:      "skill-activate",
					Action:    "read",
					Category:  "skill",
					LatencyMs: 0,
					Detail:    skillName,
				})
			}
			skillHit := true
			recordHookOutcome(h.outcomeEmitter, hookOutcomeParams{
				hookName:  "skill-activate",
				action:    "read",
				category:  "skill",
				latencyMs: 0,
				detail:    skillName,
				skillHit:  &skillHit,
				extraMeta: map[string]string{"skill": skillName},
			})
		}
	}

	return hookio.Allow(), nil
}

// isSkillPath returns true if the file path is inside a known skills directory.
// Covers: /skills/, /skills-cursor/, /agents/skills/, /agents-skills/, and plugin cache paths.
func isSkillPath(path string) bool {
	return strings.Contains(path, "/skills/") ||
		strings.Contains(path, "/skills-cursor/") ||
		strings.Contains(path, "/agents-skills/") ||
		strings.Contains(path, "/agents/skills/")
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
		log:            logger.New(paths.LogFile("sanitize-read")),
		metricsPath:    paths.MetricsFile(),
		outcomeEmitter: hookOutcomeEmitter(paths),
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
