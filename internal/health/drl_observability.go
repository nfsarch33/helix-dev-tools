package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

// httpProbeGET performs a GET with ctx and returns HTTP status (0 if transport error).
func httpProbeGET(ctx context.Context, client *http.Client, url string) (status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func prometheusReadyURL(base string) string {
	b := strings.TrimSuffix(strings.TrimSpace(base), "/")
	if b == "" {
		b = "http://127.0.0.1:9099"
	}
	return b + "/-/ready"
}

func pushgatewayMetricsURL(base string) string {
	b := strings.TrimSuffix(strings.TrimSpace(base), "/")
	if b == "" {
		b = "http://127.0.0.1:9091"
	}
	return b + "/metrics"
}

func discoverDRLComposeDir(home string) (string, bool) {
	candidates := []string{
		filepath.Join(home, "ai-agent-business-stack", "docker"),
		filepath.Join(home, "Code", "ai-agent-business-stack", "docker"),
		filepath.Join(home, "repo", "biz-stack", "ai-agent-business-stack", "docker"),
	}
	for _, d := range candidates {
		if _, err := os.Stat(filepath.Join(d, "docker-compose.drl.yml")); err == nil {
			return d, true
		}
	}
	return "", false
}

// suiteDRLEvoLoopObservability probes Prometheus (/-/ready), DRL healthz, and Pushgateway /metrics.
// Default: soft pass when endpoints are down (dev machines without Docker).
// Set FLEET_DRL_DOCTOR_STRICT=1 to fail when any probe is not HTTP 2xx.
// Set FLEET_DRL_DOCTOR_SKIP=1 to skip probes (single Pass).
func suiteDRLEvoLoopObservability(p config.Paths) *Suite {
	s := &Suite{Name: "DRL EvoLoop Observability"}
	if os.Getenv("FLEET_DRL_DOCTOR_SKIP") == "1" {
		s.Pass("skipped (FLEET_DRL_DOCTOR_SKIP=1)")
		return s
	}

	strict := os.Getenv("FLEET_DRL_DOCTOR_STRICT") == "1"
	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}

	promBase := strings.TrimSpace(os.Getenv("PROMETHEUS_URL"))
	if promBase == "" {
		promBase = "http://127.0.0.1:9099"
	}
	drlURL := strings.TrimSpace(os.Getenv("FLEET_DRL_HEALTH_URL"))
	if drlURL == "" {
		drlURL = "http://127.0.0.1:8180/healthz"
	}
	pgBase := strings.TrimSpace(os.Getenv("PROM_PUSHGATEWAY_URL"))
	if pgBase == "" {
		// Align with prom-push / shell scripts using PROM_PUSHGATEWAY
		pgBase = strings.TrimSpace(os.Getenv("PROM_PUSHGATEWAY"))
	}
	if pgBase == "" {
		pgBase = "http://127.0.0.1:9091"
	}

	probe := func(name, url string) {
		st, err := httpProbeGET(ctx, client, url)
		ok := err == nil && st >= 200 && st < 300
		if strict {
			s.Assert(name, ok, fmt.Sprintf("GET %s -> status=%d err=%v", url, st, err))
			return
		}
		if ok {
			s.Pass(fmt.Sprintf("%s OK (%s)", name, url))
			return
		}
		s.Pass(fmt.Sprintf("%s — not reachable (optional); GET %s status=%d err=%v — set FLEET_DRL_DOCTOR_STRICT=1 to fail", name, url, st, err))
	}

	probe("Prometheus ready", prometheusReadyURL(promBase))
	probe("DRL service healthz", drlURL)
	probe("Pushgateway metrics", pushgatewayMetricsURL(pgBase))

	dir, found := discoverDRLComposeDir(p.Home)
	if found {
		s.Pass(fmt.Sprintf("docker-compose.drl.yml found under %s", dir))
	} else {
		msg := "clone ai-agent-business-stack or set FLEET_DRL_COMPOSE_DIR"
		if strict {
			s.Fail("docker-compose.drl.yml discoverable", msg)
		} else {
			s.Pass("docker-compose.drl.yml — not found (" + msg + ", optional)")
		}
	}

	return s
}
