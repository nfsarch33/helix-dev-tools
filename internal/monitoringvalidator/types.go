package monitoringvalidator

// ValidationResult collects pass/fail findings from a monitoring manifest check.
type ValidationResult struct {
	Name     string
	Passed   []string
	Failures []string
}

func (v *ValidationResult) OK() bool { return len(v.Failures) == 0 }

// PrometheusRequirements specifies expected Prometheus ConfigMap properties.
type PrometheusRequirements struct {
	RequiredScrapeTargets []string
	MinScrapeInterval     string
	RequireRetentionPV    bool
	Namespace             string
}

// DefaultPrometheusRequirements returns defaults for the fleet monitoring stack.
func DefaultPrometheusRequirements() PrometheusRequirements {
	return PrometheusRequirements{
		RequiredScrapeTargets: []string{"vllm", "node-exporter", "llm-router"},
		MinScrapeInterval:     "30s",
		RequireRetentionPV:    true,
		Namespace:             "monitoring",
	}
}

// GrafanaRequirements specifies expected Grafana deployment properties.
type GrafanaRequirements struct {
	RequireDashboardProvisioning bool
	RequireDataSourceConfig      bool
	Namespace                    string
}

// DefaultGrafanaRequirements returns defaults.
func DefaultGrafanaRequirements() GrafanaRequirements {
	return GrafanaRequirements{
		RequireDashboardProvisioning: true,
		RequireDataSourceConfig:      true,
		Namespace:                    "monitoring",
	}
}

// AlertRuleRequirements specifies expected alert rule properties.
type AlertRuleRequirements struct {
	RequiredAlertGroups []string
	RequireLabels       []string
}

// DefaultAlertRuleRequirements returns defaults for fleet alerting.
func DefaultAlertRuleRequirements() AlertRuleRequirements {
	return AlertRuleRequirements{
		RequiredAlertGroups: []string{"mem0", "k3s", "vllm", "fleet-health"},
		RequireLabels:       []string{"severity"},
	}
}
