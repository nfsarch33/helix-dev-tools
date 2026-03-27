package cli

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/health"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
)

var (
	fleetStrictMu sync.Mutex
	fleetStrictAt time.Time
	fleetStrictOK bool
)

func fleetStrictTTL() time.Duration {
	s := strings.TrimSpace(os.Getenv("FLEET_PREFLIGHT_STRICT_TTL_SEC"))
	if s == "" {
		return 25 * time.Second
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 25 * time.Second
	}
	return time.Duration(n) * time.Second
}

// resetFleetStrictCacheForTest clears the in-process TTL cache (tests only).
func resetFleetStrictCacheForTest() {
	fleetStrictMu.Lock()
	defer fleetStrictMu.Unlock()
	fleetStrictAt = time.Time{}
	fleetStrictOK = false
}

// strictFleetPreflightDeny returns a Deny response when CURSOR_TOOLS_FLEET_PREFLIGHT_STRICT=1
// and DRL/Prometheus (and optional compose) checks fail. Uses a TTL cache to limit HTTP calls.
func strictFleetPreflightDeny() *hookio.Response {
	if strings.TrimSpace(os.Getenv("CURSOR_TOOLS_FLEET_PREFLIGHT_STRICT")) != "1" {
		return nil
	}
	ttl := fleetStrictTTL()
	now := time.Now()
	fleetStrictMu.Lock()
	defer fleetStrictMu.Unlock()
	if ttl > 0 && !fleetStrictAt.IsZero() && now.Sub(fleetStrictAt) < ttl {
		if fleetStrictOK {
			return nil
		}
		return fleetStrictDenyMessage()
	}
	ctx := context.Background()
	opts := health.FleetPreflightOptions{
		DRLHealthURL:  os.Getenv("FLEET_DRL_HEALTH_URL"),
		PromHealthURL: os.Getenv("FLEET_PROM_HEALTH_URL"),
		HTTPGet: func(ctx context.Context, c *http.Client, u string) (int, error) {
			return fleetPreflightHTTPGet(ctx, c, u)
		},
	}
	if cd := strings.TrimSpace(os.Getenv("FLEET_DRL_COMPOSE_DIR")); cd != "" {
		opts.ComposeDir = cd
	}
	res := health.RunFleetPreflight(ctx, opts)
	fleetStrictAt = now
	fleetStrictOK = res.OK()
	if res.OK() {
		return nil
	}
	return fleetStrictDenyMessage()
}

func fleetStrictDenyMessage() *hookio.Response {
	return hookio.Deny(
		"BLOCKED: fleet DRL/Prometheus preflight failed (CURSOR_TOOLS_FLEET_PREFLIGHT_STRICT=1).",
		"Bring up the stack: make -C ~/Code/global-kb fleet-drl-compose-up FLEET_DRL_COMPOSE_DIR=<ai-agent-business-stack>. See global-kb sop/drl-evolver-metrics-operators.md section 2.3.",
	)
}
