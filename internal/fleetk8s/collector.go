package fleetk8s

import "fmt"

// ClusterStatus is the top-level output of fleet k8s-status.
type ClusterStatus struct {
	Nodes   []NodeInfo     `json:"nodes"`
	Pods    []PodInfo      `json:"pods"`
	Summary ClusterSummary `json:"summary"`
}

// ClusterSummary holds aggregate counts.
type ClusterSummary struct {
	TotalNodes      int `json:"total_nodes"`
	ReadyNodes      int `json:"ready_nodes"`
	TotalGPUs       int `json:"total_gpus"`
	AllocatableGPUs int `json:"allocatable_gpus"`
	TotalPods       int `json:"total_pods"`
	RunningPods     int `json:"running_pods"`
	PendingPods     int `json:"pending_pods"`
	FailedPods      int `json:"failed_pods"`
}

// Collector gathers cluster status from a KubectlRunner.
type Collector struct {
	runner KubectlRunner
}

// NewCollector creates a Collector backed by the given runner.
func NewCollector(runner KubectlRunner) *Collector {
	return &Collector{runner: runner}
}

// Collect fetches node and pod data and computes the summary.
func (c *Collector) Collect() (*ClusterStatus, error) {
	nodesRaw, err := c.runner.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("collect nodes: %w", err)
	}
	nodes, err := ParseNodesJSON(nodesRaw)
	if err != nil {
		return nil, err
	}

	podsRaw, err := c.runner.GetPods()
	if err != nil {
		return nil, fmt.Errorf("collect pods: %w", err)
	}
	pods, err := ParsePodsJSON(podsRaw)
	if err != nil {
		return nil, err
	}

	summary := computeSummary(nodes, pods)
	return &ClusterStatus{
		Nodes:   nodes,
		Pods:    pods,
		Summary: summary,
	}, nil
}

func computeSummary(nodes []NodeInfo, pods []PodInfo) ClusterSummary {
	s := ClusterSummary{
		TotalNodes: len(nodes),
		TotalPods:  len(pods),
	}
	for _, n := range nodes {
		if n.Status == "Ready" {
			s.ReadyNodes++
		}
		s.TotalGPUs += n.GPUCount
		s.AllocatableGPUs += n.GPUAllocatable
	}
	for _, p := range pods {
		switch p.Phase {
		case "Running":
			s.RunningPods++
		case "Pending":
			s.PendingPods++
		case "Failed":
			s.FailedPods++
		}
	}
	return s
}
