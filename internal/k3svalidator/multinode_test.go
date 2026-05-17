package k3svalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMultiNode_TwoNodes(t *testing.T) {
	nodes := []K3sNode{
		{Name: "cp-1", Status: "Ready", Roles: "control-plane"},
		{Name: "worker-1", Status: "Ready", Roles: "worker"},
	}
	err := ValidateMultiNode(nodes, 2)
	assert.NoError(t, err)
}

func TestValidateMultiNode_InsufficientNodes(t *testing.T) {
	nodes := []K3sNode{
		{Name: "cp-1", Status: "Ready", Roles: "control-plane"},
	}
	err := ValidateMultiNode(nodes, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minimum required is 2")
}

func TestValidateMultiNode_NoControlPlane(t *testing.T) {
	nodes := []K3sNode{
		{Name: "w1", Status: "Ready", Roles: "worker"},
		{Name: "w2", Status: "Ready", Roles: "worker"},
	}
	err := ValidateMultiNode(nodes, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no control-plane")
}

func TestClassifyNodes_WithGPU(t *testing.T) {
	nodes := []K3sNode{
		{Name: "cp-1", Status: "Ready", Roles: "control-plane"},
		{Name: "gpu-1", Status: "Ready", Roles: "worker"},
	}
	labels := map[string]map[string]string{
		"gpu-1": {
			"nvidia.com/gpu.product": "NVIDIA-GeForce-RTX-3090",
			"nvidia.com/gpu.count":   "1",
		},
	}
	status := ClassifyNodes(nodes, labels)
	assert.Equal(t, 2, status.TotalNodes)
	assert.Equal(t, 2, status.ReadyNodes)
	assert.Equal(t, []string{"cp-1"}, status.ControlPlanes)
	assert.Equal(t, []string{"gpu-1"}, status.Workers)
	require.Len(t, status.GPUNodes, 1)
	assert.Equal(t, "NVIDIA-GeForce-RTX-3090", status.GPUNodes[0].GPUProduct)
}

func TestClassifyNodes_NoGPU(t *testing.T) {
	nodes := []K3sNode{
		{Name: "cp-1", Status: "Ready", Roles: "control-plane"},
		{Name: "w-1", Status: "Ready", Roles: "worker"},
	}
	labels := map[string]map[string]string{}
	status := ClassifyNodes(nodes, labels)
	assert.Empty(t, status.GPUNodes)
}

func TestValidateGPULabels_AllPresent(t *testing.T) {
	labels := map[string]map[string]string{
		"gpu-1": {"nvidia.com/gpu.product": "RTX-3090", "nvidia.com/gpu.count": "1"},
		"gpu-2": {"nvidia.com/gpu.product": "RTX-4070-Ti", "nvidia.com/gpu.count": "1"},
	}
	err := ValidateGPULabels(labels, []string{"gpu-1", "gpu-2"})
	assert.NoError(t, err)
}

func TestValidateGPULabels_Missing(t *testing.T) {
	labels := map[string]map[string]string{
		"gpu-1": {"nvidia.com/gpu.product": "RTX-3090"},
	}
	err := ValidateGPULabels(labels, []string{"gpu-1", "gpu-2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gpu-2")
}

func TestValidateGPULabels_NoProductLabel(t *testing.T) {
	labels := map[string]map[string]string{
		"gpu-1": {"nvidia.com/gpu.count": "1"},
	}
	err := ValidateGPULabels(labels, []string{"gpu-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no gpu.product label")
}

func TestValidateCrossNodeScheduling_TwoReady(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Status: "Ready"},
		{Name: "n2", Status: "Ready"},
	}
	check := ValidateCrossNodeScheduling(nodes)
	assert.True(t, check.CanScheduleGPU)
	assert.True(t, check.CanScheduleNonGPU)
	assert.True(t, check.DNSResolution)
	assert.Empty(t, check.Error)
}

func TestValidateCrossNodeScheduling_SingleNode(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Status: "Ready"},
	}
	check := ValidateCrossNodeScheduling(nodes)
	assert.NotEmpty(t, check.Error)
}

func TestValidateCrossNodeScheduling_OneNotReady(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Status: "Ready"},
		{Name: "n2", Status: "NotReady"},
	}
	check := ValidateCrossNodeScheduling(nodes)
	assert.False(t, check.CanScheduleNonGPU)
	assert.True(t, check.CanScheduleGPU)
}
