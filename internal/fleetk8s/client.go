package fleetk8s

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// KubectlRunner abstracts kubectl execution for testability.
type KubectlRunner interface {
	GetNodes() ([]byte, error)
	GetPods() ([]byte, error)
}

// DefaultRunner shells out to kubectl via os/exec.
type DefaultRunner struct {
	Kubeconfig string
}

func (r *DefaultRunner) kubectlArgs(resource string) []string {
	args := []string{"get", resource, "-o", "json"}
	if r.Kubeconfig != "" {
		args = append([]string{"--kubeconfig", r.Kubeconfig}, args...)
	}
	return args
}

func (r *DefaultRunner) GetNodes() ([]byte, error) {
	out, err := exec.Command("kubectl", r.kubectlArgs("nodes")...).Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get nodes: %w", err)
	}
	return out, nil
}

func (r *DefaultRunner) GetPods() ([]byte, error) {
	args := r.kubectlArgs("pods")
	args = append(args, "--all-namespaces")
	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get pods: %w", err)
	}
	return out, nil
}

// NodeInfo holds parsed node status.
type NodeInfo struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Roles          string `json:"roles"`
	GPUCount       int    `json:"gpu_count"`
	GPUAllocatable int    `json:"gpu_allocatable"`
}

// PodInfo holds parsed pod status.
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Phase     string `json:"phase"`
}

type k8sNodeList struct {
	Items []k8sNode `json:"items"`
}

type k8sNode struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		Conditions  []k8sCondition    `json:"conditions"`
		Capacity    map[string]string `json:"capacity"`
		Allocatable map[string]string `json:"allocatable"`
	} `json:"status"`
}

type k8sCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type k8sPodList struct {
	Items []k8sPod `json:"items"`
}

type k8sPod struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

// ParseNodesJSON parses kubectl get nodes -o json output.
func ParseNodesJSON(data []byte) ([]NodeInfo, error) {
	var list k8sNodeList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse nodes JSON: %w", err)
	}
	nodes := make([]NodeInfo, 0, len(list.Items))
	for _, item := range list.Items {
		node := NodeInfo{
			Name:           item.Metadata.Name,
			Status:         nodeReadyStatus(item.Status.Conditions),
			Roles:          nodeRoles(item.Metadata.Labels),
			GPUCount:       parseGPU(item.Status.Capacity),
			GPUAllocatable: parseGPU(item.Status.Allocatable),
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// ParsePodsJSON parses kubectl get pods -A -o json output.
func ParsePodsJSON(data []byte) ([]PodInfo, error) {
	var list k8sPodList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse pods JSON: %w", err)
	}
	pods := make([]PodInfo, 0, len(list.Items))
	for _, item := range list.Items {
		pods = append(pods, PodInfo{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Phase:     item.Status.Phase,
		})
	}
	return pods, nil
}

func nodeReadyStatus(conds []k8sCondition) string {
	for _, c := range conds {
		if c.Type == "Ready" {
			if c.Status == "True" {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

func nodeRoles(labels map[string]string) string {
	var roles []string
	for k := range labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		return "worker"
	}
	return strings.Join(roles, ",")
}

func parseGPU(resources map[string]string) int {
	v, ok := resources["nvidia.com/gpu"]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
