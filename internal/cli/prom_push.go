// runx-public-repo-gate: allow-file internal_service_id — prom-push integrates with the upstream open-source Prometheus Pushgateway component, named explicitly by convention

package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
	"github.com/nfsarch33/cursor-tools/internal/prombridge"
)

var promPushFlags struct {
	pushgateway  string
	job          string
	instance     string
	days         int
	withSmoke    bool
	promURL      string
	drlHealthURL string
	dryRun       bool
	timeout      time.Duration
}

var promPushCmd = &cobra.Command{
	Use:   "prom-push",
	Short: "Push metrics.jsonl rollups (+ optional EvoLoop smoke) to Prometheus Pushgateway",
	Long: `Aggregates ~/.cursor/hooks/metrics.jsonl for the last N days and POSTs Prometheus
exposition text to Pushgateway (scraped by DRL Prometheus). Optionally probes DRL Prometheus
and drl-service health and pushes ironclaw_evoloop_smoke_* gauges.

Example:
  cursor-tools prom-push --with-smoke
  cursor-tools prom-push --dry-run
  PROM_PUSHGATEWAY=http://127.0.0.1:9091 cursor-tools prom-push`,
	RunE: runPromPush,
}

func init() {
	promPushCmd.Flags().StringVar(&promPushFlags.pushgateway, "pushgateway", envOr("PROM_PUSHGATEWAY", "http://127.0.0.1:9091"), "Pushgateway base URL")
	promPushCmd.Flags().StringVar(&promPushFlags.job, "job", envOr("PROM_PUSH_JOB", "cursor-hooks"), "Pushgateway job label")
	h, _ := os.Hostname()
	if strings.TrimSpace(h) == "" {
		h = "unknown"
	}
	promPushCmd.Flags().StringVar(&promPushFlags.instance, "instance", envOr("PROM_PUSH_INSTANCE", h), "Pushgateway instance label")
	promPushCmd.Flags().IntVar(&promPushFlags.days, "days", 1, "Rollup window in days for metrics.jsonl")
	promPushCmd.Flags().BoolVar(&promPushFlags.withSmoke, "with-smoke", false, "Probe DRL Prometheus + drl-service and push smoke gauges")
	promPushCmd.Flags().StringVar(&promPushFlags.promURL, "prometheus-url", envOr("PROMETHEUS_URL", "http://127.0.0.1:9099"), "Prometheus /-/healthy URL (path appended if missing)")
	promPushCmd.Flags().StringVar(&promPushFlags.drlHealthURL, "drl-health-url", envOr("FLEET_DRL_HEALTH_URL", "http://127.0.0.1:8180/healthz"), "drl-service health URL")
	promPushCmd.Flags().BoolVar(&promPushFlags.dryRun, "dry-run", false, "Print exposition to stdout; do not POST")
	promPushCmd.Flags().DurationVar(&promPushFlags.timeout, "timeout", 8*time.Second, "HTTP client timeout for push and probes")
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func runPromPush(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	metricsPath := p.MetricsFile()

	events, err := metrics.LoadAll(metricsPath)
	if err != nil {
		return fmt.Errorf("loading metrics: %w", err)
	}

	since := time.Now().UTC().Add(-time.Duration(promPushFlags.days) * 24 * time.Hour)
	summary := metrics.Summarise(events, since)
	window := fmt.Sprintf("%dh", promPushFlags.days*24)

	var smoke *prombridge.EvoloopSmoke
	if promPushFlags.withSmoke {
		smoke = probeEvoloopSmoke(promPushFlags.promURL, promPushFlags.drlHealthURL, promPushFlags.timeout)
	}

	body := prombridge.Format(summary, window, smoke)

	if promPushFlags.dryRun {
		fmt.Print(body)
		return nil
	}

	pushURL, err := pushgatewayURL(promPushFlags.pushgateway, promPushFlags.job, promPushFlags.instance)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: promPushFlags.timeout}
	req, err := http.NewRequest(http.MethodPost, pushURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; version=0.0.4")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushgateway POST: %w", err)
	}
	defer resp.Body.Close()
	slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pushgateway %s: %s", resp.Status, strings.TrimSpace(string(slurp)))
	}
	fmt.Printf("prom-push: pushed to %s (%d bytes)\n", pushURL, len(body))
	return nil
}

func pushgatewayURL(base, job, instance string) (string, error) {
	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	if base == "" {
		return "", fmt.Errorf("empty pushgateway URL")
	}
	if _, err := url.Parse(base); err != nil {
		return "", fmt.Errorf("pushgateway URL: %w", err)
	}
	// Pushgateway push path: /metrics/job/<job>/instance/<instance>
	suffix := "/metrics/job/" + url.PathEscape(job) + "/instance/" + url.PathEscape(instance)
	return base + suffix, nil
}

func probeEvoloopSmoke(promBase, drlURL string, timeout time.Duration) *prombridge.EvoloopSmoke {
	client := &http.Client{Timeout: timeout}
	promHealth := strings.TrimRight(strings.TrimSpace(promBase), "/") + "/-/healthy"
	if strings.HasSuffix(strings.TrimSpace(promBase), "/-/healthy") {
		promHealth = strings.TrimSpace(promBase)
	}
	out := &prombridge.EvoloopSmoke{
		CheckedAt: time.Now().UTC(),
	}
	if req, err := http.NewRequest(http.MethodGet, promHealth, nil); err == nil {
		resp, err := client.Do(req)
		out.PrometheusHealthy = err == nil && resp != nil && resp.StatusCode == 200
		if resp != nil {
			resp.Body.Close()
		}
	}
	if req, err := http.NewRequest(http.MethodGet, strings.TrimSpace(drlURL), nil); err == nil {
		resp, err := client.Do(req)
		out.DRLServiceHealthy = err == nil && resp != nil && resp.StatusCode == 200
		if resp != nil {
			resp.Body.Close()
		}
	}
	return out
}
