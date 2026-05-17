package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
)

var rtkSessionSavingsCmd = &cobra.Command{
	Use:   "rtk-session-savings",
	Short: "Stop: log rtk token savings on session end",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := config.DefaultPaths()
		logFile := filepath.Join(paths.LogDir(), "rtk-session-savings.ndjson")
		bin, err := exec.LookPath("rtk")
		if err != nil {
			return nil
		}
		return collectRtkSavings(bin, logFile)
	},
}

type rtkSavingsEntry struct {
	TotalCommands int    `json:"total_commands"`
	TokensSaved   string `json:"tokens_saved"`
	Efficiency    string `json:"efficiency"`
}

var (
	reCmds       = regexp.MustCompile(`Total commands:\s+([\d,]+)`)
	reTokens     = regexp.MustCompile(`Tokens saved:\s+(\S+)`)
	reEfficiency = regexp.MustCompile(`(\d+\.?\d*%)`)
)

func parseRtkGainOutput(output string) rtkSavingsEntry {
	entry := rtkSavingsEntry{}
	if m := reCmds.FindStringSubmatch(output); len(m) > 1 {
		cleaned := strings.ReplaceAll(m[1], ",", "")
		n, _ := strconv.Atoi(cleaned)
		entry.TotalCommands = n
	}
	if m := reTokens.FindStringSubmatch(output); len(m) > 1 {
		entry.TokensSaved = m[1]
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Efficiency") || strings.Contains(line, "efficiency") {
			if m := reEfficiency.FindStringSubmatch(line); len(m) > 1 {
				entry.Efficiency = m[1]
				break
			}
		}
	}
	return entry
}

func collectRtkSavings(rtkBin, logFile string) error {
	if _, err := os.Stat(rtkBin); err != nil {
		return nil
	}

	out, err := exec.Command(rtkBin, "gain").Output()
	if err != nil {
		return nil
	}
	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil
	}

	entry := parseRtkGainOutput(output)
	return writeRtkSavingsNDJSON(logFile, entry)
}

func writeRtkSavingsNDJSON(logFile string, entry rtkSavingsEntry) error {
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	record := map[string]interface{}{
		"ts":             time.Now().UTC().Format(time.RFC3339),
		"event":          "rtk_session_savings",
		"total_commands": entry.TotalCommands,
		"tokens_saved":   entry.TokensSaved,
		"efficiency":     entry.Efficiency,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
