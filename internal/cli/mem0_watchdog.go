// runx-public-repo-gate: allow-file network_topology
// Mem0 OSS canonical local port is 18888 — watchdog default; not a private host.
package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/mem0watchdog"
)

var (
	mem0WatchdogProbeURL  string
	mem0WatchdogInterval  time.Duration
	mem0WatchdogTimeout   time.Duration
	mem0WatchdogThreshold int
	mem0WatchdogLogPath   string
	mem0WatchdogPortPat   string
	mem0WatchdogOnce      bool
	mem0WatchdogKill      bool
)

var mem0WatchdogCmd = &cobra.Command{
	Use:           "mem0-watchdog",
	Short:         "Probe Mem0 OSS tunnel and auto-kill stale ssh forwards",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Probes the Mem0 OSS endpoint (default http://127.0.0.1:18888/docs).

Modes:
  --once       Run a single probe, exit 0 on healthy, 1 on fail (no kill).
  --kill       Run --once then kill stale ssh forwards if probe fails.
  (default)    Long-running daemon: probe every --interval; after
               --threshold consecutive failures, kill stale ssh tunnels
               and reset the counter. Logs to ~/logs/runx/mem0-watchdog.ndjson.

Designed for launchd / systemd / ` + "`cursor-tools daemon`" + ` invocation.`,
	RunE: runMem0Watchdog,
}

func init() {
	d := mem0watchdog.DefaultConfig()
	mem0WatchdogCmd.Flags().StringVar(&mem0WatchdogProbeURL, "probe-url", d.ProbeURL, "HTTP endpoint to probe")
	mem0WatchdogCmd.Flags().DurationVar(&mem0WatchdogInterval, "interval", d.ProbeInterval, "Time between probes")
	mem0WatchdogCmd.Flags().DurationVar(&mem0WatchdogTimeout, "timeout", d.ProbeTimeout, "Probe timeout")
	mem0WatchdogCmd.Flags().IntVar(&mem0WatchdogThreshold, "threshold", d.FailThreshold, "Consecutive failures before tunnel kill")
	mem0WatchdogCmd.Flags().StringVar(&mem0WatchdogLogPath, "log-path", d.LogPath, "NDJSON log path")
	mem0WatchdogCmd.Flags().StringVar(&mem0WatchdogPortPat, "port-pattern", d.PortPattern, "ssh process pattern for kill")
	mem0WatchdogCmd.Flags().BoolVar(&mem0WatchdogOnce, "once", false, "Single probe; exit 0 on ok, 1 on fail")
	mem0WatchdogCmd.Flags().BoolVar(&mem0WatchdogKill, "kill", false, "With --once: kill stale tunnels on probe fail")
}

func runMem0Watchdog(cmd *cobra.Command, args []string) error {
	cfg := mem0watchdog.Config{
		ProbeURL:      mem0WatchdogProbeURL,
		ProbeInterval: mem0WatchdogInterval,
		ProbeTimeout:  mem0WatchdogTimeout,
		FailThreshold: mem0WatchdogThreshold,
		LogPath:       mem0WatchdogLogPath,
		PortPattern:   mem0WatchdogPortPat,
	}
	w := mem0watchdog.New(cfg)

	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if mem0WatchdogOnce {
		probeCtx, cancel := context.WithTimeout(ctx, cfg.ProbeTimeout)
		defer cancel()
		if err := w.Probe(probeCtx); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "mem0-watchdog: probe failed: %v\n", err)
			if mem0WatchdogKill {
				killed, _ := w.KillStaleTunnels()
				fmt.Fprintf(cmd.ErrOrStderr(), "mem0-watchdog: killed %d stale tunnel(s)\n", len(killed))
			}
			return fmt.Errorf("mem0-watchdog: probe failed")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "mem0-watchdog: probe ok")
		return nil
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "mem0-watchdog: starting daemon probe=%s interval=%s threshold=%d\n",
		cfg.ProbeURL, cfg.ProbeInterval, cfg.FailThreshold)
	err := w.Run(ctx)
	if err != nil && err != context.Canceled {
		return err
	}
	return nil
}
