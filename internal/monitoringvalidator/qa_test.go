package monitoringvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQA_ScrapeTargetCoverage_AllTargets(t *testing.T) {
	yaml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
      - job_name: vllm
        static_configs:
          - targets: ['vllm-3090:9090', 'vllm-4070ti:9090']
      - job_name: node-exporter
        static_configs:
          - targets: ['gpu-host-1:9100', 'gpu-host-2:9100', 'control-plane:9100']
      - job_name: llm-router
        static_configs:
          - targets: ['llm-router:9787']
      - job_name: mem0-api
        static_configs:
          - targets: ['mem0-api:8080']
      - job_name: fleet-health
        static_configs:
          - targets: ['fleet-health:9788']
`
	reqs := DefaultPrometheusRequirements()
	reqs.RequiredScrapeTargets = []string{"vllm", "node-exporter", "llm-router", "mem0-api", "fleet-health"}
	result, err := ValidatePrometheusConfig([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "all 5 scrape targets should be covered; failures: %v", result.Failures)
	assert.Len(t, result.Passed, 6) // 5 targets + namespace
}

func TestQA_DashboardJSON_GrafanaVolumeMount(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
  namespace: monitoring
spec:
  template:
    spec:
      containers:
        - name: grafana
          image: grafana/grafana:11.0.0
          volumeMounts:
            - name: dashboard-provisioning
              mountPath: /etc/grafana/provisioning/dashboards
            - name: dashboard-files
              mountPath: /var/lib/grafana/dashboards
            - name: datasource-config
              mountPath: /etc/grafana/provisioning/datasources
      volumes:
        - name: dashboard-provisioning
          configMap:
            name: grafana-dashboard-provisioning
        - name: dashboard-files
          configMap:
            name: grafana-dashboard-json
        - name: datasource-config
          configMap:
            name: grafana-datasources
`
	reqs := DefaultGrafanaRequirements()
	result, err := ValidateGrafanaDeployment([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "multiple dashboard mounts should pass; failures: %v", result.Failures)
}

func TestQA_AlertRuleSyntax_CompleteConfig(t *testing.T) {
	yaml := `groups:
  - name: mem0
    rules:
      - alert: Mem0APIDown
        expr: up{job="mem0-api"} == 0
        for: 5m
        labels:
          severity: critical
          team: infra
        annotations:
          summary: "Mem0 API is down"
          description: "Mem0 API has been unreachable for 5 minutes"
      - alert: Mem0HighLatency
        expr: histogram_quantile(0.99, rate(mem0_request_duration_seconds_bucket[5m])) > 2
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Mem0 P99 latency above 2s"
  - name: k3s
    rules:
      - alert: K3sNodeNotReady
        expr: kube_node_status_condition{condition="Ready",status="true"} == 0
        for: 10m
        labels:
          severity: warning
      - alert: K3sPodCrashLooping
        expr: rate(kube_pod_container_status_restarts_total[15m]) > 0
        for: 5m
        labels:
          severity: critical
  - name: vllm
    rules:
      - alert: VLLMGPUOOMRisk
        expr: nvidia_gpu_memory_used_bytes / nvidia_gpu_memory_total_bytes > 0.95
        for: 5m
        labels:
          severity: critical
      - alert: VLLMHighQueueDepth
        expr: vllm_pending_requests > 50
        for: 2m
        labels:
          severity: warning
  - name: fleet-health
    rules:
      - alert: FleetNodeDown
        expr: fleet_node_up == 0
        for: 5m
        labels:
          severity: critical
      - alert: FleetDaemonStale
        expr: time() - fleet_last_check_timestamp > 300
        for: 10m
        labels:
          severity: warning
`
	reqs := DefaultAlertRuleRequirements()
	result, err := ValidateAlertRules([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "complete alert config should pass; failures: %v", result.Failures)
	t.Logf("Passed checks: %v", result.Passed)
}

func TestQA_RetentionConfig_LargeStorage(t *testing.T) {
	yaml := `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-data
  namespace: monitoring
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi
  storageClassName: local-path
`
	result, err := ValidateRetentionPV([]byte(yaml))
	require.NoError(t, err)
	assert.True(t, result.OK(), "100Gi should pass; failures: %v", result.Failures)
}

func TestQA_RetentionConfig_ExactMinimum(t *testing.T) {
	yaml := `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-data
  namespace: monitoring
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: local-path
`
	result, err := ValidateRetentionPV([]byte(yaml))
	require.NoError(t, err)
	assert.True(t, result.OK(), "10Gi should meet 10Gi minimum; failures: %v", result.Failures)
}

func TestQA_ParseStorageValues(t *testing.T) {
	tests := []struct {
		input  string
		wantGi int
	}{
		{"50Gi", 50},
		{"100Gi", 100},
		{"1Ti", 1024},
		{"5120Mi", 5},
		{"10Gi", 10},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseStorageGi(tt.input)
			assert.Equal(t, tt.wantGi, got)
		})
	}
}
