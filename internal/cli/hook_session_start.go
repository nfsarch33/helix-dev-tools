package cli

import (
	"bufio"
	"context"
	"encoding/json"
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
)

var sessionStartCmd = &cobra.Command{
	Use:   "session-start",
	Short: "sessionStart: workspace doctor, resource probe, sync-counts on session begin",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runSessionStart(os.Stdin, os.Stdout)
	},
}

type sessionStartHandler struct {
	log            *logger.Logger
	paths          config.Paths
	metricsPath    string
	outcomeEmitter outcomes.Emitter
}

func (h *sessionStartHandler) Handle(_ context.Context, _ *hookio.Input) (*hookio.Response, error) {
	started := time.Now()
	defer func() {
		latencyMs := time.Since(started).Milliseconds()
		if h.metricsPath != "" {
			_ = metrics.Record(h.metricsPath, metrics.Event{
				Hook:      "session-start",
				Action:    "run",
				Category:  "housekeeping",
				Detail:    "session-start",
				LatencyMs: latencyMs,
			})
		}
		recordHookOutcome(h.outcomeEmitter, hookOutcomeParams{
			hookName:  "session-start",
			action:    "run",
			category:  "housekeeping",
			latencyMs: latencyMs,
			detail:    "session-start",
		})
	}()

	h.runSyncCounts()
	h.runWorkspaceDoctor()

	probe := h.readResourceProbe()

	h.log.LogEntry(logger.Entry{
		Level:   "info",
		Message: "session-start completed",
		Hook:    "session-start",
		Result:  "ok",
		Fields: map[string]any{
			"resource_tier": probe.Tier,
			"resource_free": probe.FreePct,
			"resource_err":  probe.Err,
		},
	})

	if probe.Tier == "RED" {
		msg := fmt.Sprintf("RESOURCE PROBE: %s (free=%d%%) -- consider closing unused applications before proceeding", probe.Tier, probe.FreePct)
		return &hookio.Response{
			Continue:     true,
			UserMessage:  msg,
			AgentMessage: msg,
		}, nil
	}

	return hookio.Empty(), nil
}

func (h *sessionStartHandler) runWorkspaceDoctor() {
	out, err := runSelfCommandOutput(30*time.Second, h.paths, "workspace", "doctor", "--json")
	if err != nil {
		h.log.Log(fmt.Sprintf("workspace doctor error: %s", string(out)))
		return
	}
	h.log.Log("workspace doctor: ok")
}

func (h *sessionStartHandler) runSyncCounts() {
	out, err := runSelfCommandOutput(30*time.Second, h.paths, "sync-counts", "--apply")
	if err != nil {
		h.log.Log(fmt.Sprintf("sync-counts error: %s", string(out)))
	}
}

func (h *sessionStartHandler) readResourceProbe() resourceProbeSnapshot {
	probePath := filepath.Join(h.paths.Home, "logs", "runx", "resource-probe.ndjson")
	return readLastProbeEntry(probePath)
}

func readLastProbeEntry(path string) resourceProbeSnapshot {
	f, err := os.Open(path)
	if err != nil {
		return resourceProbeSnapshot{Tier: "UNKNOWN", Err: "probe file not found"}
	}
	defer func() { _ = f.Close() }()

	var last string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			last = line
		}
	}
	if last == "" {
		return resourceProbeSnapshot{Tier: "UNKNOWN", Err: "probe file empty"}
	}

	var result resourceProbeSnapshot
	if err := json.Unmarshal([]byte(last), &result); err != nil {
		return resourceProbeSnapshot{Tier: "UNKNOWN", Err: "probe parse error: " + err.Error()}
	}
	return normalizeResourceProbeSnapshot(result)
}

func runSessionStart(stdin *os.File, stdout *os.File) error {
	paths := config.DefaultPaths()
	handler := &sessionStartHandler{
		log:            logger.New(paths.LogFile("session-start")),
		paths:          paths,
		metricsPath:    paths.MetricsFile(),
		outcomeEmitter: hookOutcomeEmitter(paths),
	}

	input, err := hookio.ReadInput(stdin)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Empty())
		return nil
	}

	resp, _ := handler.Handle(context.Background(), input)
	_ = hookio.WriteResponse(stdout, resp)
	return nil
}
