package k3svalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegression_NodeNotReady_MultipleNodes(t *testing.T) {
	output := `NAME              STATUS     ROLES           AGE   VERSION
node-a            Ready      control-plane   5d    v1.35.4+k3s1
node-b            NotReady   <none>          3d    v1.35.4+k3s1
node-c            Ready      <none>          1d    v1.35.4+k3s1`
	nodes, err := ParseNodeStatus(output)
	require.NoError(t, err)
	require.Len(t, nodes, 3)

	err = ValidateAllReady(nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node-b")
	assert.NotContains(t, err.Error(), "node-a")
	assert.NotContains(t, err.Error(), "node-c")
}

func TestRegression_AllNotReady(t *testing.T) {
	output := `NAME     STATUS     ROLES           AGE   VERSION
node-a   NotReady   control-plane   5d    v1.35.4+k3s1
node-b   NotReady   <none>          3d    v1.35.4+k3s1`
	nodes, err := ParseNodeStatus(output)
	require.NoError(t, err)

	err = ValidateAllReady(nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node-a")
	assert.Contains(t, err.Error(), "node-b")
}

func TestRegression_VersionMismatch_ThreeNodes(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Version: "v1.35.4+k3s1"},
		{Name: "n2", Version: "v1.34.0+k3s1"},
		{Name: "n3", Version: "v1.33.0+k3s1"},
	}
	err := CheckVersionCompatibility(nodes)
	require.Error(t, err)
}

func TestRegression_VersionParse_NoPlus(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Version: "v1.35.4"},
		{Name: "n2", Version: "v1.35.4"},
	}
	err := CheckVersionCompatibility(nodes)
	assert.NoError(t, err)
}

func TestRegression_SingleNode_AlwaysCompatible(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Version: "v1.35.4+k3s1"},
	}
	err := CheckVersionCompatibility(nodes)
	assert.NoError(t, err)
}

func TestRegression_Kubeconfig_MultipleCluster(t *testing.T) {
	yaml := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://10.0.0.1:6443
    certificate-authority-data: LS0t
  name: cluster-1
- cluster:
    server: https://10.0.0.2:6443
    certificate-authority-data: LS0t
  name: cluster-2
contexts:
- context:
    cluster: cluster-1
    user: admin
  name: ctx1
current-context: ctx1
users:
- name: admin
  user:
    client-certificate-data: LS0t`
	err := ValidateKubeconfig([]byte(yaml))
	assert.NoError(t, err)
}

func TestRegression_Kubeconfig_OneClusterMissingServer(t *testing.T) {
	yaml := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://10.0.0.1:6443
  name: good
- cluster:
    certificate-authority-data: LS0t
  name: bad`
	err := ValidateKubeconfig([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
}

func TestRegression_ParseNodeStatus_ExtraWhitespace(t *testing.T) {
	output := `NAME              STATUS   ROLES           AGE   VERSION
  desktop-078m990   Ready    control-plane   13h   v1.35.4+k3s1  `
	nodes, err := ParseNodeStatus(output)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "desktop-078m990", nodes[0].Name)
}

func TestRegression_ParseK3sVersion_OlderFormat(t *testing.T) {
	output := `k3s version v1.30.6+k3s1 (abc12345)
go version go1.23.0`
	ver, err := ParseK3sVersion(output)
	require.NoError(t, err)
	assert.Equal(t, "v1.30.6+k3s1", ver.K3sVersion)
	assert.Equal(t, "1.30.6", ver.KubeVersion)
	assert.Equal(t, "go1.23.0", ver.GoVersion)
}

// Dashboard health probe tests
func TestDashboardHealthProbe_URLConstruction(t *testing.T) {
	probe := NewDashboardProbe("10.0.0.1", 30043)
	assert.Equal(t, "https://10.0.0.1:30043", probe.URL())
}

func TestDashboardHealthProbe_DefaultPort(t *testing.T) {
	probe := NewDashboardProbe("localhost", 0)
	assert.Equal(t, "https://localhost:30043", probe.URL())
}
