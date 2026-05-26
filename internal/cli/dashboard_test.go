package cli

import "testing"

func TestDashboardCommand_HasServeChild(t *testing.T) {
	if dashboardCmd == nil {
		t.Fatal("dashboardCmd is nil")
	}
	if dashboardCmd.Use != "dashboard" {
		t.Errorf("dashboardCmd.Use = %q, want %q", dashboardCmd.Use, "dashboard")
	}

	serve, _, err := dashboardCmd.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("Find(serve) error: %v", err)
	}
	if serve == nil || serve.Use != "serve" {
		t.Fatalf("expected serve subcommand, got %v", serve)
	}
	for _, name := range []string{"port", "manifest", "engram-url"} {
		if serve.Flags().Lookup(name) == nil {
			t.Errorf("dashboard serve missing --%s flag", name)
		}
	}
}

func TestDashboardServeChild_DefaultPort(t *testing.T) {
	flag := dashboardServeChildCmd.Flags().Lookup("port")
	if flag == nil {
		t.Fatal("missing --port flag")
	}
	if flag.DefValue != "9095" {
		t.Errorf("port default = %q, want %q", flag.DefValue, "9095")
	}
}

func TestDashboardServeChild_AliasParity(t *testing.T) {
	if dashboardServeCmd.RunE == nil || dashboardServeChildCmd.RunE == nil {
		t.Fatal("RunE must be set on both forms")
	}
}
