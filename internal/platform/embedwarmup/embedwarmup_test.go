package embedwarmup

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"object":"list","data":[{"object":"embedding"}]}`)
	}))
	defer srv.Close()

	p := NewProber(srv.URL+"/v1/embeddings", 5*time.Second)
	result := p.Probe()
	if !result.Healthy {
		t.Errorf("expected healthy, got error: %s", result.Error)
	}
	if result.Latency <= 0 {
		t.Error("latency should be positive")
	}
}

func TestProbeUnhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewProber(srv.URL+"/v1/embeddings", 5*time.Second)
	result := p.Probe()
	if result.Healthy {
		t.Error("expected unhealthy on 500")
	}
}

func TestProbeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprintf(w, `{"ok":true}`)
	}))
	defer srv.Close()

	p := NewProber(srv.URL+"/v1/embeddings", 50*time.Millisecond)
	result := p.Probe()
	if result.Healthy {
		t.Error("expected unhealthy on timeout")
	}
}

func TestProbeConnectionRefused(t *testing.T) {
	p := NewProber("http://127.0.0.1:1/v1/embeddings", 1*time.Second)
	result := p.Probe()
	if result.Healthy {
		t.Error("expected unhealthy on connection refused")
	}
}

func TestReportAggregatesHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"object":"list","data":[{"object":"embedding"}]}`)
	}))
	defer srv.Close()

	p := NewProber(srv.URL+"/v1/embeddings", 5*time.Second)
	p.Probe()
	p.Probe()
	p.Probe()

	report := p.Report()
	if report.TotalProbes != 3 {
		t.Errorf("total probes: %d", report.TotalProbes)
	}
	if report.SuccessCount != 3 {
		t.Errorf("success count: %d", report.SuccessCount)
	}
	if report.FailureCount != 0 {
		t.Errorf("failure count: %d", report.FailureCount)
	}
}

func TestReportMixedResults(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"ok":true}`)
	}))
	defer srv.Close()

	p := NewProber(srv.URL+"/v1/embeddings", 5*time.Second)
	p.Probe()
	p.Probe()
	p.Probe()

	report := p.Report()
	if report.TotalProbes != 3 {
		t.Errorf("total: %d", report.TotalProbes)
	}
	if report.SuccessCount != 2 {
		t.Errorf("success: %d", report.SuccessCount)
	}
	if report.FailureCount != 1 {
		t.Errorf("failure: %d", report.FailureCount)
	}
}

func TestAverageLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"ok":true}`)
	}))
	defer srv.Close()

	p := NewProber(srv.URL+"/v1/embeddings", 5*time.Second)
	p.Probe()
	p.Probe()

	report := p.Report()
	if report.AvgLatency <= 0 {
		t.Error("avg latency should be positive")
	}
}
