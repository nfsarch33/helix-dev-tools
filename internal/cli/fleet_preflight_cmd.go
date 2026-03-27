package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/health"
)

var (
	fleetPreflightStrict     bool
	fleetPreflightComposeDir string
)

var fleetPreflightCmd = &cobra.Command{
	Use:   "fleet-preflight",
	Short: "HTTP probes for DRL + Prometheus; optional docker compose ps",
	Long: `Runs the same checks as housekeeping when CURSOR_TOOLS_FLEET_PREFLIGHT=1.
Use --strict for CI or gates (exit code 1 on failure). Env: FLEET_DRL_HEALTH_URL,
FLEET_PROM_HEALTH_URL, FLEET_DRL_COMPOSE_DIR.`,
	RunE: runFleetPreflight,
}

func init() {
	fleetPreflightCmd.Flags().BoolVar(&fleetPreflightStrict, "strict", false, "exit with error if any probe fails")
	fleetPreflightCmd.Flags().StringVar(&fleetPreflightComposeDir, "compose-dir", "", "repo root containing docker/docker-compose.drl.yml (overrides FLEET_DRL_COMPOSE_DIR)")
}

func runFleetPreflight(_ *cobra.Command, _ []string) error {
	ctx := context.Background()
	opts := health.FleetPreflightOptions{
		DRLHealthURL:  os.Getenv("FLEET_DRL_HEALTH_URL"),
		PromHealthURL: os.Getenv("FLEET_PROM_HEALTH_URL"),
	}
	cd := strings.TrimSpace(fleetPreflightComposeDir)
	if cd == "" {
		cd = strings.TrimSpace(os.Getenv("FLEET_DRL_COMPOSE_DIR"))
	}
	if cd != "" {
		opts.ComposeDir = cd
	}
	res := health.RunFleetPreflight(ctx, opts)
	for _, p := range res.Probes {
		if p.Err != nil {
			fmt.Fprintf(os.Stderr, "[fleet-preflight] warn %s (%s) err=%v\n", p.Label, p.URL, p.Err)
			continue
		}
		code := p.Status
		if code >= 200 && code <= 299 {
			fmt.Printf("[fleet-preflight] ok %s (%s) HTTP %d\n", p.Label, p.URL, code)
			continue
		}
		fmt.Fprintf(os.Stderr, "[fleet-preflight] warn %s (%s) HTTP %d\n", p.Label, p.URL, code)
	}
	if res.ComposeRan {
		if res.ComposeOK {
			fmt.Printf("[fleet-preflight] ok docker compose ps (%s)\n", res.ComposePath)
		} else {
			fmt.Fprintf(os.Stderr, "[fleet-preflight] warn compose: %s\n", res.ComposeErr)
		}
	}
	if !res.OK() && fleetPreflightStrict {
		return health.ErrFleetPreflightFailed
	}
	if !res.OK() {
		fmt.Fprintln(os.Stderr, "[fleet-preflight] done with warnings (non-strict)")
	}
	return nil
}
