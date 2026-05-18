package grafana

import (
	"encoding/json"
	"fmt"
)

// PanelType identifies the Grafana panel visualisation type
type PanelType string

const (
	PanelTimeSeries PanelType = "timeseries"
	PanelGauge      PanelType = "gauge"
	PanelStat       PanelType = "stat"
	PanelTable      PanelType = "table"
)

// Panel is one visualisation panel in a dashboard
type Panel struct {
	ID    int
	Title string
	Type  PanelType
	Query string
}

// Dashboard is a Grafana dashboard template
type Dashboard struct {
	Title   string
	UID     string
	Panels  []Panel
}

// NewDashboard creates a dashboard with the given title and uid
func NewDashboard(title, uid string) *Dashboard {
	return &Dashboard{Title: title, UID: uid}
}

// AddPanel appends a panel to the dashboard
func (d *Dashboard) AddPanel(p Panel) {
	if p.ID == 0 {
		p.ID = len(d.Panels) + 1
	}
	d.Panels = append(d.Panels, p)
}

// Validate returns errors for any panel missing Title or Query
func (d *Dashboard) Validate() []error {
	var errs []error
	for _, p := range d.Panels {
		if p.Title == "" {
			errs = append(errs, fmt.Errorf("panel %d missing title", p.ID))
		}
		if p.Query == "" {
			errs = append(errs, fmt.Errorf("panel %d missing query", p.ID))
		}
	}
	return errs
}

// ToJSON serialises the dashboard to JSON
func (d *Dashboard) ToJSON() ([]byte, error) {
	type dashJSON struct {
		Title  string  `json:"title"`
		UID    string  `json:"uid"`
		Panels []Panel `json:"panels"`
	}
	return json.Marshal(dashJSON{Title: d.Title, UID: d.UID, Panels: d.Panels})
}
