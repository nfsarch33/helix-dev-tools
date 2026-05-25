// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop client filters Mem0 capsules by the canonical evoloop-daemon source label and producer-machine name

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/coordination"
	"github.com/nfsarch33/helix-dev-tools/internal/evoloop"
)

// evoloopClient is the subset of *evoloop.Client we exercise. Narrowing the
// surface lets tests inject a fake without touching HTTP at all.
type evoloopClient interface {
	Recent(ctx context.Context, opts evoloop.RecentOptions) ([]evoloop.Capsule, error)
}

// evoloopFactory builds a client for a given resolved config. Tests replace
// this to avoid requiring real MCP config / API key. When debug is non-nil
// the client logs the resolved Mem0 query payload to it before each fetch.
var evoloopFactory = defaultEvoloopFactory

func defaultEvoloopFactory(p config.Paths, debug io.Writer) (evoloopClient, error) {
	apiKey, userID, err := coordination.ResolveCredentials(p.CursorMCPConfig())
	if err != nil {
		return nil, fmt.Errorf("resolving Mem0 credentials: %w", err)
	}
	baseURL := strings.TrimSpace(os.Getenv("MEM0_BASE_URL"))
	appID := strings.TrimSpace(os.Getenv("EVOLOOP_APP_ID"))
	c := evoloop.NewClient(apiKey, userID, baseURL, appID)
	c.Debug = debug
	return c, nil
}

var evoloopRecentFlags struct {
	kind    string
	machine string
	limit   int
	json    bool
	debug   bool
}

var evoloopCmd = &cobra.Command{
	Use:   "evoloop",
	Short: "Read EvoLoop-DRL capsules (rollups and cycles) from Mem0",
	Long: `Read fleet-wide EvoLoop-DRL capsules from the shared Mem0 layer
(app_id: cursor-global-kb) so any Cursor instance can see the latest
self-improvement signal at session start, no matter which node produced it.

Subcommands:
  recent    Show the most recent EvoLoop rollup / cycle capsules`,
	SilenceUsage: true,
}

var evoloopRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show the most recent EvoLoop capsules across the fleet",
	Long: `Queries Mem0 (app_id: cursor-global-kb) for EvoLoop capsules that the
evoloop-daemon publishes from every node and prints them newest-first.

Defaults to --kind=rollup because the daily rollup is the cheap cross-node
summary. Pass --kind=cycle for individual feedback-loop cycles, or --kind=all
to merge both streams.`,
	SilenceUsage: true,
	RunE:         runEvoloopRecent,
}

func init() {
	evoloopRecentCmd.Flags().StringVar(&evoloopRecentFlags.kind, "kind", "rollup", "Capsule kind to list: rollup, cycle, or all")
	evoloopRecentCmd.Flags().StringVar(&evoloopRecentFlags.machine, "machine", "", "Filter by producing machine (e.g. gpu-host-1, macbook). Empty = all machines.")
	evoloopRecentCmd.Flags().IntVar(&evoloopRecentFlags.limit, "limit", 10, "Maximum number of capsules to return")
	evoloopRecentCmd.Flags().BoolVar(&evoloopRecentFlags.json, "json", false, "Output JSON instead of a human-readable table")
	evoloopRecentCmd.Flags().BoolVar(&evoloopRecentFlags.debug, "debug", false, "Print the resolved Mem0 query to stderr before fetching capsules")

	evoloopCmd.AddCommand(evoloopRecentCmd)
}

func parseEvoloopKinds(flag string) ([]evoloop.CapsuleKind, error) {
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case "", "rollup", "rollups":
		return []evoloop.CapsuleKind{evoloop.KindRollup}, nil
	case "cycle", "cycles":
		return []evoloop.CapsuleKind{evoloop.KindCycle}, nil
	case "all", "both":
		return []evoloop.CapsuleKind{evoloop.KindRollup, evoloop.KindCycle}, nil
	default:
		return nil, fmt.Errorf("unknown --kind %q (want rollup, cycle, or all)", flag)
	}
}

func runEvoloopRecent(cmd *cobra.Command, _ []string) error {
	kinds, err := parseEvoloopKinds(evoloopRecentFlags.kind)
	if err != nil {
		return err
	}
	if evoloopRecentFlags.limit <= 0 {
		return errors.New("--limit must be >= 1")
	}

	var debug io.Writer
	if evoloopRecentFlags.debug {
		debug = cmd.ErrOrStderr()
	}
	client, err := evoloopFactory(config.DefaultPaths(), debug)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	capsules, err := client.Recent(ctx, evoloop.RecentOptions{
		Kinds:   kinds,
		Machine: evoloopRecentFlags.machine,
		Limit:   evoloopRecentFlags.limit,
	})
	if err != nil {
		return fmt.Errorf("listing evoloop capsules: %w", err)
	}

	out := cmd.OutOrStdout()
	if evoloopRecentFlags.json {
		return writeEvoloopJSON(out, capsules)
	}
	writeEvoloopTable(out, capsules, evoloopRecentFlags.machine, kinds)
	return nil
}

func writeEvoloopJSON(w io.Writer, capsules []evoloop.Capsule) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if capsules == nil {
		capsules = []evoloop.Capsule{}
	}
	return enc.Encode(capsules)
}

func writeEvoloopTable(w io.Writer, capsules []evoloop.Capsule, machine string, kinds []evoloop.CapsuleKind) {
	filter := describeEvoloopFilter(machine, kinds)

	// Route all output through w (cmd.OutOrStdout) so tests can capture it
	// deterministically and downstream callers can redirect to a file. Errors
	// writing to stdout/stderr are deliberately ignored; callers see nothing
	// they can recover from.
	if len(capsules) == 0 {
		_, _ = fmt.Fprintf(w, "  INFO  no EvoLoop capsules found%s\n", filter)
		return
	}

	banner := strings.Repeat("=", 60)
	_, _ = fmt.Fprintln(w, banner)
	_, _ = fmt.Fprintln(w, "  cursor-tools evoloop recent")
	_, _ = fmt.Fprintln(w, banner)
	_, _ = fmt.Fprintf(w, "\n  Capsules: %d%s\n\n", len(capsules), filter)

	for _, c := range capsules {
		_, _ = fmt.Fprintf(w, "  %s [%s] %s %s\n", capsuleIcon(c.Kind), c.Machine, formatCapsuleTimestamp(c.CreatedAt), summariseCapsule(c))
	}
}

func describeEvoloopFilter(machine string, kinds []evoloop.CapsuleKind) string {
	var parts []string
	kindNames := make([]string, 0, len(kinds))
	for _, k := range kinds {
		kindNames = append(kindNames, string(k))
	}
	if len(kindNames) > 0 {
		parts = append(parts, "kind="+strings.Join(kindNames, ","))
	}
	if machine != "" {
		parts = append(parts, "machine="+machine)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " ") + ")"
}

func summariseCapsule(c evoloop.Capsule) string {
	switch c.Kind {
	case evoloop.KindRollup:
		return fmt.Sprintf("day=%s cycles=%d improved=%d rolled_back=%d mean_delta=%+.3f last_kpi=%.3f",
			c.Day, c.Cycles, c.Improved, c.RolledBack, c.MeanDelta, c.LastKPI)
	case evoloop.KindCycle:
		// Canonical evoloop_cycle capsules record kpi_before/kpi_after.
		// agent_outcome / legacy capsules promoted to KindCycle only carry
		// a single kpi_delta (Day 1 schema reconciliation), so fall back
		// to that when before/after are missing.
		if c.KPIBefore == 0 && c.KPIAfter == 0 && c.KPIDelta != 0 {
			summary := fmt.Sprintf("cycle=%s kpi_delta=%+.3f duration_ms=%d",
				c.CycleID, c.KPIDelta, c.DurationMS)
			if c.Event != "" {
				summary += " event=" + c.Event
			}
			return summary
		}
		delta := c.KPIAfter - c.KPIBefore
		return fmt.Sprintf("cycle=%s kpi=%.3f→%.3f (%+.3f) duration_ms=%d",
			c.CycleID, c.KPIBefore, c.KPIAfter, delta, c.DurationMS)
	default:
		trimmed := strings.TrimSpace(c.Text)
		if len(trimmed) > 120 {
			trimmed = trimmed[:117] + "..."
		}
		return trimmed
	}
}

func capsuleIcon(k evoloop.CapsuleKind) string {
	switch k {
	case evoloop.KindRollup:
		return "R"
	case evoloop.KindCycle:
		return "C"
	default:
		return "?"
	}
}

func formatCapsuleTimestamp(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	return t.UTC().Format("2006-01-02 15:04Z")
}
