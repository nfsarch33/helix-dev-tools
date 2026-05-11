package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nfsarch33/cursor-tools/internal/supervisor"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run cursor-tools as a long-lived daemon with supervised services",
	Long: `Starts a supervisor managing the v337 service set:

  - auto-rebuild       rebuild cursor-tools when source changes
  - session-monitor    watch Cursor agent transcripts for stalls
  - agentrace-serve    HTTP API for the agentrace Go port (v327-3)
  - mem0-oci-retry     OCI A1.Flex continuous retry-loop (Phase 0.2)
  - vendor-mirror-sync weekly mirror refresh (v326-5)
  - resource-probe     5-min memory_pressure snapshot writer

All services run concurrently with independent panic recovery and
1s/2s/4s/8s/16s/30s backoff on crashes (reset after 5 min healthy).
The daemon listens for SIGINT/SIGTERM and releases every service
gracefully.

Doctor integration: ` + "`cursor-tools doctor stack`" + ` reports per-service
health via supervisor.Health(name).

Existing ` + "`cursor-tools auto-rebuild`" + ` subcommand (planned v323-2)
stays as a standalone command for development; the daemon's
auto-rebuild service is what production launchd targets.`,
	RunE: runDaemon,
}

// daemonServiceFactory creates the service set for the daemon. Injectable
// for testing; production wires the real services in init().
var daemonServiceFactory = defaultDaemonServices

// daemonServiceNames is the canonical list every doctor/probe consumer
// checks against. Keep in sync with defaultDaemonServices.
var daemonServiceNames = []string{
	"auto-rebuild",
	"session-monitor",
	"agentrace-serve",
	"mem0-oci-retry",
	"vendor-mirror-sync",
	"resource-probe",
}

func defaultDaemonServices() []supervisor.Service {
	return []supervisor.Service{
		&stubService{name: "auto-rebuild"},
		&stubService{name: "session-monitor"},
		&stubService{name: "agentrace-serve"},
		&stubService{name: "mem0-oci-retry"},
		&stubService{name: "vendor-mirror-sync"},
		&stubService{name: "resource-probe"},
	}
}

// stubService is a placeholder that blocks on context until the real
// service implementations land. Each stub will be replaced by wiring
// into existing internal packages (auto-rebuild, session-monitor,
// agentrace) or new ones (mem0-oci-retry, vendor-mirror-sync,
// resource-probe).
type stubService struct {
	name string
}

func (s *stubService) Name() string { return s.name }

func (s *stubService) Run(ctx context.Context) error {
	slog.Info("daemon: service started (stub)", "service", s.name)
	<-ctx.Done()
	slog.Info("daemon: service stopping", "service", s.name)
	return ctx.Err()
}

func runDaemon(cmd *cobra.Command, args []string) error {
	sup := supervisor.New()

	for _, svc := range daemonServiceFactory() {
		if err := sup.Register(svc); err != nil {
			return fmt.Errorf("daemon: register %s: %w", svc.Name(), err)
		}
	}

	slog.Info("daemon: starting supervisor", "services", sup.Names())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return sup.Run(ctx)
}
