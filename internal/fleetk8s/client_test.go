package fleetk8s

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const oneNodeJSON = `{
  "items": [{
    "metadata": {"name": "node-1", "labels": {"node-role.kubernetes.io/control-plane": ""}},
    "status": {
      "conditions": [{"type": "Ready", "status": "True"}],
      "capacity": {"nvidia.com/gpu": "3"},
      "allocatable": {"nvidia.com/gpu": "3"}
    }
  }]
}`

const twoNodeJSON = `{
  "items": [
    {
      "metadata": {"name": "node-1", "labels": {"node-role.kubernetes.io/control-plane": ""}},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "capacity": {"nvidia.com/gpu": "3"},
        "allocatable": {"nvidia.com/gpu": "3"}
      }
    },
    {
      "metadata": {"name": "node-2", "labels": {}},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "capacity": {"nvidia.com/gpu": "1"},
        "allocatable": {"nvidia.com/gpu": "1"}
      }
    }
  ]
}`

const gpuSaturatedJSON = `{
  "items": [{
    "metadata": {"name": "node-1", "labels": {"node-role.kubernetes.io/control-plane": ""}},
    "status": {
      "conditions": [{"type": "Ready", "status": "True"}],
      "capacity": {"nvidia.com/gpu": "3"},
      "allocatable": {"nvidia.com/gpu": "0"}
    }
  }]
}`

const noGPUNodeJSON = `{
  "items": [{
    "metadata": {"name": "control", "labels": {"node-role.kubernetes.io/control-plane": ""}},
    "status": {
      "conditions": [{"type": "Ready", "status": "True"}],
      "capacity": {},
      "allocatable": {}
    }
  }]
}`

const podPendingJSON = `{
  "items": [{
    "metadata": {"name": "vllm-abc", "namespace": "vllm"},
    "status": {"phase": "Pending"}
  }]
}`

const podFailedJSON = `{
  "items": [{
    "metadata": {"name": "mem0-xyz", "namespace": "mem0"},
    "status": {"phase": "Failed"}
  }]
}`

const podRunningJSON = `{
  "items": [{
    "metadata": {"name": "coredns-abc", "namespace": "kube-system"},
    "status": {"phase": "Running"}
  }]
}`

func TestParseNodes_SingleNode(t *testing.T) {
	nodes, err := ParseNodesJSON([]byte(oneNodeJSON))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-1", nodes[0].Name)
	assert.Equal(t, "Ready", nodes[0].Status)
	assert.Equal(t, "control-plane", nodes[0].Roles)
	assert.Equal(t, 3, nodes[0].GPUCount)
	assert.Equal(t, 3, nodes[0].GPUAllocatable)
}

func TestParseNodes_TwoNodes(t *testing.T) {
	nodes, err := ParseNodesJSON([]byte(twoNodeJSON))
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	assert.Equal(t, "node-1", nodes[0].Name)
	assert.Equal(t, 3, nodes[0].GPUCount)
	assert.Equal(t, "node-2", nodes[1].Name)
	assert.Equal(t, 1, nodes[1].GPUCount)
	assert.Equal(t, "worker", nodes[1].Roles)
}

func TestParseNodes_GPUSaturated(t *testing.T) {
	nodes, err := ParseNodesJSON([]byte(gpuSaturatedJSON))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, 3, nodes[0].GPUCount)
	assert.Equal(t, 0, nodes[0].GPUAllocatable)
}

func TestParsePods_Pending(t *testing.T) {
	pods, err := ParsePodsJSON([]byte(podPendingJSON))
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "vllm-abc", pods[0].Name)
	assert.Equal(t, "vllm", pods[0].Namespace)
	assert.Equal(t, "Pending", pods[0].Phase)
}

func TestParsePods_Failed(t *testing.T) {
	pods, err := ParsePodsJSON([]byte(podFailedJSON))
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "Failed", pods[0].Phase)
}

func TestParseNodes_NetworkDown(t *testing.T) {
	_, err := ParseNodesJSON([]byte(`not json`))
	require.Error(t, err)
}

func TestParseNodes_NoKubeconfig(t *testing.T) {
	runner := &fakeRunner{err: errors.New("kubeconfig not found")}
	_, err := runner.GetNodes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig")
}

func TestParseNodes_PartialStatus(t *testing.T) {
	nodes, err := ParseNodesJSON([]byte(noGPUNodeJSON))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, 0, nodes[0].GPUCount)
	assert.Equal(t, 0, nodes[0].GPUAllocatable)
	assert.Equal(t, "Ready", nodes[0].Status)
}

type fakeRunner struct {
	nodesJSON []byte
	podsJSON  []byte
	err       error
}

func (f *fakeRunner) GetNodes() ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.nodesJSON, nil
}

func (f *fakeRunner) GetPods() ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.podsJSON, nil
}
