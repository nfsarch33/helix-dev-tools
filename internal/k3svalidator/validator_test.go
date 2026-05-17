package k3svalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNodeStatus_SingleReady(t *testing.T) {
	output := `NAME              STATUS   ROLES           AGE   VERSION
desktop-078m990   Ready    control-plane   13h   v1.35.4+k3s1`
	nodes, err := ParseNodeStatus(output)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "desktop-078m990", nodes[0].Name)
	assert.Equal(t, "Ready", nodes[0].Status)
	assert.Equal(t, "control-plane", nodes[0].Roles)
	assert.Equal(t, "v1.35.4+k3s1", nodes[0].Version)
}

func TestParseNodeStatus_MultipleNodes(t *testing.T) {
	output := `NAME              STATUS   ROLES           AGE   VERSION
desktop-078m990   Ready    control-plane   13h   v1.35.4+k3s1
desktop-0s5prj9   Ready    <none>          13h   v1.35.4+k3s1`
	nodes, err := ParseNodeStatus(output)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	assert.Equal(t, "Ready", nodes[0].Status)
	assert.Equal(t, "Ready", nodes[1].Status)
	assert.Equal(t, "worker", nodes[1].Roles)
}

func TestParseNodeStatus_NotReady(t *testing.T) {
	output := `NAME              STATUS     ROLES           AGE   VERSION
desktop-078m990   NotReady   control-plane   13h   v1.35.4+k3s1`
	nodes, err := ParseNodeStatus(output)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "NotReady", nodes[0].Status)
}

func TestParseNodeStatus_EmptyOutput(t *testing.T) {
	_, err := ParseNodeStatus("")
	require.Error(t, err)
}

func TestParseNodeStatus_HeaderOnly(t *testing.T) {
	output := `NAME   STATUS   ROLES   AGE   VERSION`
	nodes, err := ParseNodeStatus(output)
	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestValidateAllReady_AllReady(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Status: "Ready"},
		{Name: "n2", Status: "Ready"},
	}
	err := ValidateAllReady(nodes)
	assert.NoError(t, err)
}

func TestValidateAllReady_OneNotReady(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Status: "Ready"},
		{Name: "n2", Status: "NotReady"},
	}
	err := ValidateAllReady(nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "n2")
}

func TestValidateAllReady_Empty(t *testing.T) {
	err := ValidateAllReady(nil)
	require.Error(t, err)
}

func TestParseK3sVersion_Valid(t *testing.T) {
	output := `k3s version v1.35.4+k3s1 (5dc8fe68)
go version go1.25.9`
	ver, err := ParseK3sVersion(output)
	require.NoError(t, err)
	assert.Equal(t, "v1.35.4+k3s1", ver.K3sVersion)
	assert.Equal(t, "1.35.4", ver.KubeVersion)
	assert.Equal(t, "go1.25.9", ver.GoVersion)
}

func TestParseK3sVersion_Empty(t *testing.T) {
	_, err := ParseK3sVersion("")
	require.Error(t, err)
}

func TestParseK3sVersion_Malformed(t *testing.T) {
	_, err := ParseK3sVersion("not a version string")
	require.Error(t, err)
}

func TestCheckVersionCompatibility_Same(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Version: "v1.35.4+k3s1"},
		{Name: "n2", Version: "v1.35.4+k3s1"},
	}
	err := CheckVersionCompatibility(nodes)
	assert.NoError(t, err)
}

func TestCheckVersionCompatibility_MinorSkew(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Version: "v1.35.4+k3s1"},
		{Name: "n2", Version: "v1.34.2+k3s1"},
	}
	err := CheckVersionCompatibility(nodes)
	assert.NoError(t, err)
}

func TestCheckVersionCompatibility_TooMuchSkew(t *testing.T) {
	nodes := []K3sNode{
		{Name: "n1", Version: "v1.35.4+k3s1"},
		{Name: "n2", Version: "v1.33.0+k3s1"},
	}
	err := CheckVersionCompatibility(nodes)
	require.Error(t, err)
}

func TestValidateKubeconfig_Valid(t *testing.T) {
	yaml := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
    certificate-authority-data: LS0t
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    client-certificate-data: LS0t
    client-key-data: LS0t`
	err := ValidateKubeconfig([]byte(yaml))
	assert.NoError(t, err)
}

func TestValidateKubeconfig_Empty(t *testing.T) {
	err := ValidateKubeconfig(nil)
	require.Error(t, err)
}

func TestValidateKubeconfig_MissingServer(t *testing.T) {
	yaml := `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: LS0t
  name: default`
	err := ValidateKubeconfig([]byte(yaml))
	require.Error(t, err)
}

func TestValidateKubeconfig_NotYAML(t *testing.T) {
	err := ValidateKubeconfig([]byte("not yaml at all {{{"))
	require.Error(t, err)
}
