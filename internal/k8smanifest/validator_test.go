package k8smanifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validOmniParserYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: uiauto
  labels:
    app.kubernetes.io/name: omniparser
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: omniparser
  template:
    metadata:
      labels:
        app.kubernetes.io/name: omniparser
    spec:
      containers:
        - name: omniparser
          image: omniparser:v2
          ports:
            - name: http
              containerPort: 8082
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
      nodeSelector:
        nvidia.com/gpu.present: "true"
`

func TestValidateOmniParser_ValidManifest(t *testing.T) {
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser([]byte(validOmniParserYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "expected valid manifest to pass; failures: %v", result.Failures)
	assert.NotEmpty(t, result.Passed)
}

func TestValidateOmniParser_MissingGPU(t *testing.T) {
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
              memory: 2Gi
            limits:
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
	assert.False(t, result.OK(), "should fail without GPU resources")
	assert.Contains(t, result.Failures[0], "GPU")
}

func TestValidateOmniParser_MissingModelVolume(t *testing.T) {
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
      volumes: []
`
	reqs := DefaultOmniParserRequirements()
	result, err := ValidateOmniParser([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK())
	found := false
	for _, f := range result.Failures {
		if contains(f, "volume") || contains(f, "model") {
			found = true
		}
	}
	assert.True(t, found, "expected failure about model volume mount; got %v", result.Failures)
}

func TestValidateOmniParser_MissingHealthProbes(t *testing.T) {
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
	assert.False(t, result.OK())
	found := false
	for _, f := range result.Failures {
		if contains(f, "probe") || contains(f, "Probe") {
			found = true
		}
	}
	assert.True(t, found, "expected failure about health probes; got %v", result.Failures)
}

func TestValidateOmniParser_InsufficientMemoryLimit(t *testing.T) {
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
              memory: 1Gi
            limits:
              nvidia.com/gpu: "1"
              memory: 2Gi
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
	assert.False(t, result.OK())
	found := false
	for _, f := range result.Failures {
		if contains(f, "memory") {
			found = true
		}
	}
	assert.True(t, found, "expected failure about memory limits; got %v", result.Failures)
}

func TestValidateOmniParser_WrongNamespace(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: omniparser
  namespace: default
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
	assert.False(t, result.OK())
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
