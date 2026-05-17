package dockerhealth

// ContainerState represents the health state of a Docker container.
type ContainerState struct {
	Name          string
	Image         string
	Status        string
	Health        string
	RestartPolicy string
	Running       bool
	RestartCount  int
}

// HealthCheckResult holds the outcome of a Docker health check.
type HealthCheckResult struct {
	Host       string
	Containers []ContainerState
	Healthy    int
	Unhealthy  int
	Total      int
	Errors     []string
}

// AlertRule represents a Prometheus alerting rule.
type AlertRule struct {
	Alert       string
	Expr        string
	For         string
	Labels      map[string]string
	Annotations map[string]string
}

// AlertRuleFile represents a Prometheus alert rules YAML file.
type AlertRuleFile struct {
	Groups []AlertRuleGroup
}

// AlertRuleGroup holds a named group of alert rules.
type AlertRuleGroup struct {
	Name  string
	Rules []AlertRule
}
