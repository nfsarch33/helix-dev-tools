package k3svalidator

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQA_Dashboard_NamespaceExists(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run integration tests")
	}

	out, err := exec.Command("kubectl", "get", "ns", "kubernetes-dashboard",
		"-o", "jsonpath={.status.phase}").Output()
	require.NoError(t, err)
	assert.Equal(t, "Active", string(out))
}

func TestQA_Dashboard_ServiceExists(t *testing.T) {
	if os.Getenv("CURSOR_TOOLS_INTEGRATION") != "1" {
		t.Skip("set CURSOR_TOOLS_INTEGRATION=1 to run integration tests")
	}

	out, err := exec.Command("kubectl", "get", "svc", "kubernetes-dashboard",
		"-n", "kubernetes-dashboard", "-o", "jsonpath={.spec.type}").Output()
	require.NoError(t, err)
	assert.NotEmpty(t, string(out))
}

func TestQA_Dashboard_Port30043_Unit(t *testing.T) {
	probe := NewDashboardProbe("127.0.0.1", 30043)
	assert.Equal(t, "https://127.0.0.1:30043", probe.URL())
}

func TestQA_Dashboard_RBAC_ReadOnlyIsValid(t *testing.T) {
	yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dashboard-readonly
rules:
- apiGroups: [""]
  resources: ["pods", "services", "namespaces", "nodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch"]`
	err := ValidateDashboardRBAC([]byte(yaml))
	assert.NoError(t, err)
}

func TestQA_Dashboard_RBAC_AdminIsTooWide(t *testing.T) {
	yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dashboard-admin
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]`
	err := ValidateDashboardRBAC([]byte(yaml))
	require.Error(t, err)
}

func TestQA_Dashboard_PortAccessibility_Timeout(t *testing.T) {
	err := CheckPortAccessibility("192.0.2.1", 30043, 200*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not accessible")
}

func TestQA_Dashboard_RBAC_RoleKindAccepted(t *testing.T) {
	yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: dashboard-ns-reader
  namespace: kubernetes-dashboard
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]`
	err := ValidateDashboardRBAC([]byte(yaml))
	assert.NoError(t, err)
}
