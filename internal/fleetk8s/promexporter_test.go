package fleetk8s

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func healthyStatus() *HealthStatus {
	return &HealthStatus{
		Timestamp: time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
		Healthy:   true,
		NodeStatuses: []NodeHealthState{
			{Name: "node-1", Ready: true, GPUCount: 1, GPUAllocatable: 1, Roles: "worker"},
			{Name: "node-2", Ready: true, GPUCount: 2, GPUAllocatable: 2, Roles: "worker"},
		},
		PodAggregation: PodHealthSummary{
			TotalPods:   3,
			RunningPods: 3,
			FailedPods:  0,
			RestartCounts: map[string]int{
				"vllm-0": 2,
			},
			ByNamespace: map[string]NamespacePodSummary{
				"llm-cluster": {Total: 2, Running: 2},
				"mem0":        {Total: 1, Running: 1},
			},
		},
		DaemonStatus: DaemonState{
			Running:       true,
			LastCheck:     time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC),
			CheckInterval: "60s",
			Version:       "v0.1.0",
		},
	}
}

func TestExportPrometheus_NodeUpMetric(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, `fleet_node_up{node="node-1",roles="worker"} 1`)
	assert.Contains(t, output, `fleet_node_up{node="node-2",roles="worker"} 1`)
}

func TestExportPrometheus_NodeDown(t *testing.T) {
	status := healthyStatus()
	status.NodeStatuses[1].Ready = false
	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, `fleet_node_up{node="node-2",roles="worker"} 0`)
}

func TestExportPrometheus_PodRestartCount(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, `fleet_pod_restart_count{pod="vllm-0"} 2`)
}

func TestExportPrometheus_DaemonLastCheck(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, "fleet_daemon_last_check ")
	assert.Contains(t, output, "fleet_daemon_up 1")
}

func TestExportPrometheus_DaemonDown(t *testing.T) {
	status := healthyStatus()
	status.DaemonStatus.Running = false
	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, "fleet_daemon_up 0")
}

func TestExportPrometheus_MetricNames(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)
	names := metrics.MetricNames()

	expected := []string{
		"fleet_node_up",
		"fleet_node_gpu_count",
		"fleet_node_gpu_allocatable",
		"fleet_pod_total",
		"fleet_pod_running",
		"fleet_pod_failed",
		"fleet_pod_restart_count",
		"fleet_daemon_last_check",
		"fleet_daemon_up",
	}
	for _, e := range expected {
		assert.Contains(t, names, e, "missing metric: %s", e)
	}
}

func TestExportPrometheus_ValidExpositionFormat(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)

	for _, line := range metrics.Lines {
		if strings.HasPrefix(line, "#") {
			require.True(t,
				strings.HasPrefix(line, "# HELP ") || strings.HasPrefix(line, "# TYPE "),
				"invalid comment line: %q", line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		require.GreaterOrEqual(t, len(parts), 2, "metric line must have name and value: %q", line)
	}
}

func TestExportPrometheus_GPUMetrics(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, `fleet_node_gpu_count{node="node-1"} 1`)
	assert.Contains(t, output, `fleet_node_gpu_count{node="node-2"} 2`)
	assert.Contains(t, output, `fleet_node_gpu_allocatable{node="node-1"} 1`)
	assert.Contains(t, output, `fleet_node_gpu_allocatable{node="node-2"} 2`)
}

func TestExportPrometheus_NamespacePodMetrics(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, `fleet_pod_total{namespace="llm-cluster"} 2`)
	assert.Contains(t, output, `fleet_pod_running{namespace="llm-cluster"} 2`)
	assert.Contains(t, output, `fleet_pod_total{namespace="mem0"} 1`)
}
