package fleetk8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollector_TwoNodeCluster(t *testing.T) {
	runner := &fakeRunner{
		nodesJSON: []byte(twoNodeJSON),
		podsJSON:  []byte(podRunningJSON),
	}
	c := NewCollector(runner)
	status, err := c.Collect()
	require.NoError(t, err)

	assert.Equal(t, 2, status.Summary.TotalNodes)
	assert.Equal(t, 2, status.Summary.ReadyNodes)
	assert.Equal(t, 4, status.Summary.TotalGPUs)
	assert.Equal(t, 4, status.Summary.AllocatableGPUs)
	require.Len(t, status.Nodes, 2)
	require.Len(t, status.Pods, 1)
}

func TestCollector_GPUSaturated(t *testing.T) {
	runner := &fakeRunner{
		nodesJSON: []byte(gpuSaturatedJSON),
		podsJSON:  []byte(`{"items": []}`),
	}
	c := NewCollector(runner)
	status, err := c.Collect()
	require.NoError(t, err)

	assert.Equal(t, 3, status.Summary.TotalGPUs)
	assert.Equal(t, 0, status.Summary.AllocatableGPUs)
}

func TestCollector_NodeError(t *testing.T) {
	runner := &fakeRunner{err: assert.AnError}
	c := NewCollector(runner)
	_, err := c.Collect()
	require.Error(t, err)
}

func TestCollector_PodError(t *testing.T) {
	runner := &fakeRunner{
		nodesJSON: []byte(oneNodeJSON),
		err:       nil,
	}
	runnerBadPods := &fakeRunnerSplitErr{
		nodesJSON: []byte(oneNodeJSON),
		podsErr:   assert.AnError,
	}
	_ = runner
	c := NewCollector(runnerBadPods)
	_, err := c.Collect()
	require.Error(t, err)
}

func TestCollector_EmptyCluster(t *testing.T) {
	runner := &fakeRunner{
		nodesJSON: []byte(`{"items": []}`),
		podsJSON:  []byte(`{"items": []}`),
	}
	c := NewCollector(runner)
	status, err := c.Collect()
	require.NoError(t, err)
	assert.Equal(t, 0, status.Summary.TotalNodes)
	assert.Equal(t, 0, status.Summary.TotalGPUs)
	assert.Empty(t, status.Nodes)
	assert.Empty(t, status.Pods)
}

type fakeRunnerSplitErr struct {
	nodesJSON []byte
	podsErr   error
}

func (f *fakeRunnerSplitErr) GetNodes() ([]byte, error) {
	return f.nodesJSON, nil
}

func (f *fakeRunnerSplitErr) GetPods() ([]byte, error) {
	return nil, f.podsErr
}
