package ansiblevalidator_test

import (
	"os"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/ansiblevalidator"
)

func TestSSHMatrixBuilder_DefaultFleet(t *testing.T) {
	matrix := ansiblevalidator.NewDefaultSSHMatrix()
	if matrix == nil {
		t.Fatal("NewDefaultSSHMatrix returned nil")
	}
	if len(matrix.Routes) == 0 {
		t.Fatal("expected non-empty routes in default matrix")
	}
}

func TestSSHMatrixBuilder_AddRoute(t *testing.T) {
	matrix := ansiblevalidator.NewSSHMatrix()
	matrix.AddRoute("test-host", ansiblevalidator.SSHRoute{
		Alias:       "test-host",
		Description: "test route",
		Port:        22,
	})

	if _, ok := matrix.Routes["test-host"]; !ok {
		t.Fatal("expected test-host in routes")
	}
}

func TestSSHMatrixBuilder_AddRouteWithFallback(t *testing.T) {
	matrix := ansiblevalidator.NewSSHMatrix()
	fallback := &ansiblevalidator.SSHRoute{
		Alias:       "test-host-wslexe",
		Description: "wsl.exe fallback",
	}
	matrix.AddRoute("test-host", ansiblevalidator.SSHRoute{
		Alias:       "test-host-direct",
		Description: "direct route",
		Port:        22,
		Fallback:    fallback,
	})

	route := matrix.Routes["test-host"]
	if route.Fallback == nil {
		t.Fatal("expected fallback route")
	}
	if route.Fallback.Alias != "test-host-wslexe" {
		t.Errorf("expected wslexe fallback, got %s", route.Fallback.Alias)
	}
}

func TestSSHMatrixValidate_MissingRequiredRoutes(t *testing.T) {
	matrix := ansiblevalidator.NewSSHMatrix()
	matrix.AddRoute("host-a", ansiblevalidator.SSHRoute{Alias: "host-a"})

	result := ansiblevalidator.ValidateSSHMatrix(matrix, []string{"host-a", "host-b"})
	if result.Valid {
		t.Fatal("expected invalid when host-b is missing")
	}
}

func TestSSHMatrixValidate_AllRoutesPresent(t *testing.T) {
	matrix := ansiblevalidator.NewSSHMatrix()
	matrix.AddRoute("host-a", ansiblevalidator.SSHRoute{Alias: "host-a"})
	matrix.AddRoute("host-b", ansiblevalidator.SSHRoute{Alias: "host-b"})

	result := ansiblevalidator.ValidateSSHMatrix(matrix, []string{"host-a", "host-b"})
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
}

func TestWslExeFallback_Unit(t *testing.T) {
	result := ansiblevalidator.SimulateWslExeFallback("test-host", "Ubuntu", "hostname")
	if result.HostAlias != "test-host" {
		t.Errorf("expected alias test-host, got %s", result.HostAlias)
	}
	if result.WslDistro != "Ubuntu" {
		t.Errorf("expected distro Ubuntu, got %s", result.WslDistro)
	}
}

func TestIntegration_SSHMatrixProbe(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run SSH matrix probe")
	}

	matrix := ansiblevalidator.NewDefaultSSHMatrix()
	probeResult := ansiblevalidator.ProbeSSHMatrix(matrix)
	t.Logf("SSH matrix probe: %d/%d reachable", probeResult.Reachable, probeResult.Total)

	if probeResult.Total == 0 {
		t.Fatal("expected at least one route in the matrix")
	}
}

func TestIntegration_WslExeFallback(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run wsl.exe fallback")
	}

	winAlias := os.Getenv("CURSOR_TOOLS_WIN2_ALIAS")
	if winAlias == "" {
		t.Skip("set CURSOR_TOOLS_WIN2_ALIAS for win2 wsl.exe fallback test")
	}

	result := ansiblevalidator.WslExeFallbackProbe(winAlias, "Ubuntu")
	t.Logf("wsl.exe fallback result: reachable=%v output=%q error=%s",
		result.Reachable, result.Output, result.Error)
}
