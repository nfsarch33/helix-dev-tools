package fleetk8s

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	nodesJSON []byte
	podsJSON  []byte
	nodesErr  error
	podsErr   error
}

func (m *mockRunner) GetNodes() ([]byte, error) { return m.nodesJSON, m.nodesErr }
func (m *mockRunner) GetPods() ([]byte, error)  { return m.podsJSON, m.podsErr }

func makeNodesJSON(nodes ...k8sNode) []byte {
	list := k8sNodeList{Items: nodes}
	b, _ := json.Marshal(list)
	return b
}

func makePodsJSON(pods ...k8sPod) []byte {
	list := k8sPodList{Items: pods}
	b, _ := json.Marshal(list)
	return b
}

func readyNode(name string, gpus int) k8sNode {
	n := k8sNode{}
	n.Metadata.Name = name
	n.Metadata.Labels = map[string]string{"node-role.kubernetes.io/worker": ""}
	n.Status.Conditions = []k8sCondition{{Type: "Ready", Status: "True"}}
	n.Status.Capacity = map[string]string{"nvidia.com/gpu": json.Number(json.Number(string(rune('0' + gpus)))).String()}
	n.Status.Allocatable = map[string]string{"nvidia.com/gpu": json.Number(json.Number(string(rune('0' + gpus)))).String()}
	return n
}

func notReadyNode(name string) k8sNode {
	n := k8sNode{}
	n.Metadata.Name = name
	n.Status.Conditions = []k8sCondition{{Type: "Ready", Status: "False"}}
	n.Status.Capacity = map[string]string{}
	n.Status.Allocatable = map[string]string{}
	return n
}

func runningPod(name, ns string) k8sPod {
	p := k8sPod{}
	p.Metadata.Name = name
	p.Metadata.Namespace = ns
	p.Status.Phase = "Running"
	return p
}

func failedPod(name, ns string) k8sPod {
	p := k8sPod{}
	p.Metadata.Name = name
	p.Metadata.Namespace = ns
	p.Status.Phase = "Failed"
	return p
}

func TestHealthAdapter_AllHealthy(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("node-1", 1), readyNode("node-2", 2)),
		podsJSON:  makePodsJSON(runningPod("vllm-0", "llm-cluster"), runningPod("mem0-api-0", "mem0")),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.True(t, status.Healthy)
	assert.Len(t, status.NodeStatuses, 2)
	assert.Equal(t, 2, status.PodAggregation.TotalPods)
	assert.Equal(t, 2, status.PodAggregation.RunningPods)
	assert.Equal(t, 0, status.PodAggregation.FailedPods)
	assert.True(t, status.DaemonStatus.Running)
	assert.Equal(t, "v0.1.0", status.DaemonStatus.Version)
}

func TestHealthAdapter_NodeNotReady(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("node-1", 1), notReadyNode("node-2")),
		podsJSON:  makePodsJSON(runningPod("vllm-0", "llm-cluster")),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.False(t, status.Healthy, "cluster should be unhealthy with NotReady node")
	assert.False(t, status.NodeStatuses[1].Ready)
}

func TestHealthAdapter_FailedPods(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("node-1", 1)),
		podsJSON:  makePodsJSON(runningPod("vllm-0", "llm-cluster"), failedPod("crash-0", "llm-cluster")),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.False(t, status.Healthy, "cluster should be unhealthy with failed pods")
	assert.Equal(t, 1, status.PodAggregation.FailedPods)
}

func TestHealthAdapter_PodAggregationByNamespace(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("node-1", 1)),
		podsJSON: makePodsJSON(
			runningPod("vllm-0", "llm-cluster"),
			runningPod("mem0-api-0", "mem0"),
			runningPod("mem0-neo4j-0", "mem0"),
		),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)

	llmNS := status.PodAggregation.ByNamespace["llm-cluster"]
	assert.Equal(t, 1, llmNS.Total)
	assert.Equal(t, 1, llmNS.Running)

	mem0NS := status.PodAggregation.ByNamespace["mem0"]
	assert.Equal(t, 2, mem0NS.Total)
	assert.Equal(t, 2, mem0NS.Running)
}

func TestHealthAdapter_GPUTracking(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("gpu-node", 2)),
		podsJSON:  makePodsJSON(),
	}
	adapter := NewHealthAdapter(runner, "v0.1.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.Equal(t, 2, status.NodeStatuses[0].GPUCount)
	assert.Equal(t, 2, status.NodeStatuses[0].GPUAllocatable)
}

func TestHealthAdapter_DaemonEndpoint(t *testing.T) {
	runner := &mockRunner{
		nodesJSON: makeNodesJSON(readyNode("node-1", 0)),
		podsJSON:  makePodsJSON(),
	}
	adapter := NewHealthAdapter(runner, "v0.2.0")
	status, err := adapter.CheckHealth()
	require.NoError(t, err)
	assert.True(t, status.DaemonStatus.Running)
	assert.Equal(t, "60s", status.DaemonStatus.CheckInterval)
	assert.Equal(t, "v0.2.0", status.DaemonStatus.Version)
	assert.False(t, status.DaemonStatus.LastCheck.IsZero())
}
