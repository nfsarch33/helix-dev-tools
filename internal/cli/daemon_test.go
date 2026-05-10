package cli

import (
	"context"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/supervisor"
)

type testDaemonService struct {
	name    string
	started bool
}

func (s *testDaemonService) Name() string { return s.name }
func (s *testDaemonService) Run(ctx context.Context) error {
	s.started = true
	<-ctx.Done()
	return ctx.Err()
}

func TestDaemonCmd_Exists(t *testing.T) {
	if daemonCmd == nil {
		t.Fatal("daemonCmd should be defined")
	}
	if daemonCmd.Use != "daemon" {
		t.Errorf("expected Use=daemon, got %s", daemonCmd.Use)
	}
}

func TestDefaultDaemonServices_Names(t *testing.T) {
	svcs := defaultDaemonServices()
	if len(svcs) != 3 {
		t.Fatalf("expected 3 default services, got %d", len(svcs))
	}

	expected := map[string]bool{
		"auto-rebuild":    false,
		"session-monitor": false,
		"agentrace-serve": false,
	}
	for _, svc := range svcs {
		if _, ok := expected[svc.Name()]; !ok {
			t.Errorf("unexpected service: %s", svc.Name())
		}
		expected[svc.Name()] = true
	}
	for name, found := range expected {
		if !found {
			t.Errorf("missing service: %s", name)
		}
	}
}

func TestDaemonServiceFactory_Replaceable(t *testing.T) {
	testSvc := &testDaemonService{name: "test"}
	original := daemonServiceFactory
	defer func() { daemonServiceFactory = original }()

	daemonServiceFactory = func() []supervisor.Service {
		return []supervisor.Service{testSvc}
	}

	svcs := daemonServiceFactory()
	if len(svcs) != 1 || svcs[0].Name() != "test" {
		t.Error("factory override did not take effect")
	}
}
