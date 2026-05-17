package k3svalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDashboardRBAC_Valid(t *testing.T) {
	yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dashboard-reader
rules:
- apiGroups: [""]
  resources: ["pods", "services", "nodes"]
  verbs: ["get", "list", "watch"]`
	err := ValidateDashboardRBAC([]byte(yaml))
	assert.NoError(t, err)
}

func TestValidateDashboardRBAC_OverlyPermissive(t *testing.T) {
	yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: god-mode
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]`
	err := ValidateDashboardRBAC([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overly permissive")
}

func TestValidateDashboardRBAC_Empty(t *testing.T) {
	err := ValidateDashboardRBAC(nil)
	require.Error(t, err)
}

func TestValidateDashboardRBAC_WrongKind(t *testing.T) {
	yaml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: foo`
	err := ValidateDashboardRBAC([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected ClusterRole or Role")
}

func TestValidateDashboardRBAC_NoName(t *testing.T) {
	yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app: dashboard
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]`
	err := ValidateDashboardRBAC([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no name")
}

func TestValidateServiceAccount_Valid(t *testing.T) {
	yaml := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: dashboard-admin
  namespace: kubernetes-dashboard`
	err := ValidateServiceAccount([]byte(yaml))
	assert.NoError(t, err)
}

func TestValidateServiceAccount_Empty(t *testing.T) {
	err := ValidateServiceAccount(nil)
	require.Error(t, err)
}

func TestValidateServiceAccount_NoNamespace(t *testing.T) {
	yaml := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: dashboard-admin`
	err := ValidateServiceAccount([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no namespace")
}

func TestValidateServiceAccount_WrongKind(t *testing.T) {
	yaml := `apiVersion: v1
kind: Secret
metadata:
  name: foo
  namespace: bar`
	err := ValidateServiceAccount([]byte(yaml))
	require.Error(t, err)
}

func TestValidateNodePortService_Valid(t *testing.T) {
	yaml := `apiVersion: v1
kind: Service
metadata:
  name: dashboard-nodeport
spec:
  type: NodePort
  ports:
  - port: 443
    targetPort: 8443
    nodePort: 30043`
	err := ValidateNodePortService([]byte(yaml), 30043)
	assert.NoError(t, err)
}

func TestValidateNodePortService_WrongPort(t *testing.T) {
	yaml := `apiVersion: v1
kind: Service
metadata:
  name: dashboard-nodeport
spec:
  type: NodePort
  ports:
  - port: 443
    targetPort: 8443
    nodePort: 30080`
	err := ValidateNodePortService([]byte(yaml), 30043)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "30043 not found")
}

func TestValidateNodePortService_NotNodePort(t *testing.T) {
	yaml := `apiVersion: v1
kind: Service
metadata:
  name: dashboard-clusterip
spec:
  type: ClusterIP
  ports:
  - port: 443
    targetPort: 8443`
	err := ValidateNodePortService([]byte(yaml), 30043)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected NodePort")
}

func TestValidateNodePortService_Empty(t *testing.T) {
	err := ValidateNodePortService(nil, 30043)
	require.Error(t, err)
}

func TestValidateNodePortService_AnyPort(t *testing.T) {
	yaml := `apiVersion: v1
kind: Service
metadata:
  name: dashboard-nodeport
spec:
  type: NodePort
  ports:
  - port: 443
    nodePort: 31234`
	err := ValidateNodePortService([]byte(yaml), 0)
	assert.NoError(t, err)
}

func TestCheckPortAccessibility_InvalidHost(t *testing.T) {
	err := CheckPortAccessibility("192.0.2.1", 99999, 100*1000*1000)
	require.Error(t, err)
}
