package loggingvalidator

// ValidationResult collects pass/fail findings from a logging manifest check.
type ValidationResult struct {
	Name     string
	Passed   []string
	Failures []string
}

func (v *ValidationResult) OK() bool { return len(v.Failures) == 0 }

// LokiRequirements specifies expected Loki StatefulSet properties.
type LokiRequirements struct {
	Namespace         string
	MinStorageGi      int
	RequireRetention  bool
	RetentionPeriod   string // e.g. "168h" (7 days)
	RequirePersistence bool
}

// DefaultLokiRequirements returns defaults for the fleet logging stack.
func DefaultLokiRequirements() LokiRequirements {
	return LokiRequirements{
		Namespace:         "logging",
		MinStorageGi:      10,
		RequireRetention:  true,
		RetentionPeriod:   "168h",
		RequirePersistence: true,
	}
}

// PromtailRequirements specifies expected Promtail DaemonSet properties.
type PromtailRequirements struct {
	Namespace            string
	RequireHostLogMount  bool
	RequirePodLogMount   bool
	RequireLokiEndpoint  bool
	LokiEndpoint         string
}

// DefaultPromtailRequirements returns defaults for the fleet logging stack.
func DefaultPromtailRequirements() PromtailRequirements {
	return PromtailRequirements{
		Namespace:           "logging",
		RequireHostLogMount: true,
		RequirePodLogMount:  true,
		RequireLokiEndpoint: true,
		LokiEndpoint:        "http://loki:3100/loki/api/v1/push",
	}
}

// LogRetentionRequirements specifies log retention policy properties.
type LogRetentionRequirements struct {
	MaxRetentionDays int
	MinRetentionDays int
	RequireCompaction bool
}

// DefaultLogRetentionRequirements returns defaults.
func DefaultLogRetentionRequirements() LogRetentionRequirements {
	return LogRetentionRequirements{
		MaxRetentionDays:  30,
		MinRetentionDays:  7,
		RequireCompaction: true,
	}
}

// LogQueryRequirements specifies log query validation properties.
type LogQueryRequirements struct {
	RequiredLabels    []string
	ForbiddenPatterns []string
	MaxRangeHours     int
}

// DefaultLogQueryRequirements returns defaults.
func DefaultLogQueryRequirements() LogQueryRequirements {
	return LogQueryRequirements{
		RequiredLabels:    []string{"namespace", "pod"},
		ForbiddenPatterns: []string{`.*`, `.+`},
		MaxRangeHours:     168,
	}
}
