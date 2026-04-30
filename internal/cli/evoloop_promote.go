package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/coordination"
	"github.com/nfsarch33/cursor-tools/internal/evoloop"
	"github.com/nfsarch33/cursor-tools/internal/mem0outbox"
)

var evoloopPromoteFlags struct {
	auto    bool
	machine string
	gateCmd string
	sigma   float64
	window  time.Duration
	limit   int
	outbox  string
	cursor  string
	userID  string
	json    bool
	debug   bool
}

var evoloopPromoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote EvoLoop rollups under a TDD gate; rollback on KPI regression",
	Long: `Walk recent EvoLoop rollup capsules and promote those that show
improvement (improved cycles >= 1, mean_delta >= 0) to the Mem0 outbox
provided the configured TDD gate exits 0.

Defaults to dry-run: promotion / rollback decisions are reported but no
outbox writes happen. Pass --auto to actually mutate the outbox; --auto
requires --gate-cmd so promotions are never silent.

Rollback pass: for each producing machine, the latest LastKPI sample is
compared against the rolling-window mean; a regression of at least
--sigma standard deviations emits a rollback capsule against the most
recent prior promotion for that machine.`,
	SilenceUsage: true,
	RunE:         runEvoloopPromote,
}

func init() {
	evoloopPromoteCmd.Flags().BoolVar(&evoloopPromoteFlags.auto, "auto", false, "Apply promotion / rollback writes to the Mem0 outbox (default: dry-run)")
	evoloopPromoteCmd.Flags().StringVar(&evoloopPromoteFlags.machine, "machine", "", "Filter candidate rollups to a single producing machine")
	evoloopPromoteCmd.Flags().StringVar(&evoloopPromoteFlags.gateCmd, "gate-cmd", "", "Shell command invoked as the TDD gate; required with --auto. Empty in dry-run skips the gate.")
	evoloopPromoteCmd.Flags().Float64Var(&evoloopPromoteFlags.sigma, "sigma", 1.0, "Standard-deviation threshold for KPI regression (rollback)")
	evoloopPromoteCmd.Flags().DurationVar(&evoloopPromoteFlags.window, "window", 24*time.Hour, "Rolling window for regression analysis")
	evoloopPromoteCmd.Flags().IntVar(&evoloopPromoteFlags.limit, "limit", 20, "Maximum candidate rollups to evaluate")
	evoloopPromoteCmd.Flags().StringVar(&evoloopPromoteFlags.outbox, "outbox", "", "Override path to pending.jsonl (defaults to ~/.cursor/mem0-outbox/pending.jsonl)")
	evoloopPromoteCmd.Flags().StringVar(&evoloopPromoteFlags.cursor, "cursor", "", "Override path to outbox cursor (defaults to ~/.cursor/mem0-outbox/cursor)")
	evoloopPromoteCmd.Flags().StringVar(&evoloopPromoteFlags.userID, "user-id", "", "Override Mem0 user_id for emitted capsules; defaults to MCP-config-resolved id")
	evoloopPromoteCmd.Flags().BoolVar(&evoloopPromoteFlags.json, "json", false, "Output JSON decisions instead of a human-readable summary")
	evoloopPromoteCmd.Flags().BoolVar(&evoloopPromoteFlags.debug, "debug", false, "Print resolved Mem0 query payload to stderr before fetching capsules")

	evoloopCmd.AddCommand(evoloopPromoteCmd)
}

// promotionDeps wires every non-deterministic seam used by
// runEvoloopPromote so tests can drive the command without touching
// HTTP, MCP credentials, the filesystem, or os/exec.
type promotionDeps struct {
	clientFactory func(p config.Paths, debug io.Writer) (evoloopClient, error)
	resolveUserID func(p config.Paths) (string, error)
	gateRunner    func(cmd string) evoloop.GateRunner
	openWriter    func(path string) (writeCloser, error)
	now           func() time.Time
}

type writeCloser interface {
	Append(c mem0outbox.Capsule) error
	Close() error
}

var defaultPromotionDeps = promotionDeps{
	clientFactory: func(p config.Paths, debug io.Writer) (evoloopClient, error) {
		return evoloopFactory(p, debug)
	},
	resolveUserID: func(p config.Paths) (string, error) {
		_, userID, err := coordination.ResolveCredentials(p.CursorMCPConfig())
		if err != nil {
			return "", err
		}
		return userID, nil
	},
	gateRunner: func(cmd string) evoloop.GateRunner {
		return shellGateRunner(cmd)
	},
	openWriter: func(path string) (writeCloser, error) {
		w, err := mem0outbox.NewWriter(path)
		if err != nil {
			return nil, err
		}
		return w, nil
	},
	now: time.Now,
}

// promotionDepsOverride lets tests inject a custom dependency set.
// Production code never sets this. Reset in t.Cleanup to avoid leaking
// state between tests.
var promotionDepsOverride *promotionDeps

func resolvedPromotionDeps() promotionDeps {
	if promotionDepsOverride != nil {
		return *promotionDepsOverride
	}
	return defaultPromotionDeps
}

func runEvoloopPromote(cmd *cobra.Command, _ []string) error {
	if evoloopPromoteFlags.limit <= 0 {
		return errors.New("--limit must be >= 1")
	}
	if evoloopPromoteFlags.sigma <= 0 {
		return errors.New("--sigma must be > 0")
	}
	if evoloopPromoteFlags.window <= 0 {
		return errors.New("--window must be > 0")
	}
	if evoloopPromoteFlags.auto && strings.TrimSpace(evoloopPromoteFlags.gateCmd) == "" {
		return errors.New("--auto requires --gate-cmd; refusing to silently promote")
	}

	deps := resolvedPromotionDeps()
	paths := config.DefaultPaths()

	var debug io.Writer
	if evoloopPromoteFlags.debug {
		debug = cmd.ErrOrStderr()
	}
	client, err := deps.clientFactory(paths, debug)
	if err != nil {
		return err
	}

	userID := strings.TrimSpace(evoloopPromoteFlags.userID)
	if userID == "" {
		resolved, err := deps.resolveUserID(paths)
		if err != nil {
			return fmt.Errorf("resolving Mem0 user id: %w", err)
		}
		userID = resolved
	}
	if userID == "" {
		return errors.New("Mem0 user_id is empty; pass --user-id or configure ~/.cursor/mcp.json")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	rollups, err := client.Recent(ctx, evoloop.RecentOptions{
		Kinds:   []evoloop.CapsuleKind{evoloop.KindRollup},
		Machine: evoloopPromoteFlags.machine,
		Limit:   evoloopPromoteFlags.limit,
	})
	if err != nil {
		return fmt.Errorf("listing rollup capsules: %w", err)
	}
	history, err := client.Recent(ctx, evoloop.RecentOptions{
		Kinds: []evoloop.CapsuleKind{evoloop.KindPromotion, evoloop.KindRollback},
		Limit: 100,
	})
	if err != nil {
		return fmt.Errorf("listing history capsules: %w", err)
	}
	priorPromos := filterPromotionHistory(history)

	outboxPath, cursorPath := resolveOutboxPaths(paths, evoloopPromoteFlags.outbox, evoloopPromoteFlags.cursor)

	runner := &evoloop.PromoteRunner{
		Now:    deps.now,
		Gate:   selectGate(deps, evoloopPromoteFlags.gateCmd),
		UserID: userID,
		DryRun: !evoloopPromoteFlags.auto,
	}

	var closer io.Closer
	if !runner.DryRun {
		w, err := deps.openWriter(outboxPath)
		if err != nil {
			return fmt.Errorf("open outbox writer: %w", err)
		}
		closer = w
		runner.Writer = w.Append
	}

	criteria := evoloop.DefaultPromotionCriteria()
	if evoloopPromoteFlags.machine != "" {
		criteria.OnlyMachines = []string{evoloopPromoteFlags.machine}
	}

	summary, runErr := runner.Run(ctx, evoloop.PromoteOptions{
		Candidates: rollups,
		History:    priorPromos,
		Rollups:    rollups,
		Criteria:   criteria,
		Window:     evoloopPromoteFlags.window,
		Sigma:      evoloopPromoteFlags.sigma,
	})
	if closer != nil {
		_ = closer.Close()
	}
	if runErr != nil {
		return runErr
	}

	out := cmd.OutOrStdout()
	if evoloopPromoteFlags.json {
		return writePromoteJSON(out, summary, !runner.DryRun, outboxPath, cursorPath)
	}
	writePromoteTable(out, summary, !runner.DryRun, outboxPath)
	return nil
}

// selectGate returns the gate runner injected by deps when a non-empty
// command is configured, or a no-op gate when --gate-cmd is empty in
// dry-run mode (the no-op records ExitCode=0 so candidates still
// progress through the runner unchanged).
func selectGate(deps promotionDeps, cmd string) evoloop.GateRunner {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return func(_ context.Context, _ evoloop.Capsule) (evoloop.GateResult, error) {
			return evoloop.GateResult{ExitCode: 0, Stdout: "no --gate-cmd configured (dry-run)"}, nil
		}
	}
	return deps.gateRunner(cmd)
}

// shellGateRunner spawns the supplied command via "sh -c" and surfaces
// stdout/stderr into the GateResult. The capsule fields are exposed to
// the gate via environment variables (CYLRL_CAPSULE_*) so a TDD script
// can scope its run to the producing machine / day if it wants.
func shellGateRunner(command string) evoloop.GateRunner {
	return func(ctx context.Context, c evoloop.Capsule) (evoloop.GateResult, error) {
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Env = append(os.Environ(),
			"CYLRL_CAPSULE_ID="+c.ID,
			"CYLRL_CAPSULE_MACHINE="+c.Machine,
			"CYLRL_CAPSULE_DAY="+c.Day,
			fmt.Sprintf("CYLRL_CAPSULE_LAST_KPI=%.6f", c.LastKPI),
			fmt.Sprintf("CYLRL_CAPSULE_MEAN_DELTA=%.6f", c.MeanDelta),
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				return evoloop.GateResult{
					ExitCode: -1,
					Stdout:   stdout.String(),
					Stderr:   stderr.String(),
				}, err
			}
		}
		return evoloop.GateResult{
			ExitCode: exitCode,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
		}, nil
	}
}

func resolveOutboxPaths(p config.Paths, override, cursorOverride string) (string, string) {
	outbox := strings.TrimSpace(override)
	if outbox == "" {
		outbox = filepath.Join(p.Home, ".cursor", "mem0-outbox", "pending.jsonl")
	}
	cursor := strings.TrimSpace(cursorOverride)
	if cursor == "" {
		cursor = filepath.Join(filepath.Dir(outbox), "cursor")
	}
	return outbox, cursor
}

// filterPromotionHistory keeps only capsules with metadata.kind in
// {evoloop_promotion, evoloop_rollback} so the runner's dedup and
// rollback correlation isn't fooled by unrelated rows that share an
// app_id namespace.
func filterPromotionHistory(history []evoloop.Capsule) []evoloop.Capsule {
	out := make([]evoloop.Capsule, 0, len(history))
	for _, c := range history {
		if c.Metadata == nil {
			continue
		}
		switch c.Metadata["kind"] {
		case "evoloop_promotion", "evoloop_rollback":
			out = append(out, c)
		}
	}
	return out
}

func writePromoteJSON(w io.Writer, summary evoloop.PromoteSummary, applied bool, outboxPath, cursorPath string) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	payload := map[string]any{
		"applied":     applied,
		"outbox_path": outboxPath,
		"cursor_path": cursorPath,
		"summary": map[string]int{
			"promoted":  summary.Promoted,
			"skipped":   summary.Skipped,
			"failed":    summary.Failed,
			"rollbacks": summary.Rollbacks,
		},
		"decisions": summary.Decisions,
	}
	return enc.Encode(payload)
}

func writePromoteTable(w io.Writer, summary evoloop.PromoteSummary, applied bool, outboxPath string) {
	mode := "DRY-RUN"
	if applied {
		mode = "APPLIED"
	}
	banner := strings.Repeat("=", 60)
	_, _ = fmt.Fprintln(w, banner)
	_, _ = fmt.Fprintf(w, "  cursor-tools evoloop promote (%s)\n", mode)
	_, _ = fmt.Fprintln(w, banner)
	_, _ = fmt.Fprintf(w, "\n  promoted=%d skipped=%d failed=%d rollbacks=%d\n",
		summary.Promoted, summary.Skipped, summary.Failed, summary.Rollbacks)
	if applied {
		_, _ = fmt.Fprintf(w, "  outbox: %s\n", outboxPath)
	}
	if len(summary.Decisions) == 0 {
		_, _ = fmt.Fprintln(w, "\n  INFO  no decisions to report")
		return
	}
	_, _ = fmt.Fprintln(w)
	for _, d := range summary.Decisions {
		var icon string
		switch d.State {
		case evoloop.PromotionStatePromoted:
			icon = "P"
		case evoloop.PromotionStateGateFailed:
			icon = "F"
		case evoloop.PromotionStateRolledBack:
			icon = "X"
		default:
			icon = "-"
		}
		_, _ = fmt.Fprintf(w, "  %s [%s] %s state=%s reason=%q\n", icon, d.Machine, d.CapsuleID, d.State, d.Reason)
	}
}
