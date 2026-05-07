package coordination

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// AlertmanagerWebhook is the subset of the Alertmanager webhook payload that
// cursor-tools needs to turn a firing SLO alert into a coordination blocker.
type AlertmanagerWebhook struct {
	Receiver          string              `json:"receiver"`
	Status            string              `json:"status"`
	GroupKey          string              `json:"groupKey"`
	CommonLabels      map[string]string   `json:"commonLabels"`
	CommonAnnotations map[string]string   `json:"commonAnnotations"`
	Alerts            []AlertmanagerAlert `json:"alerts"`
}

type AlertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
}

// AlertmanagerBlockerSignal builds a deterministic blocker signal from an
// Alertmanager webhook payload. It only considers firing alerts; resolved-only
// webhooks are still recorded as a low-priority status note.
func AlertmanagerBlockerSignal(payload AlertmanagerWebhook, sprint string, recordedAt time.Time) Signal {
	firing := firingAlerts(payload.Alerts)
	alerts := firing
	if len(alerts) == 0 {
		alerts = payload.Alerts
	}

	alertName := firstLabel(payload.CommonLabels, alerts, "alertname")
	severity := firstLabel(payload.CommonLabels, alerts, "severity")
	slo := firstLabel(payload.CommonLabels, alerts, "slo")
	summary := firstAnnotation(payload.CommonAnnotations, alerts, "summary")
	if summary == "" {
		summary = "Alertmanager notification received"
	}

	status := strings.TrimSpace(payload.Status)
	if status == "" {
		status = "unknown"
	}
	priority := "normal"
	if len(firing) > 0 && (severity == "critical" || severity == "emergency") {
		priority = "high"
	}

	parts := []string{fmt.Sprintf("Alertmanager %s", status)}
	if alertName != "" {
		parts = append(parts, alertName)
	}
	parts = append(parts, summary)
	if len(firing) > 1 {
		parts = append(parts, fmt.Sprintf("(%d firing alerts)", len(firing)))
	}

	return Signal{
		Type:     SignalBlocker,
		Machine:  LocalMachine(),
		Message:  strings.Join(parts, ": "),
		Priority: priority,
		Sprint:   sprint,
		Metadata: map[string]string{
			"source":         "alertmanager",
			"receiver":       payload.Receiver,
			"group_key":      payload.GroupKey,
			"alertname":      alertName,
			"severity":       severity,
			"slo":            slo,
			"status":         status,
			"alert_count":    fmt.Sprintf("%d", len(payload.Alerts)),
			"firing_count":   fmt.Sprintf("%d", len(firing)),
			"startup_prompt": "true",
			"recorded_at":    recordedAt.Format(time.RFC3339),
			"generator_urls": strings.Join(generatorURLs(alerts), ","),
		},
	}
}

func firingAlerts(alerts []AlertmanagerAlert) []AlertmanagerAlert {
	out := make([]AlertmanagerAlert, 0, len(alerts))
	for _, alert := range alerts {
		if strings.EqualFold(alert.Status, "firing") {
			out = append(out, alert)
		}
	}
	return out
}

func firstLabel(common map[string]string, alerts []AlertmanagerAlert, key string) string {
	if value := strings.TrimSpace(common[key]); value != "" {
		return value
	}
	for _, alert := range alerts {
		if value := strings.TrimSpace(alert.Labels[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstAnnotation(common map[string]string, alerts []AlertmanagerAlert, key string) string {
	if value := strings.TrimSpace(common[key]); value != "" {
		return value
	}
	for _, alert := range alerts {
		if value := strings.TrimSpace(alert.Annotations[key]); value != "" {
			return value
		}
	}
	return ""
}

func generatorURLs(alerts []AlertmanagerAlert) []string {
	set := map[string]struct{}{}
	for _, alert := range alerts {
		if alert.GeneratorURL == "" {
			continue
		}
		set[alert.GeneratorURL] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for url := range set {
		out = append(out, url)
	}
	sort.Strings(out)
	return out
}
