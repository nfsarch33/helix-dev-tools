package ingressvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validIngressYAML = `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web-ingress
  namespace: production
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    kubernetes.io/ingress.class: nginx
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - example.com
        - www.example.com
      secretName: example-tls
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web-service
                port:
                  number: 80
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: api-service
                port:
                  number: 8080
`

func TestParseIngress_Valid(t *testing.T) {
	ing, err := ParseIngress([]byte(validIngressYAML))
	require.NoError(t, err)
	assert.Equal(t, "web-ingress", ing.Metadata.Name)
	assert.Equal(t, "nginx", ing.Spec.IngressClassName)
	assert.Len(t, ing.Spec.TLS, 1)
	assert.Len(t, ing.Spec.Rules, 1)
}

func TestParseIngress_Invalid(t *testing.T) {
	_, err := ParseIngress([]byte(`not valid yaml: [[[`))
	assert.Error(t, err)

	_, err = ParseIngress([]byte(`apiVersion: v1
kind: Service
metadata:
  name: not-ingress
`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Ingress")
}

func TestTLS_Valid(t *testing.T) {
	ing, err := ParseIngress([]byte(validIngressYAML))
	require.NoError(t, err)
	err = ValidateTLSConfig(*ing)
	assert.NoError(t, err)
}

func TestTLS_MissingSecret(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: no-tls-secret
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - example.com
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 80
`
	ing, err := ParseIngress([]byte(yaml))
	require.NoError(t, err)
	err = ValidateTLSConfig(*ing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secretName")
}

func TestTLS_HostMismatch(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: host-mismatch
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - other.com
      secretName: other-tls
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 80
`
	ing, err := ParseIngress([]byte(yaml))
	require.NoError(t, err)
	err = ValidateTLSConfig(*ing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not covered by TLS")
}

func TestIngressClass_Match(t *testing.T) {
	ing, err := ParseIngress([]byte(validIngressYAML))
	require.NoError(t, err)
	err = ValidateIngressClass(*ing, "nginx")
	assert.NoError(t, err)
}

func TestIngressClass_Mismatch(t *testing.T) {
	ing, err := ParseIngress([]byte(validIngressYAML))
	require.NoError(t, err)
	err = ValidateIngressClass(*ing, "traefik")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "traefik")
}

func TestPathRouting_Valid(t *testing.T) {
	ing, err := ParseIngress([]byte(validIngressYAML))
	require.NoError(t, err)
	err = ValidatePathRouting(*ing)
	assert.NoError(t, err)
}

func TestPathRouting_Duplicate(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dup-paths
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: api-v1
                port:
                  number: 8080
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: api-v2
                port:
                  number: 8081
`
	ing, err := ParseIngress([]byte(yaml))
	require.NoError(t, err)
	err = ValidatePathRouting(*ing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestPathRouting_MissingBackend(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: no-backend
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: ""
                port:
                  number: 0
`
	ing, err := ParseIngress([]byte(yaml))
	require.NoError(t, err)
	err = ValidatePathRouting(*ing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backend")
}

func TestCertManager_Valid(t *testing.T) {
	annotations := map[string]string{
		"cert-manager.io/cluster-issuer": "letsencrypt-prod",
	}
	err := ValidateCertManager(annotations)
	assert.NoError(t, err)
}

func TestCertManager_MissingAnnotation(t *testing.T) {
	annotations := map[string]string{
		"kubernetes.io/ingress.class": "nginx",
	}
	err := ValidateCertManager(annotations)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster-issuer")
}
