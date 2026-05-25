package fleetk8s

import (
	"fmt"
	"time"
)

// HealthStatus represents the fleet-health daemon's aggregated view.
type HealthStatus struct {
	Timestamp       time.Time         `json:"timestamp"`
	Healthy         bool              `json:"healthy"`
	NodeStatuses    []NodeHealthState `json:"node_statuses"`
	PodAggregation  PodHealthSummary  `json:"pod_aggregation"`
	DaemonStatus    DaemonState       `json:"daemon_status"`
}

// NodeHealthState tracks lifecycle state for a single node.
type NodeHealthState struct {
	Name           string `json:"name"`
	Ready          bool   `json:"ready"`
	GPUCount       int    `json:"gpu_count"`
	GPUAllocatable int    `json:"gpu_allocatable"`
	Roles          string `json:"roles"`
}

// PodHealthSummary aggregates pod health per namespace.
type PodHealthSummary struct {
	TotalPods      int                       `json:"total_pods"`
	RunningPods    int                       `json:"running_pods"`
	FailedPods     int                       `json:"failed_pods"`
	RestartCounts  map[string]int            `json:"restart_counts"`
	ByNamespace    map[string]NamespacePodSummary `json:"by_namespace"`
}

// NamespacePodSummary holds per-namespace pod counts.
type NamespacePodSummary struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
}

// DaemonState holds fleet-health daemon metadata.
type DaemonState struct {
	Running       bool      `json:"running"`
	LastCheck     time.Time `json:"last_check"`
	CheckInterval string    `json:"check_interval"`
	Version       string    `json:"version"`
}

// HealthAdapter bridges KubectlRunner to fleet-health status.
type HealthAdapter struct {
	runner  KubectlRunner
	version string
}

// NewHealthAdapter creates a HealthAdapter.
func NewHealthAdapter(runner KubectlRunner, version string) *HealthAdapter {
	return &HealthAdapter{runner: runner, version: version}
}

// CheckHealth performs a full cluster health check.
func (a *HealthAdapter) CheckHealth() (*HealthStatus, error) {
	nodesRaw, err := a.runner.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("health check nodes: %w", err)
	}
	nodes, err := ParseNodesJSON(nodesRaw)
	if err != nil {
		return nil, fmt.Errorf("parse nodes: %w", err)
	}

	podsRaw, err := a.runner.GetPods()
	if err != nil {
		return nil, fmt.Errorf("health check pods: %w", err)
	}
	pods, err := ParsePodsJSON(podsRaw)
	if err != nil {
		return nil, fmt.Errorf("parse pods: %w", err)
	}

	now := time.Now()
	status := &HealthStatus{
		Timestamp:      now,
		NodeStatuses:   buildNodeHealthStates(nodes),
		PodAggregation: buildPodHealthSummary(pods),
		DaemonStatus: DaemonState{
			Running:       true,
			LastCheck:     now,
			CheckInterval: "60s",
			Version:       a.version,
		},
	}

	status.Healthy = isClusterHealthy(status)
	return status, nil
}

func buildNodeHealthStates(nodes []NodeInfo) []NodeHealthState {
	states := make([]NodeHealthState, len(nodes))
	for i, n := range nodes {
		states[i] = NodeHealthState{
			Name:           n.Name,
			Ready:          n.Status == "Ready",
			GPUCount:       n.GPUCount,
			GPUAllocatable: n.GPUAllocatable,
			Roles:          n.Roles,
		}
	}
	return states
}

func buildPodHealthSummary(pods []PodInfo) PodHealthSummary {
	summary := PodHealthSummary{
		RestartCounts: make(map[string]int),
		ByNamespace:   make(map[string]NamespacePodSummary),
	}
	for _, p := range pods {
		summary.TotalPods++
		ns := summary.ByNamespace[p.Namespace]
		ns.Total++
		switch p.Phase {
		case "Running":
			summary.RunningPods++
			ns.Running++
		case "Failed":
			summary.FailedPods++
			ns.Failed++
		case "Pending":
			ns.Pending++
		}
		summary.ByNamespace[p.Namespace] = ns
	}
	return summary
}

func isClusterHealthy(status *HealthStatus) bool {
	for _, ns := range status.NodeStatuses {
		if !ns.Ready {
			return false
		}
	}
	return status.PodAggregation.FailedPods <= 0
}
