package k8smanifest

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQA_RealOmniParserManifest(t *testing.T) {
	path := os.Getenv("OMNIPARSER_MANIFEST")
	if path == "" {
		path = os.ExpandEnv("$HOME/Code/global-kb/k8s/uiauto/omniparser-deployment.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("skipping QA: omniparser manifest not found at %s", path)
	}
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser(data, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "real OmniParser manifest should pass validation; failures: %v", result.Failures)
	t.Logf("Passed checks: %v", result.Passed)
}

func TestQA_GPUScheduling_NodeSelector(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: uiauto
spec:
  template:
    spec:
      containers:
        - name: omniparser
          image: omniparser:v2
          resources:
            requests:
              nvidia.com/gpu: "1"
              memory: 2Gi
            limits:
              nvidia.com/gpu: "1"
              memory: 8Gi
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
          volumeMounts:
            - name: model-cache
              mountPath: /models
      nodeSelector:
        nvidia.com/gpu.present: "true"
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: omniparser-models
`
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "should pass with nodeSelector for GPU; failures: %v", result.Failures)
}

func TestQA_GPUScheduling_TwoGPU(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: uiauto
spec:
  template:
    spec:
      containers:
        - name: omniparser
          image: omniparser:v2
          resources:
            requests:
              nvidia.com/gpu: "2"
              memory: 4Gi
            limits:
              nvidia.com/gpu: "2"
              memory: 16Gi
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
          volumeMounts:
            - name: model-cache
              mountPath: /models
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: omniparser-models
`
	reqs := DefaultOmniParserRequirements()
	reqs.MinGPU = 2
	result, err := ValidateOmniParser([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "should pass with 2 GPUs; failures: %v", result.Failures)
}

func TestQA_ResourceLimits_ExactMinimum(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: uiauto
spec:
  template:
    spec:
      containers:
        - name: omniparser
          image: omniparser:v2
          resources:
            requests:
              nvidia.com/gpu: "1"
              memory: 2Gi
            limits:
              nvidia.com/gpu: "1"
              memory: 4Gi
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
          volumeMounts:
            - name: model-cache
              mountPath: /models
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: omniparser-models
`
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "4Gi = 4096Mi should meet 4096Mi minimum; failures: %v", result.Failures)
}

func TestQA_ResourceLimits_MiFormat(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: uiauto
spec:
  template:
    spec:
      containers:
        - name: omniparser
          image: omniparser:v2
          resources:
            requests:
              nvidia.com/gpu: "1"
              memory: 2048Mi
            limits:
              nvidia.com/gpu: "1"
              memory: 8192Mi
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
          volumeMounts:
            - name: model-cache
              mountPath: /models
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: omniparser-models
`
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "8192Mi should meet minimum; failures: %v", result.Failures)
}

func TestQA_HealthProbes_CustomPaths(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: uiauto
spec:
  template:
    spec:
      containers:
        - name: omniparser
          image: omniparser:v2
          resources:
            requests:
              nvidia.com/gpu: "1"
              memory: 2Gi
            limits:
              nvidia.com/gpu: "1"
              memory: 8Gi
          livenessProbe:
            httpGet:
              path: /health
              port: 8082
          readinessProbe:
            httpGet:
              path: /ready
              port: 8082
          volumeMounts:
            - name: model-cache
              mountPath: /models
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: omniparser-models
`
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "custom probe paths should still pass; failures: %v", result.Failures)
}

func TestQA_MultiDocManifest(t *testing.T) {
	yaml := `apiVersion: v1
kind: Service
metadata:
  name: omniparser
  namespace: uiauto
spec:
  selector:
    app.kubernetes.io/name: omniparser
  ports:
    - port: 8082
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: uiauto
spec:
  template:
    spec:
      containers:
        - name: omniparser
          image: omniparser:v2
          resources:
            requests:
              nvidia.com/gpu: "1"
              memory: 2Gi
            limits:
              nvidia.com/gpu: "1"
              memory: 8Gi
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
          volumeMounts:
            - name: model-cache
              mountPath: /models
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: omniparser-models
`
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "should find Deployment in multi-doc; failures: %v", result.Failures)
}

func TestQA_IngressWithBackendService(t *testing.T) {
	yaml := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: omniparser-ingress
  namespace: uiauto
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
spec:
  tls:
    - hosts:
        - omniparser.local
      secretName: omniparser-tls
  rules:
    - host: omniparser.local
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: omniparser
                port:
                  number: 8082
          - path: /metrics
            pathType: Exact
            backend:
              service:
                name: omniparser
                port:
                  number: 9090
`
	reqs := DefaultServiceMeshRequirements()
	result, err := ValidateServiceMesh([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "multi-path ingress should pass; failures: %v", result.Failures)
}

func TestQA_ParseMemoryValues(t *testing.T) {
	tests := []struct {
		input string
		wantMi int
	}{
		{"8Gi", 8192},
		{"4Gi", 4096},
		{"2048Mi", 2048},
		{"512Mi", 512},
		{"1Gi", 1024},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseMemoryMi(tt.input)
			assert.Equal(t, tt.wantMi, got)
		})
	}
}
