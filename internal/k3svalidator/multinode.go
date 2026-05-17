package k3svalidator

import (
	"fmt"
	"strings"
)

// MultiNodeStatus aggregates multi-node cluster health.
type MultiNodeStatus struct {
	TotalNodes     int        `json:"total_nodes"`
	ReadyNodes     int        `json:"ready_nodes"`
	ControlPlanes  []string   `json:"control_planes"`
	Workers        []string   `json:"workers"`
	GPUNodes       []GPUNode  `json:"gpu_nodes"`
	Errors         []string   `json:"errors,omitempty"`
}

// GPUNode holds GPU label information for a node.
type GPUNode struct {
	Name       string `json:"name"`
	GPUProduct string `json:"gpu_product"`
	GPUCount   string `json:"gpu_count"`
}

// ValidateMultiNode checks that the cluster has at least minNodes nodes
// and at least one control-plane.
func ValidateMultiNode(nodes []K3sNode, minNodes int) error {
	if len(nodes) < minNodes {
		return fmt.Errorf("cluster has %d nodes, minimum required is %d", len(nodes), minNodes)
	}

	hasCP := false
	for _, n := range nodes {
		if strings.Contains(n.Roles, "control-plane") {
			hasCP = true
			break
		}
	}
	if !hasCP {
		return fmt.Errorf("no control-plane node found")
	}
	return nil
}

// ClassifyNodes separates nodes into control-planes, workers, and GPU-equipped.
func ClassifyNodes(nodes []K3sNode, labels map[string]map[string]string) *MultiNodeStatus {
	status := &MultiNodeStatus{
		TotalNodes: len(nodes),
	}

	for _, n := range nodes {
		if n.Status == "Ready" {
			status.ReadyNodes++
		}
		if strings.Contains(n.Roles, "control-plane") {
			status.ControlPlanes = append(status.ControlPlanes, n.Name)
		} else {
			status.Workers = append(status.Workers, n.Name)
		}

		if nodeLabels, ok := labels[n.Name]; ok {
			gpuProduct := nodeLabels["nvidia.com/gpu.product"]
			gpuCount := nodeLabels["nvidia.com/gpu.count"]
			if gpuProduct != "" || gpuCount != "" {
				status.GPUNodes = append(status.GPUNodes, GPUNode{
					Name:       n.Name,
					GPUProduct: gpuProduct,
					GPUCount:   gpuCount,
				})
			}
		}
	}
	return status
}

// ValidateGPULabels checks that expected GPU nodes have proper labels.
func ValidateGPULabels(labels map[string]map[string]string, expectedGPUNodes []string) error {
	var missing []string
	for _, name := range expectedGPUNodes {
		nodeLabels, ok := labels[name]
		if !ok {
			missing = append(missing, name+" (no labels)")
			continue
		}
		if nodeLabels["nvidia.com/gpu.product"] == "" {
			missing = append(missing, name+" (no gpu.product label)")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("GPU labels missing on nodes: %s", strings.Join(missing, ", "))
	}
	return nil
}

// SchedulingCheck represents a cross-node scheduling validation result.
type SchedulingCheck struct {
	CanScheduleGPU    bool   `json:"can_schedule_gpu"`
	CanScheduleNonGPU bool   `json:"can_schedule_non_gpu"`
	DNSResolution     bool   `json:"dns_resolution"`
	Error             string `json:"error,omitempty"`
}

// ValidateCrossNodeScheduling checks basic scheduling requirements.
func ValidateCrossNodeScheduling(nodes []K3sNode) *SchedulingCheck {
	check := &SchedulingCheck{}

	if len(nodes) < 2 {
		check.Error = "need at least 2 nodes for cross-node scheduling"
		return check
	}

	readyCount := 0
	for _, n := range nodes {
		if n.Status == "Ready" {
			readyCount++
		}
	}

	check.CanScheduleNonGPU = readyCount >= 2
	check.CanScheduleGPU = readyCount >= 1
	check.DNSResolution = readyCount >= 1

	return check
}
