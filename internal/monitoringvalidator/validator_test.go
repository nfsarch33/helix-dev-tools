package monitoringvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validPromConfigMapYAML = `apiVersion: v1
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
          - targets: ['gpu-host-1:9100', 'gpu-host-2:9100']
      - job_name: llm-router
        static_configs:
          - targets: ['llm-router:9787']
`

func TestValidatePrometheusConfig_Valid(t *testing.T) {
	reqs := DefaultPrometheusRequirements()
	result, err := ValidatePrometheusConfig([]byte(validPromConfigMapYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid config should pass; failures: %v", result.Failures)
	assert.NotEmpty(t, result.Passed)
}

func TestValidatePrometheusConfig_MissingScrapeTarget(t *testing.T) {
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
          - targets: ['vllm-3090:9090']
`
	reqs := DefaultPrometheusRequirements()
	result, err := ValidatePrometheusConfig([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail with missing scrape targets")
}

func TestValidatePrometheusConfig_WrongNamespace(t *testing.T) {
	yaml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: default
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
      - job_name: vllm
        static_configs:
          - targets: ['vllm:9090']
      - job_name: node-exporter
        static_configs:
          - targets: ['gpu-host-1:9100']
      - job_name: llm-router
        static_configs:
          - targets: ['router:9787']
`
	reqs := DefaultPrometheusRequirements()
	result, err := ValidatePrometheusConfig([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK())
}

func TestValidatePrometheusConfig_EmptyData(t *testing.T) {
	yaml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitoring
data: {}
`
	reqs := DefaultPrometheusRequirements()
	result, err := ValidatePrometheusConfig([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK())
}

const validGrafanaYAML = `apiVersion: apps/v1
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
            - name: dashboards
              mountPath: /var/lib/grafana/dashboards
            - name: datasources
              mountPath: /etc/grafana/provisioning/datasources
      volumes:
        - name: dashboards
          configMap:
            name: grafana-dashboards
        - name: datasources
          configMap:
            name: grafana-datasources
`

func TestValidateGrafanaDeployment_Valid(t *testing.T) {
	reqs := DefaultGrafanaRequirements()
	result, err := ValidateGrafanaDeployment([]byte(validGrafanaYAML), reqs)
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid grafana should pass; failures: %v", result.Failures)
}

func TestValidateGrafanaDeployment_NoDashboardMount(t *testing.T) {
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
            - name: datasources
              mountPath: /etc/grafana/provisioning/datasources
      volumes:
        - name: datasources
          configMap:
            name: grafana-datasources
`
	reqs := DefaultGrafanaRequirements()
	result, err := ValidateGrafanaDeployment([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail without dashboard provisioning")
}

func TestValidateGrafanaDeployment_NoDataSourceMount(t *testing.T) {
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
            - name: dashboards
              mountPath: /var/lib/grafana/dashboards
      volumes:
        - name: dashboards
          configMap:
            name: grafana-dashboards
`
	reqs := DefaultGrafanaRequirements()
	result, err := ValidateGrafanaDeployment([]byte(yaml), reqs)
	require.NoError(t, err)
	assert.False(t, result.OK(), "should fail without datasource config")
}

const validRetentionPVYAML = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: prometheus-data
  namespace: monitoring
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: local-path
`

func TestValidateRetentionPV_Valid(t *testing.T) {
	result, err := ValidateRetentionPV([]byte(validRetentionPVYAML))
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid PVC should pass; failures: %v", result.Failures)
}

func TestValidateRetentionPV_TooSmall(t *testing.T) {
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
      storage: 5Gi
  storageClassName: local-path
`
	result, err := ValidateRetentionPV([]byte(yaml))
	require.NoError(t, err)
	assert.False(t, result.OK(), "5Gi PVC should fail minimum check")
}
