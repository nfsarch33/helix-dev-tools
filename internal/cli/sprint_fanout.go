package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/fanout"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
)

var sprintFanoutFlags struct {
	sprintID     string
	ownersPath   string
	outputJSON   bool
	dryRun       bool
	handoffDir   string
}

var sprintFanoutCmd = &cobra.Command{
	Use:   "sprint-fanout",
	Short: "Assign unassigned sprint tickets to agents based on capabilities",
	Long: `Reads sprintboard tickets and assigns unassigned ones to agents
based on the owner manifest, agent capabilities, and keyword matching.

  cursor-tools sprint-fanout --sprint v6500
  cursor-tools sprint-fanout --sprint v6500 --dry-run
  cursor-tools sprint-fanout --sprint v6500 --json
  cursor-tools sprint-fanout --sprint v6500 --handoff-dir ~/Code/global-kb/session-handoffs/

Assignment rules:
  cursor-parent -> infra, coordination, sprint tooling, monitoring
  claude-code   -> Helixon platform, Engram adapters, migration
  codex         -> EC product, ecommerce, frontend`,
	RunE: runSprintFanout,
}

func init() {
	home, _ := os.UserHomeDir()
	defaultOwners := filepath.Join(home, ".config", "runx", "owners.yaml")
	defaultHandoffs := filepath.Join(home, "Code", "global-kb", "session-handoffs")

	sprintFanoutCmd.Flags().StringVar(&sprintFanoutFlags.sprintID, "sprint", "", "Sprint ID to fan out (required)")
	sprintFanoutCmd.Flags().StringVar(&sprintFanoutFlags.ownersPath, "owners", defaultOwners, "Path to owners.yaml manifest")
	sprintFanoutCmd.Flags().BoolVar(&sprintFanoutFlags.outputJSON, "json", false, "Output assignments as JSON")
	sprintFanoutCmd.Flags().BoolVar(&sprintFanoutFlags.dryRun, "dry-run", false, "Show assignments without persisting")
	sprintFanoutCmd.Flags().StringVar(&sprintFanoutFlags.handoffDir, "handoff-dir", defaultHandoffs, "Directory for generated handoff docs")
	_ = sprintFanoutCmd.MarkFlagRequired("sprint")
}

func runSprintFanout(cmd *cobra.Command, args []string) error {
	dbPath := sprintboard.DefaultDBPath()
	store, err := sprintboard.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open sprintboard: %w", err)
	}
	defer store.Close()

	_, err = store.GetSprint(sprintFanoutFlags.sprintID)
	if err != nil {
		return fmt.Errorf("sprint %q: %w", sprintFanoutFlags.sprintID, err)
	}

	tickets, err := store.ListTickets(sprintFanoutFlags.sprintID)
	if err != nil {
		return fmt.Errorf("list tickets: %w", err)
	}

	profiles := fanout.DefaultAgentProfiles()
	engine := fanout.NewEngine(profiles)
	assignments := engine.AssignTickets(tickets)

	if len(assignments) == 0 {
		fmt.Fprintln(os.Stderr, "No unassigned tickets found.")
		return nil
	}

	if sprintFanoutFlags.outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(assignments)
	}

	fmt.Fprintf(os.Stdout, "Sprint Fanout: %s (%d assignments)\n", sprintFanoutFlags.sprintID, len(assignments))
	fmt.Fprintln(os.Stdout, strings.Repeat("─", 70))
	fmt.Fprintf(os.Stdout, "%-12s %-20s %-40s\n", "Agent", "Ticket", "Title")
	fmt.Fprintln(os.Stdout, strings.Repeat("─", 70))

	for _, a := range assignments {
		title := a.TicketTitle
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Fprintf(os.Stdout, "%-12s %-20s %-40s\n", a.AgentID, a.TicketID, title)
	}

	if sprintFanoutFlags.dryRun {
		fmt.Fprintln(os.Stderr, "\n(dry-run: no changes persisted)")
		return nil
	}

	for _, a := range assignments {
		if err := store.AssignTicket(a.TicketID, a.AgentID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: assign %s to %s: %v\n", a.TicketID, a.AgentID, err)
			continue
		}
	}

	if sprintFanoutFlags.handoffDir != "" {
		if err := os.MkdirAll(sprintFanoutFlags.handoffDir, 0o755); err != nil {
			return fmt.Errorf("create handoff dir: %w", err)
		}
		for _, a := range assignments {
			doc := fanout.GenerateHandoffDoc(a)
			filename := fmt.Sprintf("%s-fanout-%s-%s.md",
				time.Now().Format("2006-01-02"),
				sprintFanoutFlags.sprintID,
				a.AgentID,
			)
			path := filepath.Join(sprintFanoutFlags.handoffDir, filename)
			if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "warning: write handoff %s: %v\n", path, err)
			}
		}
		fmt.Fprintf(os.Stdout, "\nHandoff docs written to %s\n", sprintFanoutFlags.handoffDir)
	}

	fmt.Fprintf(os.Stdout, "\nAssigned %d tickets.\n", len(assignments))
	return nil
}
