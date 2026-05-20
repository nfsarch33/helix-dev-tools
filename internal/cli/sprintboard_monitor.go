package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var sprintboardMonitorFlags struct {
	sprintID string
}

var sprintboardMonitorCmd = &cobra.Command{
	Use:   "sprintboard-monitor",
	Short: "Append Sprintboard metrics snapshot to sprintboard-monitor.ndjson",
	Long: `Reads ~/.config/helix-dev-tools/sprintboard.db and appends one NDJSON
line to ~/logs/runx/sprintboard-monitor.ndjson for operator dashboards.`,
	RunE: runSprintboardMonitor,
}

func init() {
	sprintboardMonitorCmd.Flags().StringVar(&sprintboardMonitorFlags.sprintID, "sprint", "v7100", "Sprint id to summarize")
}

func runSprintboardMonitor(cmd *cobra.Command, _ []string) error {
	sprintID := sprintboardMonitorFlags.sprintID
	store, err := sprintboard.Open(sprintboard.DefaultDBPath())
	if err != nil {
		return err
	}
	defer store.Close()

	summary, err := store.SprintSummary(sprintID)
	if err != nil {
		return err
	}
	counts := make(map[string]int)
	for st, n := range summary.TicketsByStatus {
		counts[string(st)] = n
	}
	inProgress := counts[string(sprintboard.StatusInProgress)]
	deps, err := countDependencies(sprintID)
	if err != nil {
		return err
	}

	ev := map[string]interface{}{
		"event":       "sprintboard_monitor",
		"ts":          time.Now().Format(time.RFC3339),
		"sprint_id":   sprintID,
		"by_status":   counts,
		"in_progress": inProgress,
		"dep_edges":   deps,
		"total":       summary.TotalTickets,
	}
	path, err := appendMonitorNDJSON(ev)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ok sprintboard-monitor sprint=%s path=%s in_progress=%d\n", sprintID, path, inProgress)
	return nil
}

func countDependencies(sprintID string) (int, error) {
	db, err := sql.Open("sqlite", sprintboard.DefaultDBPath())
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM ticket_dependencies d JOIN tickets t ON t.id = d.ticket_id WHERE t.sprint_id = ?`,
		sprintID,
	).Scan(&n)
	return n, err
}

func appendMonitorNDJSON(ev map[string]interface{}) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, "logs", "runx", "sprintboard-monitor.ndjson")
	data, err := json.Marshal(ev)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return "", err
	}
	return path, nil
}
