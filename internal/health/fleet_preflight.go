package health

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultFleetDRLHealthURL  = "http://127.0.0.1:8180/healthz"
	DefaultFleetPromHealthURL = "http://127.0.0.1:9099/-/healthy"
	defaultFleetHTTPTimeout   = 4 * time.Second
	defaultFleetProbeDeadline = 10 * time.Second
)

// FleetPreflightProbe is one HTTP probe result.
type FleetPreflightProbe struct {
	Label  string
	URL    string
	Status int
	Err    error
}

// FleetPreflightResult aggregates probes and optional compose check.
type FleetPreflightResult struct {
	Probes      []FleetPreflightProbe
	ComposeOK   bool
	ComposeErr  string
	ComposePath string
	AnyFailed   bool
	ComposeRan  bool
}

// OK is true when every HTTP probe returned 2xx and compose (if run) succeeded.
func (r FleetPreflightResult) OK() bool {
	if r.AnyFailed {
		return false
	}
	for _, p := range r.Probes {
		if p.Err != nil || p.Status < 200 || p.Status > 299 {
			return false
		}
	}
	if r.ComposeRan && !r.ComposeOK {
		return false
	}
	return true
}

// FleetPreflightOptions configures RunFleetPreflight.
type FleetPreflightOptions struct {
	DRLHealthURL  string
	PromHealthURL string
	ComposeDir    string
	ComposeRel    string
	HTTPClient    *http.Client
	HTTPGet       func(ctx context.Context, client *http.Client, url string) (int, error)
	ComposePS     func(ctx context.Context, composeFile string) (string, error)
}

// RunFleetPreflight runs DRL + Prometheus HTTP probes and optional docker compose ps.
func RunFleetPreflight(ctx context.Context, opts FleetPreflightOptions) FleetPreflightResult {
	drl := strings.TrimSpace(opts.DRLHealthURL)
	if drl == "" {
		drl = DefaultFleetDRLHealthURL
	}
	prom := strings.TrimSpace(opts.PromHealthURL)
	if prom == "" {
		prom = DefaultFleetPromHealthURL
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultFleetHTTPTimeout}
	}
	httpGet := opts.HTTPGet
	if httpGet == nil {
		httpGet = func(c context.Context, cl *http.Client, u string) (int, error) {
			return httpProbeGET(c, cl, u)
		}
	}

	deadlineCtx, cancel := context.WithTimeout(ctx, defaultFleetProbeDeadline)
	defer cancel()

	var out FleetPreflightResult
	for _, probe := range []struct {
		label string
		url   string
	}{
		{"drl-service", drl},
		{"prometheus", prom},
	} {
		code, err := httpGet(deadlineCtx, client, probe.url)
		p := FleetPreflightProbe{Label: probe.label, URL: probe.url, Status: code, Err: err}
		out.Probes = append(out.Probes, p)
		if err != nil || code < 200 || code > 299 {
			out.AnyFailed = true
		}
	}

	composeDir := strings.TrimSpace(opts.ComposeDir)
	if composeDir == "" {
		return out
	}
	rel := strings.TrimSpace(opts.ComposeRel)
	if rel == "" {
		rel = "docker/docker-compose.drl.yml"
	}
	composePath := filepath.Join(composeDir, rel)
	out.ComposePath = composePath

	composeRun := opts.ComposePS
	if composeRun == nil {
		composeRun = defaultComposePS
	}
	out.ComposeRan = true
	outStr, err := composeRun(deadlineCtx, composePath)
	if err != nil {
		out.ComposeOK = false
		out.ComposeErr = err.Error()
		out.AnyFailed = true
		return out
	}
	_ = outStr
	out.ComposeOK = true
	return out
}

func defaultComposePS(ctx context.Context, composeFile string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "ps")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// ErrFleetPreflightFailed is returned from strict callers when result is not OK.
var ErrFleetPreflightFailed = fmt.Errorf("fleet preflight: one or more checks failed")
