package k3svalidator

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

const defaultDashboardPort = 30043

// DashboardProbe checks accessibility of the K8s Dashboard.
type DashboardProbe struct {
	Host string
	Port int
}

// NewDashboardProbe creates a probe for the given host and port.
// If port is 0, the default NodePort (30043) is used.
func NewDashboardProbe(host string, port int) *DashboardProbe {
	if port == 0 {
		port = defaultDashboardPort
	}
	return &DashboardProbe{Host: host, Port: port}
}

// URL returns the full HTTPS URL for the dashboard.
func (p *DashboardProbe) URL() string {
	return fmt.Sprintf("https://%s:%d", p.Host, p.Port)
}

// Check performs an HTTPS GET to the dashboard and returns nil if reachable.
func (p *DashboardProbe) Check(timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(p.URL())
	if err != nil {
		return fmt.Errorf("dashboard unreachable at %s: %w", p.URL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("dashboard returned %d at %s", resp.StatusCode, p.URL())
	}
	return nil
}
