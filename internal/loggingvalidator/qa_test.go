package loggingvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- QA: Manifest correctness ---

func TestQA_LokiStatefulSet_MissingImage(t *testing.T) {
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
	assert.True(t, result.OK(), "image is not a validator requirement; should still pass structural checks")
}

func TestQA_LokiStatefulSet_LargeStorage(t *testing.T) {
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
            storage: 1Ti
`
	reqs := DefaultLokiRequirements()
	result, err := ValidateLokiStatefulSet([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "1Ti exceeds 10Gi minimum; should pass")
}

func TestQA_PromtailDaemonSet_WithAllMounts(t *testing.T) {
	result, err := ValidatePromtailDaemonSet([]byte(validPromtailDaemonSetYAML), DefaultPromtailRequirements())
	require.NoError(t, err)
	assert.True(t, result.OK())
	assert.GreaterOrEqual(t, len(result.Passed), 4, "should have at least 4 passed checks")
}

func TestQA_PromtailDaemonSet_NoLokiEndpoint(t *testing.T) {
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
            - name: pod-logs
              mountPath: /var/log/pods
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
	assert.False(t, result.OK(), "should fail without Loki endpoint env var")
}

// --- QA: Query syntax ---

func TestQA_LogQLSyntax_SumRate(t *testing.T) {
	query := `sum(rate({namespace="logging", pod=~"loki.*"} |= "error" [5m])) by (pod)`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.True(t, result.OK(), "sum(rate(...)) should be valid; failures: %v", result.Failures)
}

func TestQA_LogQLSyntax_CountOverTime(t *testing.T) {
	query := `count_over_time({namespace="kube-system"} |= "oom" [1h])`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.True(t, result.OK(), "count_over_time should be valid; failures: %v", result.Failures)
}

func TestQA_LogQLSyntax_JSONPipeline(t *testing.T) {
	query := `{namespace="logging"} | json | level="error" | line_format "{{.message}}"`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.True(t, result.OK())
}

func TestQA_LabelMatching_ExtraLabelsOK(t *testing.T) {
	query := `{namespace="logging", pod="loki-0", container="loki", stream="stderr"}`
	reqs := DefaultLogQueryRequirements()
	result, err := ValidateLabelMatching(query, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "extra labels beyond required should still pass")
}

func TestQA_LabelMatching_NoLabelsAtAll(t *testing.T) {
	query := `{}`
	reqs := DefaultLogQueryRequirements()
	result, err := ValidateLabelMatching(query, reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "empty selector should fail label requirements")
}

// --- QA: Retention config ---

func TestQA_RetentionCompliance_Exact7Days(t *testing.T) {
	cfg := `limits_config:
  retention_period: 168h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateRetentionCompliance([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "exactly 7 days should be within range")
}

func TestQA_RetentionCompliance_Exact30Days(t *testing.T) {
	cfg := `limits_config:
  retention_period: 720h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateRetentionCompliance([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "exactly 30 days should be within range")
}

func TestQA_RetentionCompliance_14Days(t *testing.T) {
	cfg := `limits_config:
  retention_period: 336h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateRetentionCompliance([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "14 days within range should pass")
}

// --- QA: Log forwarding paths ---

func TestQA_PromtailForwarding_CustomEndpoint(t *testing.T) {
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
            - name: pod-logs
              mountPath: /var/log/pods
          env:
            - name: LOKI_ENDPOINT
              value: http://loki.logging.svc.cluster.local:3100/loki/api/v1/push
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
	assert.True(t, result.OK(), "FQDN Loki endpoint should pass; failures: %v", result.Failures)
}

func TestQA_NamespaceIsolation_WithLabels(t *testing.T) {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: logging
  labels:
    purpose: centralized-logging
    team: platform
    environment: production
`
	result, err := ValidateNamespaceIsolation([]byte(manifest), "logging")
	require.NoError(t, err)
	assert.True(t, result.OK())
}

func TestQA_LogRetention_ZeroHours(t *testing.T) {
	cfg := `limits_config:
  retention_period: 0h
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateLogRetention([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "0h retention should fail")
}

func TestQA_LogRetention_MissingPeriod(t *testing.T) {
	cfg := `limits_config: {}
compactor:
  retention_enabled: true
`
	reqs := DefaultLogRetentionRequirements()
	result, err := ValidateLogRetention([]byte(cfg), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "missing retention_period should fail")
}

func TestQA_LogQLSyntax_MultiplePipelines(t *testing.T) {
	query := `{namespace="logging"} |= "error" != "timeout" | json | line_format "{{.ts}} {{.msg}}"`
	result, err := ValidateLogQLSyntax(query)
	require.NoError(t, err)
	assert.True(t, result.OK(), "multiple pipeline stages should be valid")
}

func TestQA_LabelMatching_RegexOperator(t *testing.T) {
	query := `{namespace=~"log.*", pod!~"debug.*"}`
	reqs := DefaultLogQueryRequirements()
	result, err := ValidateLabelMatching(query, reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "regex operators should still extract label names; failures: %v", result.Failures)
}
