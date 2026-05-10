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
	Long: `Starts a supervisor managing auto-rebuild, session-monitor, and agentrace-serve
as concurrent services with independent panic recovery. Listens for SIGINT/SIGTERM
for graceful shutdown.`,
	RunE: runDaemon,
}

// daemonServiceFactory creates the service set for the daemon. Injectable
// for testing; production wires the real services in init().
var daemonServiceFactory = defaultDaemonServices

func defaultDaemonServices() []supervisor.Service {
	return []supervisor.Service{
		&stubService{name: "auto-rebuild"},
		&stubService{name: "session-monitor"},
		&stubService{name: "agentrace-serve"},
	}
}

// stubService is a placeholder that blocks on context until the real
// service implementations land. Each stub will be replaced by wiring
// into existing internal packages.
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
