package fleetk8s

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQA_NodeLifecycle_ReadyToNotReady(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("node-1", 1)),
		podsJSON:  makePodsJSON(runningPod("app-0", "default")),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")

	status1, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.True(t, status1.Healthy)

	runner.nodesJSON = makeNodesJSON(notReadyNode("node-1"))
	status2, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.False(t, status2.Healthy, "should detect node going NotReady")
}

func TestQA_DriftAlerting_NewNodeAppears(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("node-1", 1)),
		podsJSON:  makePodsJSON(),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")

	status1, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.Len(t, status1.NodeStatuses, 1)

	runner.nodesJSON = makeNodesJSON(readyNode("node-1", 1), readyNode("node-2", 2))
	status2, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.Len(t, status2.NodeStatuses, 2, "should detect new node joining cluster")
}

func TestQA_PrometheusMetricEmission_AllExpectedMetrics(t *testing.T) {
	status := &HealthStatus{
		Timestamp: time.Now(),
		Healthy:   true,
		NodeStatuses: []NodeHealthState{
			{Name: "gpu-host-1", Ready: true, GPUCount: 2, GPUAllocatable: 2, Roles: "worker"},
			{Name: "gpu-host-2", Ready: true, GPUCount: 1, GPUAllocatable: 1, Roles: "worker"},
		},
		PodAggregation: PodHealthSummary{
			TotalPods:   5,
			RunningPods: 4,
			FailedPods:  1,
			RestartCounts: map[string]int{
				"crashloop-0": 15,
			},
			ByNamespace: map[string]NamespacePodSummary{
				"llm-cluster": {Total: 3, Running: 2, Failed: 1},
				"monitoring":  {Total: 2, Running: 2},
			},
		},
		DaemonStatus: DaemonState{
			Running:       true,
			LastCheck:     time.Now(),
			CheckInterval: "60s",
			Version:       "v0.1.0",
		},
	}

	metrics := ExportPrometheus(status)
	output := metrics.String()

	requiredMetrics := []string{
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

	for _, m := range requiredMetrics {
		assert.Contains(t, output, m, "missing metric: %s", m)
	}

	assert.Contains(t, output, `fleet_pod_failed{namespace="llm-cluster"} 1`)
	assert.Contains(t, output, `fleet_pod_restart_count{pod="crashloop-0"} 15`)
}

func TestQA_DashboardPanelQueries_MetricLabels(t *testing.T) {
	status := &HealthStatus{
		Timestamp: time.Now(),
		Healthy:   true,
		NodeStatuses: []NodeHealthState{
			{Name: "gpu-host-1", Ready: true, GPUCount: 1, GPUAllocatable: 1, Roles: "control-plane,worker"},
		},
		PodAggregation: PodHealthSummary{
			TotalPods:     1,
			RunningPods:   1,
			RestartCounts: map[string]int{},
			ByNamespace: map[string]NamespacePodSummary{
				"monitoring": {Total: 1, Running: 1},
			},
		},
		DaemonStatus: DaemonState{
			Running:   true,
			LastCheck: time.Now(),
		},
	}

	metrics := ExportPrometheus(status)
	output := metrics.String()

	assert.Contains(t, output, `node="gpu-host-1"`, "node label should be present for dashboard filtering")
	assert.Contains(t, output, `roles="control-plane,worker"`, "roles label should be present")
	assert.Contains(t, output, `namespace="monitoring"`, "namespace label for per-ns dashboard panels")
}

func TestQA_MetricHELPandTYPE(t *testing.T) {
	status := healthyStatus()
	metrics := ExportPrometheus(status)

	helpCount := 0
	typeCount := 0
	for _, line := range metrics.Lines {
		if strings.HasPrefix(line, "# HELP ") {
			helpCount++
		}
		if strings.HasPrefix(line, "# TYPE ") {
			typeCount++
		}
	}

	assert.Equal(t, helpCount, typeCount, "every HELP should have a matching TYPE")
	assert.GreaterOrEqual(t, helpCount, 9, "should have at least 9 metric families")
}

func TestQA_EmptyCluster_NoNodes(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(),
		podsJSON:  makePodsJSON(),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.True(t, status.Healthy, "empty cluster should be considered healthy (no failures)")
	assert.Empty(t, status.NodeStatuses)

	metrics := ExportPrometheus(status)
	assert.NotEmpty(t, metrics.Lines, "should still emit daemon metrics even with no nodes")
	assert.Contains(t, metrics.String(), "fleet_daemon_up 1")
}

func TestQA_MultiGPUFleet_Aggregation(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(
			readyNode("rtx3090-node", 1),
			readyNode("rtx4070ti-node", 1),
			readyNode("cpu-only-node", 0),
		),
		podsJSON: makePodsJSON(
			runningPod("vllm-3090", "llm-cluster"),
			runningPod("vllm-4070ti", "llm-cluster"),
			runningPod("grafana-0", "monitoring"),
		),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)

	totalGPU := 0
	for _, ns := range status.NodeStatuses {
		totalGPU += ns.GPUCount
	}
	assert.Equal(t, 2, totalGPU, "should track 2 total GPUs across fleet")
	assert.Len(t, status.NodeStatuses, 3)
	assert.Equal(t, 0, status.NodeStatuses[2].GPUCount, "cpu-only node should have 0 GPUs")
}
