package loggingvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validLokiStatefulSetYAML = `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: loki
  namespace: logging
spec:
  serviceName: loki
  replicas: 1
  selector:
    matchLabels:
      app: loki
  template:
    metadata:
      labels:
        app: loki
    spec:
      containers:
        - name: loki
          image: grafana/loki:3.1.0
          args:
            - -config.file=/etc/loki/config.yaml
          ports:
            - containerPort: 3100
          volumeMounts:
            - name: loki-data
              mountPath: /loki
            - name: loki-config
              mountPath: /etc/loki
      volumes:
        - name: loki-config
          configMap:
            name: loki-config
  volumeClaimTemplates:
    - metadata:
        name: loki-data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
        storageClassName: local-path
`

func TestValidateLokiStatefulSet_Valid(t *testing.T) {
	reqs := DefaultLokiRequirements()
	result, err := ValidateLokiStatefulSet([]byte(validLokiStatefulSetYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid Loki StatefulSet should pass; failures: %v", result.Failures)
	assert.NotEmpty(t, result.Passed)
}

func TestValidateLokiStatefulSet_WrongNamespace(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: loki
  namespace: default
spec:
  serviceName: loki
  replicas: 1
  template:
    spec:
      containers:
        - name: loki
          image: grafana/loki:3.1.0
          volumeMounts:
            - name: loki-data
              mountPath: /loki
  volumeClaimTemplates:
    - metadata:
        name: loki-data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
`
	reqs := DefaultLokiRequirements()
	result, err := ValidateLokiStatefulSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "wrong namespace should fail")
}

func TestValidateLokiStatefulSet_NoPersistence(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: loki
  namespace: logging
spec:
  serviceName: loki
  replicas: 1
  template:
    spec:
      containers:
        - name: loki
          image: grafana/loki:3.1.0
  volumeClaimTemplates: []
`
	reqs := DefaultLokiRequirements()
	result, err := ValidateLokiStatefulSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "no persistence should fail")
}

func TestValidateLokiStatefulSet_StorageTooSmall(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: loki
  namespace: logging
spec:
  serviceName: loki
  replicas: 1
  template:
    spec:
      containers:
        - name: loki
          image: grafana/loki:3.1.0
          volumeMounts:
            - name: loki-data
              mountPath: /loki
  volumeClaimTemplates:
    - metadata:
        name: loki-data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 5Gi
`
	reqs := DefaultLokiRequirements()
	result, err := ValidateLokiStatefulSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "5Gi should fail minimum 10Gi check")
}

func TestValidateLokiStatefulSet_WrongKind(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki
  namespace: logging
spec:
  template:
    spec:
      containers:
        - name: loki
          image: grafana/loki:3.1.0
`
	reqs := DefaultLokiRequirements()
	result, err := ValidateLokiStatefulSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "Deployment instead of StatefulSet should fail")
}

const validPromtailDaemonSetYAML = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: logging
spec:
  selector:
    matchLabels:
      app: promtail
  template:
    metadata:
      labels:
        app: promtail
    spec:
      containers:
        - name: promtail
          image: grafana/promtail:3.1.0
          args:
            - -config.file=/etc/promtail/config.yaml
          volumeMounts:
            - name: host-logs
              mountPath: /var/log
              readOnly: true
            - name: pod-logs
              mountPath: /var/log/pods
              readOnly: true
            - name: promtail-config
              mountPath: /etc/promtail
          env:
            - name: LOKI_ENDPOINT
              value: http://loki:3100/loki/api/v1/push
      volumes:
        - name: host-logs
          hostPath:
            path: /var/log
        - name: pod-logs
          hostPath:
            path: /var/log/pods
        - name: promtail-config
          configMap:
            name: promtail-config
`

func TestValidatePromtailDaemonSet_Valid(t *testing.T) {
	reqs := DefaultPromtailRequirements()
	result, err := ValidatePromtailDaemonSet([]byte(validPromtailDaemonSetYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid Promtail DaemonSet should pass; failures: %v", result.Failures)
	assert.NotEmpty(t, result.Passed)
}

func TestValidatePromtailDaemonSet_WrongNamespace(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: default
spec:
  template:
    spec:
      containers:
        - name: promtail
          image: grafana/promtail:3.1.0
          volumeMounts:
            - name: host-logs
              mountPath: /var/log
            - name: pod-logs
              mountPath: /var/log/pods
          env:
            - name: LOKI_ENDPOINT
              value: http://loki:3100/loki/api/v1/push
      volumes:
        - name: host-logs
          hostPath:
            path: /var/log
        - name: pod-logs
          hostPath:
            path: /var/log/pods
`
	reqs := DefaultPromtailRequirements()
	result, err := ValidatePromtailDaemonSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK())
}

func TestValidatePromtailDaemonSet_NoHostLogMount(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: logging
spec:
  template:
    spec:
      containers:
        - name: promtail
          image: grafana/promtail:3.1.0
          volumeMounts:
            - name: pod-logs
              mountPath: /var/log/pods
          env:
            - name: LOKI_ENDPOINT
              value: http://loki:3100/loki/api/v1/push
      volumes:
        - name: pod-logs
          hostPath:
            path: /var/log/pods
`
	reqs := DefaultPromtailRequirements()
	result, err := ValidatePromtailDaemonSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail without /var/log host mount")
}

func TestValidatePromtailDaemonSet_NoPodLogMount(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: logging
spec:
  template:
    spec:
      containers:
        - name: promtail
          image: grafana/promtail:3.1.0
          volumeMounts:
            - name: host-logs
              mountPath: /var/log
          env:
            - name: LOKI_ENDPOINT
              value: http://loki:3100/loki/api/v1/push
      volumes:
        - name: host-logs
          hostPath:
            path: /var/log
`
	reqs := DefaultPromtailRequirements()
	result, err := ValidatePromtailDaemonSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail without /var/log/pods mount")
}

func TestValidatePromtailDaemonSet_WrongKind(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: promtail
  namespace: logging
spec:
  template:
    spec:
      containers:
        - name: promtail
          image: grafana/promtail:3.1.0
`
	reqs := DefaultPromtailRequirements()
	result, err := ValidatePromtailDaemonSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "Deployment instead of DaemonSet should fail")
}

func TestValidateLogRetention_Valid(t *testing.T) {
	cfg := `auth_enabled: false
limits_config:
  retention_period: 168h
compactor:
  working_directory: /loki/compactor
  compaction_interval: 10m
  retention_enabled: true
  retention_delete_delay: 2h
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateLogRetention([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid retention config should pass; failures: %v", result.Failures)
}

func TestValidateLogRetention_TooShort(t *testing.T) {
	cfg := `limits_config:
  retention_period: 48h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateLogRetention([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "48h retention below 7 day minimum should fail")
}

func TestValidateLogRetention_TooLong(t *testing.T) {
	cfg := `limits_config:
  retention_period: 2160h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateLogRetention([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "90 day retention above 30 day maximum should fail")
}

func TestValidateLogRetention_NoCompaction(t *testing.T) {
	cfg := `limits_config:
  retention_period: 168h
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateLogRetention([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail without compactor retention_enabled")
}

func TestValidateNamespaceIsolation_Valid(t *testing.T) {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: logging
  labels:
    purpose: centralized-logging
    team: platform
`
	result, err := ValidateNamespaceIsolation([]byte(manifest), "logging")
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid namespace should pass; failures: %v", result.Failures)
}

func TestValidateNamespaceIsolation_WrongName(t *testing.T) {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: default
`
	result, err := ValidateNamespaceIsolation([]byte(manifest), "logging")
	require.NoError(t, err)
	assert.False(t, result.OK())
}

func TestValidateNamespaceIsolation_WrongKind(t *testing.T) {
	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: logging
`
	result, err := ValidateNamespaceIsolation([]byte(manifest), "logging")
	require.NoError(t, err)
	assert.False(t, result.OK())
}
