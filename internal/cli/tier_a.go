package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	telemetryjson "github.com/nfsarch33/offload-telemetry/emitters/ndjson"
	telemetryenvelope "github.com/nfsarch33/offload-telemetry/envelope"
	"github.com/spf13/cobra"
)

// tierARecordArgs is the redacted record schema used by both the CLI
// surface and unit tests. The fields are intentionally minimal: tier,
// decision, route, latency_ms, and an optional sender token. No payload
// or prompt content is recorded.
type tierARecordArgs struct {
	Tier               string
	Decision           string
	Route              string
	Model              string
	LatencyMS          int64
	TokensPerSecond    float64
	TimeToFirstTokenMS int64
	CostUSD            float64
	StatusCode         int
	ParentTaskID       string
	Sender             string
}

var validTierATiers = map[string]struct{}{
	"a": {},
	"b": {},
	"c": {},
}

var validTierADecisions = map[string]struct{}{
	"offloaded":  {},
	"kept_local": {},
	"declined":   {},
}

const tierARouteMaxLen = 256

// validateTierARoute enforces a conservative character class so route
// labels never silently broaden cardinality or break Prometheus
// exposition. Allowed: ASCII letters, digits, dot, hyphen, underscore.
func validateTierARoute(r string) error {
	if r == "" {
		return fmt.Errorf("route is required")
	}
	if len(r) > tierARouteMaxLen {
		return fmt.Errorf("route exceeds %d chars", tierARouteMaxLen)
	}
	for _, c := range r {
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.', c == '-', c == '_':
			continue
		default:
			return fmt.Errorf("route contains forbidden character %q", c)
		}
	}
	return nil
}

// runTierARecord appends one redacted JSONL record to path. The path's
// parent directory is created on first use. Validation errors return a
// descriptive error and never touch the file. The function is safe to
// call from concurrent processes; appends use O_APPEND on POSIX.
func runTierARecord(path string, args tierARecordArgs) error {
	if _, ok := validTierATiers[args.Tier]; !ok {
		return fmt.Errorf("invalid tier %q (allowed: a, b, c)", args.Tier)
	}
	if _, ok := validTierADecisions[args.Decision]; !ok {
		return fmt.Errorf("invalid decision %q (allowed: offloaded, kept_local, declined)", args.Decision)
	}
	if err := validateTierARoute(args.Route); err != nil {
		return err
	}
	if args.LatencyMS < 0 {
		return fmt.Errorf("latency_ms must be >= 0, got %d", args.LatencyMS)
	}

	rec := telemetryenvelope.NewEvent(telemetryenvelope.Input{
		RecordedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		Tier:               args.Tier,
		Decision:           args.Decision,
		Route:              args.Route,
		Model:              args.Model,
		LatencyMS:          args.LatencyMS,
		TokensPerSecond:    args.TokensPerSecond,
		TimeToFirstTokenMS: args.TimeToFirstTokenMS,
		CostUSD:            args.CostUSD,
		StatusCode:         args.StatusCode,
		ParentTaskID:       args.ParentTaskID,
		Sender:             args.Sender,
	})

	body, err := telemetryjson.MarshalLine(rec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write([]byte(body)); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// tierASummary aggregates a tier-A JSONL file by (tier, decision,
// route) and returns a deterministic markdown summary. Designed for
// session-handoff evidence and quick CLI inspection.
func tierASummary(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	type bucketKey struct {
		tier, decision, route string
	}
	buckets := map[bucketKey][]int64{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return "", fmt.Errorf("invalid jsonl on line %d: %w", lineNum, err)
		}
		k := bucketKey{
			tier:     asString(rec["tier"]),
			decision: asString(rec["decision"]),
			route:    asString(rec["route"]),
		}
		latency := asInt64(rec["latency_ms"])
		buckets[k] = append(buckets[k], latency)
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan: %w", err)
	}

	keys := make([]bucketKey, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].tier != keys[j].tier {
			return keys[i].tier < keys[j].tier
		}
		if keys[i].decision != keys[j].decision {
			return keys[i].decision < keys[j].decision
		}
		return keys[i].route < keys[j].route
	})

	var sb strings.Builder
	sb.WriteString("# Tier-A Offload Telemetry Summary\n\n")
	sb.WriteString(fmt.Sprintf("- source: `%s`\n", path))
	sb.WriteString(fmt.Sprintf("- total records: %d\n\n", lineNum))
	sb.WriteString("| tier | decision | route | count | p50_latency_ms | p95_latency_ms |\n")
	sb.WriteString("|------|----------|-------|-------|----------------|----------------|\n")
	for _, k := range keys {
		samples := append([]int64(nil), buckets[k]...)
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		count := len(samples)
		p50 := percentile(samples, 0.50)
		p95 := percentile(samples, 0.95)
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %d | %d |\n", k.tier, k.decision, k.route, count, p50, p95))
	}
	return sb.String(), nil
}

func percentile(sorted []int64, q float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * q)
	return sorted[idx]
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt64(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	}
	return 0
}

var tierAFlags struct {
	tier               string
	decision           string
	route              string
	model              string
	latencyMS          int64
	tokensPerSecond    float64
	timeToFirstTokenMS int64
	costUSD            float64
	statusCode         int
	parentTaskID       string
	sender             string
	path               string
}

var tierACmd = &cobra.Command{
	Use:   "tier-a",
	Short: "Tier-A subagent offload telemetry",
	Long:  "Record redacted tier-A subagent offload decisions and summarise the persisted JSONL stream.",
}

var tierAMetricCmd = &cobra.Command{
	Use:   "metric",
	Short: "Tier-A metric subcommands",
}

var tierAMetricRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Append one redacted tier-A offload record to JSONL",
	Long: `Append a single tier-A offload record to the configured JSONL path.

The recorded fields are intentionally minimal:

  - tier:      enum (a|b|c)
  - decision:  enum (offloaded|kept_local|declined)
  - route:     stable identifier (alphanumeric, dot, hyphen, underscore)
  - latency_ms: non-negative integer
  - operational metrics: model, tokens/s, time-to-first-token, status, cost, parent-task-id
  - sender:    optional caller token

No prompt or payload content is recorded.`,
	RunE: runTierAMetricRecordCmd,
}

var tierAMetricSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Render markdown summary aggregating the tier-A JSONL stream",
	RunE:  runTierAMetricSummaryCmd,
}

func init() {
	tierAMetricRecordCmd.Flags().StringVar(&tierAFlags.tier, "tier", "a", "tier label (a|b|c)")
	tierAMetricRecordCmd.Flags().StringVar(&tierAFlags.decision, "decision", "", "decision label (offloaded|kept_local|declined)")
	tierAMetricRecordCmd.Flags().StringVar(&tierAFlags.route, "route", "", "route label (e.g. claude_code_subagent)")
	tierAMetricRecordCmd.Flags().StringVar(&tierAFlags.model, "model", "", "redacted model id")
	tierAMetricRecordCmd.Flags().Int64Var(&tierAFlags.latencyMS, "latency-ms", 0, "wall-clock latency in milliseconds")
	tierAMetricRecordCmd.Flags().Float64Var(&tierAFlags.tokensPerSecond, "tokens-per-second", 0, "observed output tokens per second")
	tierAMetricRecordCmd.Flags().Int64Var(&tierAFlags.timeToFirstTokenMS, "time-to-first-token-ms", 0, "time to first token in milliseconds")
	tierAMetricRecordCmd.Flags().Float64Var(&tierAFlags.costUSD, "cost-usd", 0, "estimated cost in USD")
	tierAMetricRecordCmd.Flags().IntVar(&tierAFlags.statusCode, "status-code", 0, "upstream status code")
	tierAMetricRecordCmd.Flags().StringVar(&tierAFlags.parentTaskID, "parent-task-id", "", "redacted parent task id")
	tierAMetricRecordCmd.Flags().StringVar(&tierAFlags.sender, "sender", "", "optional caller token (e.g. cursor-ide, router)")
	tierAMetricRecordCmd.Flags().StringVar(&tierAFlags.path, "path", defaultTierAPath(), "JSONL path")

	tierAMetricSummaryCmd.Flags().StringVar(&tierAFlags.path, "path", defaultTierAPath(), "JSONL path")

	tierAMetricCmd.AddCommand(tierAMetricRecordCmd)
	tierAMetricCmd.AddCommand(tierAMetricSummaryCmd)
	tierACmd.AddCommand(tierAMetricCmd)
	rootCmd.AddCommand(tierACmd)
}

func defaultTierAPath() string {
	if v := strings.TrimSpace(os.Getenv("CURSOR_TIER_A_METRICS_PATH")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "tier-a-metrics.jsonl"
	}
	return filepath.Join(home, ".cursor", "hooks", "tier-a-metrics.jsonl")
}

func runTierAMetricRecordCmd(_ *cobra.Command, _ []string) error {
	args := tierARecordArgs{
		Tier:               tierAFlags.tier,
		Decision:           tierAFlags.decision,
		Route:              tierAFlags.route,
		Model:              tierAFlags.model,
		LatencyMS:          tierAFlags.latencyMS,
		TokensPerSecond:    tierAFlags.tokensPerSecond,
		TimeToFirstTokenMS: tierAFlags.timeToFirstTokenMS,
		CostUSD:            tierAFlags.costUSD,
		StatusCode:         tierAFlags.statusCode,
		ParentTaskID:       tierAFlags.parentTaskID,
		Sender:             tierAFlags.sender,
	}
	if err := runTierARecord(tierAFlags.path, args); err != nil {
		return err
	}
	fmt.Fprintf(stdoutWriter(), "tier-a recorded: tier=%s decision=%s route=%s latency_ms=%d -> %s\n",
		args.Tier, args.Decision, args.Route, args.LatencyMS, tierAFlags.path)
	return nil
}

func runTierAMetricSummaryCmd(_ *cobra.Command, _ []string) error {
	out, err := tierASummary(tierAFlags.path)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(stdoutWriter(), out); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}
	return nil
}

// stdoutWriter exists so tests can swap stdout if needed; for now the
// CLI subcommands print directly to os.Stdout.
func stdoutWriter() io.Writer { return os.Stdout }
