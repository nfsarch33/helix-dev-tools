package reportformat

import (
	"fmt"
	"strings"
	"time"
)

type Section struct {
	Heading string   `json:"heading"`
	Items   []string `json:"items"`
}

type Metric struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Report struct {
	Title    string    `json:"title"`
	Date     time.Time `json:"date"`
	Sections []Section `json:"sections"`
	Metrics  []Metric  `json:"metrics"`
	Notes    []string  `json:"notes"`
}

func NewReport(title string, date time.Time) *Report {
	return &Report{Title: title, Date: date}
}

func (r *Report) AddSection(heading string, items []string) {
	r.Sections = append(r.Sections, Section{Heading: heading, Items: items})
}

func (r *Report) AddMetric(name, value string) {
	r.Metrics = append(r.Metrics, Metric{Name: name, Value: value})
}

func (r *Report) AddNote(note string) {
	r.Notes = append(r.Notes, note)
}

func (r *Report) RenderMarkdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", r.Title)
	fmt.Fprintf(&b, "**Date:** %s\n\n", r.Date.Format("2006-01-02"))

	for _, sec := range r.Sections {
		fmt.Fprintf(&b, "## %s\n\n", sec.Heading)
		for _, item := range sec.Items {
			fmt.Fprintf(&b, "- %s\n", item)
		}
		b.WriteString("\n")
	}

	if len(r.Metrics) > 0 {
		b.WriteString("## Metrics\n\n")
		b.WriteString("| Metric | Value |\n")
		b.WriteString("|--------|-------|\n")
		for _, m := range r.Metrics {
			fmt.Fprintf(&b, "| %s | %s |\n", m.Name, m.Value)
		}
		b.WriteString("\n")
	}

	if len(r.Notes) > 0 {
		b.WriteString("## Notes\n\n")
		for _, note := range r.Notes {
			fmt.Fprintf(&b, "- %s\n", note)
		}
		b.WriteString("\n")
	}

	return b.String()
}
