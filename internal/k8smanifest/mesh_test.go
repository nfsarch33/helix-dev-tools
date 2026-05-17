package k8smanifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validIngressYAML = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: omniparser-ingress
  namespace: uiauto
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  tls:
    - hosts:
        - omniparser.local
      secretName: omniparser-tls
  rules:
    - host: omniparser.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: omniparser
                port:
                  number: 8082
`

const validNetworkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: omniparser-access
  namespace: uiauto
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: omniparser
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: monitoring
        - namespaceSelector:
            matchLabels:
              name: llm-cluster
      ports:
        - port: 8082
          protocol: TCP
`

func TestValidateServiceMesh_ValidIngress(t *testing.T) {
	reqs := DefaultServiceMeshRequirements()
	result, err := ValidateServiceMesh([]byte(validIngressYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid ingress should pass; failures: %v", result.Failures)
}

func TestValidateServiceMesh_MissingTLS(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: omniparser-ingress
  namespace: uiauto
spec:
  rules:
    - host: omniparser.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: omniparser
                port:
                  number: 8082
`
	reqs := DefaultServiceMeshRequirements()
	result, err := ValidateServiceMesh([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "ingress without TLS should fail")
}

func TestValidateServiceMesh_MissingHost(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: omniparser-ingress
  namespace: uiauto
spec:
  tls:
    - secretName: omniparser-tls
  rules:
    - http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: omniparser
                port:
                  number: 8082
`
	reqs := DefaultServiceMeshRequirements()
	result, err := ValidateServiceMesh([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "ingress without host should fail")
}

func TestValidateNetworkPolicy_Valid(t *testing.T) {
	reqs := DefaultServiceMeshRequirements()
	result, err := ValidateNetworkPolicy([]byte(validNetworkPolicyYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid network policy should pass; failures: %v", result.Failures)
}

func TestValidateNetworkPolicy_NoIngressRules(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: omniparser-access
  namespace: uiauto
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: omniparser
  policyTypes:
    - Ingress
  ingress: []
`
	reqs := DefaultServiceMeshRequirements()
	result, err := ValidateNetworkPolicy([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "network policy with no ingress rules should fail")
}

func TestValidateServiceMesh_TLSNotRequired(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: omniparser-ingress
  namespace: uiauto
spec:
  rules:
    - host: omniparser.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: omniparser
                port:
                  number: 8082
`
	reqs := DefaultServiceMeshRequirements()
	reqs.RequireTLS = false
	result, err := ValidateServiceMesh([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "ingress without TLS should pass when not required")
}
