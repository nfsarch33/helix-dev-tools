package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

func TestPrometheusReadyURL(t *testing.T) {
	if u := prometheusReadyURL("http://127.0.0.1:9099"); u != "http://127.0.0.1:9099/-/ready" {
		t.Fatalf("got %q", u)
	}
	if u := prometheusReadyURL("http://127.0.0.1:9099/"); u != "http://127.0.0.1:9099/-/ready" {
		t.Fatalf("got %q", u)
	}
}

func TestHTTPProbeGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	ctx := context.Background()
	client := &http.Client{}
	st, err := httpProbeGET(ctx, client, srv.URL)
	if err != nil || st != 200 {
		t.Fatalf("status=%d err=%v", st, err)
	}
}

func TestSuiteDRLEvoLoopObservability_Skip(t *testing.T) {
	t.Setenv("FLEET_DRL_DOCTOR_SKIP", "1")
	t.Setenv("FLEET_DRL_DOCTOR_STRICT", "")
	p := config.DefaultPaths()
	s := suiteDRLEvoLoopObservability(p)
	if len(s.Results) != 1 || !s.Results[0].Passed {
		t.Fatalf("expected single pass, got %+v", s.Results)
	}
}

func TestSuiteDRLEvoLoopObservability_Strict_AllOK(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/-/ready" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	drl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	pg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer prom.Close()
	defer drl.Close()
	defer pg.Close()

	t.Setenv("FLEET_DRL_DOCTOR_STRICT", "1")
	t.Setenv("FLEET_DRL_DOCTOR_SKIP", "")
	t.Setenv("PROMETHEUS_URL", prom.URL)
	t.Setenv("FLEET_DRL_HEALTH_URL", drl.URL+"/healthz")
	t.Setenv("PROM_PUSHGATEWAY_URL", pg.URL)

	td := t.TempDir()
	composeDir := filepath.Join(td, "ai-agent-business-stack", "docker")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(composeDir, "docker-compose.drl.yml"), []byte("version: '3'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := config.Paths{Home: td}
	s := suiteDRLEvoLoopObservability(p)
	for _, r := range s.Results {
		if !r.Passed {
			t.Errorf("fail: %s — %s", r.Name, r.Detail)
		}
	}
}
