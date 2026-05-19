package observe

import (
	"fmt"
	"strings"
	"time"
)

type DashboardPanel struct {
	Title  string
	Metric string
	Value  float64
	Unit   string
	Status string
}

type Dashboard struct {
	Title       string
	Panels      []DashboardPanel
	GeneratedAt time.Time
}

func NewDashboard(title string) *Dashboard {
	return &Dashboard{Title: title, GeneratedAt: time.Now()}
}

func (d *Dashboard) AddPanel(panel DashboardPanel) {
	d.Panels = append(d.Panels, panel)
}

func (d *Dashboard) Render() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s ===\n", d.Title))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", d.GeneratedAt.Format(time.RFC3339)))

	for _, p := range d.Panels {
		status := p.Status
		if status == "" {
			status = "OK"
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s: %.1f %s\n", status, p.Title, p.Value, p.Unit))
	}
	return sb.String()
}

func (d *Dashboard) PanelCount() int {
	return len(d.Panels)
}

func (d *Dashboard) StatusSummary() map[string]int {
	summary := make(map[string]int)
	for _, p := range d.Panels {
		status := p.Status
		if status == "" {
			status = "OK"
		}
		summary[status]++
	}
	return summary
}
