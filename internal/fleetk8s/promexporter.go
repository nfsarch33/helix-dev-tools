package fleetk8s

import (
	"fmt"
	"strings"
	"time"
)

// PrometheusMetrics holds fleet-health metrics in Prometheus exposition format.
type PrometheusMetrics struct {
	Lines     []string
	Timestamp time.Time
}

// ExportPrometheus converts a HealthStatus into Prometheus text exposition format.
func ExportPrometheus(status *HealthStatus) *PrometheusMetrics {
	m := &PrometheusMetrics{Timestamp: status.Timestamp}

	m.addHelp("fleet_node_up", "Whether a fleet node is ready (1) or not (0)")
	m.addType("fleet_node_up", "gauge")
	for _, ns := range status.NodeStatuses {
		val := 0
		if ns.Ready {
			val = 1
		}
		m.addMetric("fleet_node_up", val, "node", ns.Name, "roles", ns.Roles)
	}

	m.addHelp("fleet_node_gpu_count", "Total GPU count per node")
	m.addType("fleet_node_gpu_count", "gauge")
	for _, ns := range status.NodeStatuses {
		m.addMetric("fleet_node_gpu_count", ns.GPUCount, "node", ns.Name)
	}

	m.addHelp("fleet_node_gpu_allocatable", "Allocatable GPU count per node")
	m.addType("fleet_node_gpu_allocatable", "gauge")
	for _, ns := range status.NodeStatuses {
		m.addMetric("fleet_node_gpu_allocatable", ns.GPUAllocatable, "node", ns.Name)
	}

	m.addHelp("fleet_pod_total", "Total pods per namespace")
	m.addType("fleet_pod_total", "gauge")
	for ns, summary := range status.PodAggregation.ByNamespace {
		m.addMetric("fleet_pod_total", summary.Total, "namespace", ns)
	}

	m.addHelp("fleet_pod_running", "Running pods per namespace")
	m.addType("fleet_pod_running", "gauge")
	for ns, summary := range status.PodAggregation.ByNamespace {
		m.addMetric("fleet_pod_running", summary.Running, "namespace", ns)
	}

	m.addHelp("fleet_pod_failed", "Failed pods per namespace")
	m.addType("fleet_pod_failed", "gauge")
	for ns, summary := range status.PodAggregation.ByNamespace {
		m.addMetric("fleet_pod_failed", summary.Failed, "namespace", ns)
	}

	m.addHelp("fleet_pod_restart_count", "Pod restart count per pod")
	m.addType("fleet_pod_restart_count", "counter")
	for pod, count := range status.PodAggregation.RestartCounts {
		m.addMetric("fleet_pod_restart_count", count, "pod", pod)
	}

	m.addHelp("fleet_daemon_last_check", "Unix timestamp of last health check")
	m.addType("fleet_daemon_last_check", "gauge")
	m.addMetric("fleet_daemon_last_check", int(status.DaemonStatus.LastCheck.Unix()))

	m.addHelp("fleet_daemon_up", "Whether the fleet-health daemon is running")
	m.addType("fleet_daemon_up", "gauge")
	daemonUp := 0
	if status.DaemonStatus.Running {
		daemonUp = 1
	}
	m.addMetric("fleet_daemon_up", daemonUp)

	return m
}

// String returns the full metrics output as a string.
func (m *PrometheusMetrics) String() string {
	return strings.Join(m.Lines, "\n") + "\n"
}

// MetricNames returns the unique metric names exported.
func (m *PrometheusMetrics) MetricNames() []string {
	var names []string
	seen := make(map[string]bool)
	for _, line := range m.Lines {
		if strings.HasPrefix(line, "# TYPE ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 && !seen[parts[2]] {
				names = append(names, parts[2])
				seen[parts[2]] = true
			}
		}
	}
	return names
}

func (m *PrometheusMetrics) addHelp(name, help string) {
	m.Lines = append(m.Lines, fmt.Sprintf("# HELP %s %s", name, help))
}

func (m *PrometheusMetrics) addType(name, typ string) {
	m.Lines = append(m.Lines, fmt.Sprintf("# TYPE %s %s", name, typ))
}

func (m *PrometheusMetrics) addMetric(name string, value int, labels ...string) {
	if len(labels) == 0 {
		m.Lines = append(m.Lines, fmt.Sprintf("%s %d", name, value))
		return
	}
	var labelPairs []string
	for i := 0; i+1 < len(labels); i += 2 {
		labelPairs = append(labelPairs, fmt.Sprintf(`%s="%s"`, labels[i], labels[i+1]))
	}
	m.Lines = append(m.Lines, fmt.Sprintf("%s{%s} %d", name, strings.Join(labelPairs, ","), value))
}
