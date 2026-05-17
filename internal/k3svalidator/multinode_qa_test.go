package k3svalidator

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQA_MultiNode_RealCluster(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run integration tests")
	}

	out, err := exec.Command("kubectl", "get", "nodes").Output()
	require.NoError(t, err)

	nodes, err := ParseNodeStatus(string(out))
	require.NoError(t, err)

	err = ValidateMultiNode(nodes, 2)
	assert.NoError(t, err, "production cluster must have at least 2 nodes")
}

func TestQA_GPUScheduling_RealCluster(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run integration tests")
	}

	out, err := exec.Command("kubectl", "get", "nodes").Output()
	require.NoError(t, err)

	nodes, err := ParseNodeStatus(string(out))
	require.NoError(t, err)

	check := ValidateCrossNodeScheduling(nodes)
	assert.True(t, check.CanScheduleNonGPU, "multi-node cluster must support cross-node scheduling")
}

func TestQA_DNSResolution_RealCluster(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run integration tests")
	}

	out, err := exec.Command("kubectl", "get", "svc", "-n", "kube-system", "kube-dns",
		"-o", "jsonpath={.spec.clusterIP}").Output()
	require.NoError(t, err)
	assert.NotEmpty(t, string(out), "kube-dns service must have a ClusterIP")
}

func TestQA_CrossNodeScheduling_Regression(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Status: "Ready", Roles: "control-plane"},
		{Name: "n2", Status: "Ready", Roles: "worker"},
		{Name: "n3", Status: "NotReady", Roles: "worker"},
	}
	check := ValidateCrossNodeScheduling(nodes)
	assert.True(t, check.CanScheduleNonGPU)
	assert.True(t, check.CanScheduleGPU)
}

func TestQA_MultiNode_ZeroNodes(t *testing.T) {
	err := ValidateMultiNode(nil, 1)
	require.Error(t, err)
}

func TestQA_ClassifyNodes_MixedStatus(t *testing.T) {
	nodes := []K3sNode{
		{Name: "cp", Status: "Ready", Roles: "control-plane"},
		{Name: "w1", Status: "NotReady", Roles: "worker"},
		{Name: "w2", Status: "Ready", Roles: "worker"},
	}
	labels := map[string]map[string]string{
		"w2": {"nvidia.com/gpu.product": "RTX-4070-Ti", "nvidia.com/gpu.count": "1"},
	}
	status := ClassifyNodes(nodes, labels)
	assert.Equal(t, 3, status.TotalNodes)
	assert.Equal(t, 2, status.ReadyNodes)
	require.Len(t, status.GPUNodes, 1)
	assert.Equal(t, "w2", status.GPUNodes[0].Name)
}
