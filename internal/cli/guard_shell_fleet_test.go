package cli

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestStrictFleetPreflightDeny_OffWithoutEnv(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT_STRICT", "")
	resetFleetStrictCacheForTest()
	if strictFleetPreflightDeny() != nil {
		t.Fatal("expected nil when strict mode off")
	}
}

func TestStrictFleetPreflightDeny_BlocksOnFailure(t *testing.T) {
	resetFleetStrictCacheForTest()
	t.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT_STRICT", "1")
	t.Setenv("FLEET_PREFLIGHT_STRICT_TTL_SEC", "0")
	old := fleetPreflightHTTPGet
	var calls int
	fleetPreflightHTTPGet = func(_ context.Context, _ *http.Client, _ string) (int, error) {
		calls++
		return 0, fmt.Errorf("refused")
	}
	defer func() { fleetPreflightHTTPGet = old }()

	d := strictFleetPreflightDeny()
	if d == nil || d.Permission != "deny" {
		t.Fatalf("expected deny, got %+v", d)
	}
	if calls < 1 {
		t.Fatal("expected HTTP probe")
	}
}

func TestStrictFleetPreflightDeny_CachesSuccess(t *testing.T) {
	resetFleetStrictCacheForTest()
	t.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT_STRICT", "1")
	t.Setenv("FLEET_PREFLIGHT_STRICT_TTL_SEC", "60")
	old := fleetPreflightHTTPGet
	var calls int
	fleetPreflightHTTPGet = func(_ context.Context, _ *http.Client, _ string) (int, error) {
		calls++
		return 200, nil
	}
	defer func() { fleetPreflightHTTPGet = old }()

	if strictFleetPreflightDeny() != nil {
		t.Fatal("expected allow when probes ok")
	}
	afterFirst := calls
	if afterFirst != 2 {
		t.Fatalf("expected 2 HTTP probes (drl+prom), got %d", afterFirst)
	}
	if strictFleetPreflightDeny() != nil {
		t.Fatal("expected cached allow")
	}
	if calls != afterFirst {
		t.Fatalf("expected TTL cache to skip probes, got %d want %d", calls, afterFirst)
	}
}

func TestRunFleetPreflightCmd_StrictExitCode(t *testing.T) {
	t.Setenv("FLEET_DRL_COMPOSE_DIR", "")
	t.Setenv("FLEET_DRL_HEALTH_URL", "http://127.0.0.1:9/healthz")
	t.Setenv("FLEET_PROM_HEALTH_URL", "http://127.0.0.1:9/ready")
	fleetPreflightStrict = true
	defer func() { fleetPreflightStrict = false }()
	if err := runFleetPreflight(nil, nil); err == nil {
		t.Fatal("expected error in strict mode when ports closed")
	}
}
