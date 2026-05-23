// runx-public-repo-gate: allow-file fleet_host_alias,internal_service_id — EvoLoop client filters Mem0 capsules by the canonical evoloop-daemon source label and wsl1 producer-machine name

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/outcomes"
)

var outcomeFlags struct {
	actor     string
	machine   string
	event     string
	detail    string
	latencyMs int64
	mcpTool   string
	kpiDelta  float64
	skillHit  string
	sessionID string
	sprint    string
	meta      []string
	sink      string
	jsonOut   bool
}

var outcomeCmd = &cobra.Command{
	Use:   "outcome",
	Short: "Emit agent_outcome capsules to Mem0 (app_id=cursor-global-kb)",
	Long: `Emit Sprint v253 agent_outcome capsules so the EvoLoop daemon can fan-in
worker signals. The default sink is the local NDJSON buffer (fast, offline-safe);
override with --sink=memory to publish directly to Mem0.

The emit subcommand is primarily a manual/CLI path; hooks and fleet workers
use the in-process outcomes.Emitter singleton (see internal/outcomes/default.go).`,
}

var outcomeEmitCmd = &cobra.Command{
	Use:   "emit",
	Short: "Emit a single agent_outcome capsule",
	RunE:  runOutcomeEmit,
}

var outcomeRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show the most recent locally-buffered outcomes (NDJSON tail)",
	RunE:  runOutcomeRecent,
}

func init() {
	outcomeCmd.AddCommand(outcomeEmitCmd)
	outcomeCmd.AddCommand(outcomeRecentCmd)

	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.actor, "actor", outcomes.ActorCursorTools, "Actor label (cursor-hook|cursor-tools|fleet-cli|mc-bridge|fallback-bridge|helixon-daemon|evoloop-daemon)")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.machine, "machine", "", "Machine label (defaults to hostname)")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.event, "event", "", "Event name (required, e.g. 'guard-shell:deny' or 'fleet-cli:apply')")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.detail, "detail", "", "Short detail string (<= 240 chars)")
	outcomeEmitCmd.Flags().Int64Var(&outcomeFlags.latencyMs, "latency-ms", 0, "Latency in milliseconds")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.mcpTool, "mcp-tool", "", "Optional MCP tool name")
	outcomeEmitCmd.Flags().Float64Var(&outcomeFlags.kpiDelta, "kpi-delta", 0, "Optional KPI delta")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.skillHit, "skill-hit", "", "Optional skill hit (true|false|empty)")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.sessionID, "session", "", "Session identifier")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.sprint, "sprint", "", "Sprint tag (defaults to v253 or $CURSOR_TOOLS_SPRINT)")
	outcomeEmitCmd.Flags().StringSliceVar(&outcomeFlags.meta, "meta", nil, "Extra metadata as key=value; repeatable")
	outcomeEmitCmd.Flags().StringVar(&outcomeFlags.sink, "sink", "", "Override sink: buffered|memory|multi (default from env)")
	outcomeEmitCmd.Flags().BoolVar(&outcomeFlags.jsonOut, "json", false, "Print the resolved outcome as JSON on stdout")
}

func runOutcomeEmit(_ *cobra.Command, _ []string) error {
	if strings.TrimSpace(outcomeFlags.event) == "" {
		return fmt.Errorf("--event is required")
	}

	actor := strings.TrimSpace(outcomeFlags.actor)
	if actor == "" {
		actor = outcomes.ActorCursorTools
	}
	if !outcomes.IsKnownActor(actor) {
		return fmt.Errorf("unknown actor %q (want one of %s)", actor, strings.Join(outcomes.KnownActors(), ","))
	}

	machine := strings.TrimSpace(outcomeFlags.machine)
	if machine == "" {
		machine = outcomes.LocalMachineLabel()
	}

	meta, err := parseMetaPairs(outcomeFlags.meta)
	if err != nil {
		return err
	}

	var skillHit *bool
	switch strings.ToLower(strings.TrimSpace(outcomeFlags.skillHit)) {
	case "":
		// leave nil
	case "true", "1", "yes":
		v := true
		skillHit = &v
	case "false", "0", "no":
		v := false
		skillHit = &v
	default:
		return fmt.Errorf("invalid --skill-hit %q (want true|false|empty)", outcomeFlags.skillHit)
	}

	sprint := strings.TrimSpace(outcomeFlags.sprint)
	if sprint == "" {
		sprint = sprintFromEnv()
	}

	o := outcomes.Outcome{
		Kind:      outcomes.KindAgentOutcome,
		Actor:     actor,
		Machine:   machine,
		Event:     strings.TrimSpace(outcomeFlags.event),
		Detail:    strings.TrimSpace(outcomeFlags.detail),
		LatencyMs: outcomeFlags.latencyMs,
		McpTool:   strings.TrimSpace(outcomeFlags.mcpTool),
		KPIDelta:  outcomeFlags.kpiDelta,
		SkillHit:  skillHit,
		SessionID: strings.TrimSpace(outcomeFlags.sessionID),
		Sprint:    sprint,
		Meta:      meta,
	}

	paths := config.DefaultPaths()
	sinkChoice := strings.ToLower(strings.TrimSpace(outcomeFlags.sink))
	// Clear any process-wide singleton so env var overrides take effect when
	// the operator explicitly picks a sink.
	if sinkChoice != "" {
		os.Setenv("CURSOR_TOOLS_OUTCOMES_SINK", sinkChoice)
		outcomes.ResetDefaultEmitter()
	}
	var emitter outcomes.Emitter
	switch sinkChoice {
	case "":
		emitter = outcomes.DefaultEmitter(paths.CursorMCPConfig())
	case "buffered":
		be, bErr := outcomes.NewBufferedEmitter(outcomes.BufferedConfig{
			Path: strings.TrimSpace(os.Getenv("CURSOR_TOOLS_OUTCOMES_PATH")),
		})
		if bErr != nil {
			return fmt.Errorf("creating buffered emitter: %w", bErr)
		}
		emitter = be
	case "memory", "multi":
		emitter = outcomes.DefaultEmitter(paths.CursorMCPConfig())
	default:
		return fmt.Errorf("invalid --sink %q (want buffered|memory|multi)", outcomeFlags.sink)
	}

	if err := outcomes.EmitSafeWithLog(emitter, o); err != nil {
		return fmt.Errorf("emitting outcome: %w", err)
	}

	if outcomeFlags.jsonOut {
		// Normalize a copy so stdout shows the canonical capsule (UTC ts,
		// sorted meta, bounded detail) that sinks actually persist.
		preview := o
		preview.Normalize()
		data, mErr := json.MarshalIndent(preview, "", "  ")
		if mErr == nil {
			fmt.Println(string(data))
		}
	}
	clilog.Success("outcome emitted: [%s] %s on %s", o.Actor, o.Event, o.Machine)
	return nil
}

func runOutcomeRecent(_ *cobra.Command, _ []string) error {
	path := strings.TrimSpace(os.Getenv("CURSOR_TOOLS_OUTCOMES_PATH"))
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home dir: %w", err)
		}
		path = home + "/.cache/cursor-tools/outcomes.ndjson"
	}
	data, err := os.ReadFile(path) // #nosec G304 -- explicit operator-supplied or default path
	if err != nil {
		if os.IsNotExist(err) {
			clilog.Info("no outcome buffer yet at %s", path)
			return nil
		}
		return fmt.Errorf("reading outcome buffer: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	limit := 10
	if len(lines) < limit {
		limit = len(lines)
	}
	tail := lines[len(lines)-limit:]

	clilog.Header("cursor-tools outcome recent")
	fmt.Printf("\n  Buffer path: %s\n", path)
	fmt.Printf("  Showing last %d of %d outcomes\n\n", limit, len(lines))
	for _, l := range tail {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var o outcomes.Outcome
		if err := json.Unmarshal([]byte(l), &o); err != nil {
			fmt.Printf("  raw: %s\n", l)
			continue
		}
		fmt.Printf("  [%s] %s on %s", o.Actor, o.Event, o.Machine)
		if o.LatencyMs > 0 {
			fmt.Printf(" (%dms)", o.LatencyMs)
		}
		if o.Detail != "" {
			fmt.Printf(" -- %s", o.Detail)
		}
		fmt.Println()
	}
	return nil
}

func parseMetaPairs(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(pairs))
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --meta %q (expected key=value)", p)
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			return nil, fmt.Errorf("invalid --meta %q (empty key)", p)
		}
		out[k] = v
	}
	return out, nil
}

// used by tests to parse int flags from env, kept here for future extensions.
var _ = strconv.Atoi
