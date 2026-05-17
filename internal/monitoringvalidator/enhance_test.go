package monitoringvalidator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validGrafanaDashboardJSON = `{
  "id": null,
  "uid": "fleet-overview",
  "title": "Fleet Overview",
  "tags": ["fleet", "k8s"],
  "timezone": "browser",
  "panels": [
    {
      "id": 1,
      "type": "graph",
      "title": "GPU Memory Usage",
      "targets": [
        {
          "expr": "nvidia_gpu_memory_used_bytes / nvidia_gpu_memory_total_bytes * 100",
          "legendFormat": "{{instance}}"
        }
      ],
      "datasource": "Prometheus"
    },
    {
      "id": 2,
      "type": "stat",
      "title": "Node Count",
      "targets": [
        {
          "expr": "count(kube_node_info)",
          "legendFormat": "nodes"
        }
      ],
      "datasource": "Prometheus"
    }
  ],
  "templating": {
    "list": []
  },
  "schemaVersion": 39,
  "version": 1
}`

func TestValidateDashboardJSON_Valid(t *testing.T) {
	result, err := ValidateDashboardJSON([]byte(validGrafanaDashboardJSON))
	require.NoError(t, err)
	assert.True(t, result.OK(), "valid dashboard should pass; failures: %v", result.Failures)
}

func TestValidateDashboardJSON_MissingTitle(t *testing.T) {
	json := `{
  "uid": "test-dashboard",
  "panels": [
    {
      "id": 1,
      "type": "graph",
      "title": "Test",
      "targets": [{"expr": "up"}],
      "datasource": "Prometheus"
    }
  ],
  "schemaVersion": 39
}`
	result, err := ValidateDashboardJSON([]byte(json))
	require.NoError(t, err)
	assert.False(t, result.OK())
}

func TestValidateDashboardJSON_NoPanels(t *testing.T) {
	json := `{
  "title": "Empty Dashboard",
  "uid": "empty",
  "panels": [],
  "schemaVersion": 39
}`
	result, err := ValidateDashboardJSON([]byte(json))
	require.NoError(t, err)
	assert.False(t, result.OK())
}

func TestValidateDashboardJSON_PanelMissingDatasource(t *testing.T) {
	json := `{
  "title": "Test Dashboard",
  "uid": "test",
  "panels": [
    {
      "id": 1,
      "type": "graph",
      "title": "Test Panel",
      "targets": [{"expr": "up"}]
    }
  ],
  "schemaVersion": 39
}`
	result, err := ValidateDashboardJSON([]byte(json))
	require.NoError(t, err)
	assert.False(t, result.OK())
}

func TestValidateScrapeTargets_AllPresent(t *testing.T) {
	promConfig := `global:
  scrape_interval: 15s
scrape_configs:
  - job_name: vllm
    static_configs:
      - targets: ['vllm-3090:8000']
  - job_name: node-exporter
    static_configs:
      - targets: ['host1:9100']
  - job_name: llm-router
    static_configs:
      - targets: ['router:9787']
`
	expected := []string{"vllm", "node-exporter", "llm-router"}
	result, err := ValidateScrapeTargets([]byte(promConfig), expected)
	require.NoError(t, err)
	assert.True(t, result.OK(), "all targets present; failures: %v", result.Failures)
}

func TestValidateScrapeTargets_MissingTargets(t *testing.T) {
	promConfig := `global:
  scrape_interval: 15s
scrape_configs:
  - job_name: vllm
    static_configs:
      - targets: ['vllm-3090:8000']
`
	expected := []string{"vllm", "node-exporter", "mem0-api"}
	result, err := ValidateScrapeTargets([]byte(promConfig), expected)
	require.NoError(t, err)
	assert.False(t, result.OK())
	assert.GreaterOrEqual(t, len(result.Failures), 2)
}

func TestValidateScrapeTargets_EmptyConfig(t *testing.T) {
	promConfig := `global:
  scrape_interval: 15s
scrape_configs: []
`
	expected := []string{"vllm"}
	result, err := ValidateScrapeTargets([]byte(promConfig), expected)
	require.NoError(t, err)
	assert.False(t, result.OK())
}
