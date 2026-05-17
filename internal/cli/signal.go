package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/coordination"
)

var signalFlags struct {
	to       string
	priority string
	sprint   string
}

var signalCmd = &cobra.Command{
	Use:   "signal",
	Short: "Send coordination signals between Cursor instances via Mem0",
	Long: `Write a coordination signal to the shared Mem0 layer (app_id: cursor-coordination).
Other Cursor instances on any machine can read these signals at session start.

Subcommands:
  state      Record what this machine is currently working on
  task       Dispatch a task to another machine
  blocker    Record a blocker that other machines should be aware of
  decision   Record a decision made during this session
  completed  Record a completed item
  list       List active coordination signals
  search     Search coordination signals`,
}

var signalStateCmd = &cobra.Command{
	Use:   "state [message]",
	Short: "Record what this machine is currently working on",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSignalState,
}

var signalTaskCmd = &cobra.Command{
	Use:   "task [message]",
	Short: "Dispatch a task to another machine",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSignalTask,
}

var signalBlockerCmd = &cobra.Command{
	Use:   "blocker [message]",
	Short: "Record a blocker that other machines should know about",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSignalBlocker,
}

var signalAlertWebhookCmd = &cobra.Command{
	Use:   "alert-webhook [json-file|-]",
	Short: "Record an Alertmanager webhook as a coordination blocker",
	Args:  cobra.ExactArgs(1),
	RunE:  runSignalAlertWebhook,
}

var signalDecisionCmd = &cobra.Command{
	Use:   "decision [message]",
	Short: "Record a decision made during this session",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSignalDecision,
}

var signalCompletedCmd = &cobra.Command{
	Use:   "completed [message]",
	Short: "Record a completed work item",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSignalCompleted,
}

var signalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active coordination signals",
	RunE:  runSignalList,
}

var signalSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search coordination signals",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSignalSearch,
}

func init() {
	signalCmd.PersistentFlags().StringVar(&signalFlags.sprint, "sprint", "", "Sprint identifier for the signal")
	signalTaskCmd.Flags().StringVar(&signalFlags.to, "to", "", "Target machine for the task (e.g. macbook, wsl)")
	signalTaskCmd.Flags().StringVar(&signalFlags.priority, "priority", "normal", "Priority level (low, normal, high)")

	signalCmd.AddCommand(signalStateCmd)
	signalCmd.AddCommand(signalTaskCmd)
	signalCmd.AddCommand(signalBlockerCmd)
	signalCmd.AddCommand(signalAlertWebhookCmd)
	signalCmd.AddCommand(signalDecisionCmd)
	signalCmd.AddCommand(signalCompletedCmd)
	signalCmd.AddCommand(signalListCmd)
	signalCmd.AddCommand(signalSearchCmd)
}

// newCoordinationClient is replaceable for testing.
var newCoordinationClient = defaultCoordinationClient

func defaultCoordinationClient(p config.Paths) (*coordination.Client, error) {
	apiKey, userID, err := coordination.ResolveCredentials(p.CursorMCPConfig())
	if err != nil {
		return nil, fmt.Errorf("resolving Mem0 credentials: %w", err)
	}
	return coordination.NewClient(apiKey, userID, ""), nil
}

func buildSignal(sigType coordination.SignalType, args []string) coordination.Signal {
	return coordination.Signal{
		Type:      sigType,
		Machine:   coordination.LocalMachine(),
		TargetFor: signalFlags.to,
		Message:   strings.Join(args, " "),
		Priority:  signalFlags.priority,
		Sprint:    signalFlags.sprint,
	}
}

func sendSignal(sigType coordination.SignalType, args []string) error {
	p := config.DefaultPaths()
	client, err := newCoordinationClient(p)
	if err != nil {
		return err
	}
	s := buildSignal(sigType, args)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.AddSignal(ctx, s); err != nil {
		return fmt.Errorf("sending signal: %w", err)
	}
	clilog.Success("signal sent: [%s] %s", s.Type, s.Message)
	return nil
}

func runSignalState(_ *cobra.Command, args []string) error {
	return sendSignal(coordination.SignalActiveState, args)
}

func runSignalTask(_ *cobra.Command, args []string) error {
	if signalFlags.to == "" {
		return fmt.Errorf("--to flag is required for task dispatch (e.g. --to macbook)")
	}
	return sendSignal(coordination.SignalTaskDispatch, args)
}

func runSignalBlocker(_ *cobra.Command, args []string) error {
	return sendSignal(coordination.SignalBlocker, args)
}

func runSignalAlertWebhook(_ *cobra.Command, args []string) error {
	payload, err := readSignalWebhookPayload(args[0])
	if err != nil {
		return err
	}
	var webhook coordination.AlertmanagerWebhook
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return fmt.Errorf("parsing Alertmanager webhook: %w", err)
	}

	p := config.DefaultPaths()
	client, err := newCoordinationClient(p)
	if err != nil {
		return err
	}

	s := coordination.AlertmanagerBlockerSignal(webhook, signalFlags.sprint, time.Now())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.AddSignal(ctx, s); err != nil {
		return fmt.Errorf("sending alert blocker signal: %w", err)
	}
	clilog.Success("alert blocker signal sent: %s", s.Message)
	return nil
}

func runSignalDecision(_ *cobra.Command, args []string) error {
	return sendSignal(coordination.SignalDecision, args)
}

func runSignalCompleted(_ *cobra.Command, args []string) error {
	return sendSignal(coordination.SignalCompleted, args)
}

func readSignalWebhookPayload(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading webhook payload: %w", err)
	}
	return data, nil
}

func runSignalList(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	client, err := newCoordinationClient(p)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	signals, err := client.ListSignals(ctx)
	if err != nil {
		return fmt.Errorf("listing signals: %w", err)
	}

	if len(signals) == 0 {
		clilog.Info("no active coordination signals")
		return nil
	}

	machine := coordination.LocalMachine()
	pendingTasks := coordination.FilterPendingTasks(signals, machine)

	clilog.Header("cursor-tools signal list")
	fmt.Printf("\n  Total signals: %d\n", len(signals))
	if len(pendingTasks) > 0 {
		fmt.Printf("  Pending tasks for %s: %d\n", machine, len(pendingTasks))
	}
	fmt.Println()

	for _, s := range signals {
		icon := signalIcon(s.Type)
		target := ""
		if s.TargetFor != "" {
			target = fmt.Sprintf(" → %s", s.TargetFor)
		}
		pri := ""
		if s.Priority != "" && s.Priority != "normal" {
			pri = fmt.Sprintf(" [%s]", s.Priority)
		}
		fmt.Printf("  %s [%s] %s%s%s\n", icon, s.Machine, s.Message, target, pri)
	}
	return nil
}

func runSignalSearch(_ *cobra.Command, args []string) error {
	p := config.DefaultPaths()
	client, err := newCoordinationClient(p)
	if err != nil {
		return err
	}

	query := strings.Join(args, " ")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	signals, err := client.SearchSignals(ctx, query, 10)
	if err != nil {
		return fmt.Errorf("searching signals: %w", err)
	}

	if len(signals) == 0 {
		clilog.Info("no matching signals for %q", query)
		return nil
	}

	clilog.Header("cursor-tools signal search")
	fmt.Printf("\n  Query: %q (%d results)\n\n", query, len(signals))
	for _, s := range signals {
		icon := signalIcon(s.Type)
		fmt.Printf("  %s [%s] %s\n", icon, s.Machine, s.Message)
	}
	return nil
}

func signalIcon(t coordination.SignalType) string {
	switch t {
	case coordination.SignalActiveState:
		return ">"
	case coordination.SignalTaskDispatch:
		return "T"
	case coordination.SignalDecision:
		return "D"
	case coordination.SignalBlocker:
		return "!"
	case coordination.SignalCompleted:
		return "*"
	default:
		return "?"
	}
}
