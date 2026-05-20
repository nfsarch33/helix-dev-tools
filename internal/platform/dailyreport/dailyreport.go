package dailyreport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type Config struct {
	TokenUsagePath  string
	AgentTracePath  string
	SprintboardPath string
}

type tokenRecord struct {
	Timestamp    time.Time `json:"ts"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Cost         float64   `json:"cost_usd"`
}

type traceRecord struct {
	Timestamp  time.Time `json:"ts"`
	Tool       string    `json:"tool"`
	DurationMs int       `json:"duration_ms"`
}

type Generator struct {
	cfg Config
}

func NewGenerator(cfg Config) *Generator {
	return &Generator{cfg: cfg}
}

func (g *Generator) Generate(day time.Time) (string, error) {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	tokens := g.readTokenRecords(dayStart, dayEnd)
	traces := g.readTraceRecords(dayStart, dayEnd)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Daily Agent Status Report\n\n**Date:** %s\n\n", dayStart.Format("2006-01-02")))

	g.writeTokenSection(&sb, tokens)
	g.writeToolSection(&sb, traces)
	g.writeSprintboardSection(&sb)

	return sb.String(), nil
}

func (g *Generator) readTokenRecords(from, to time.Time) []tokenRecord {
	return readNDJSON[tokenRecord](g.cfg.TokenUsagePath, from, to)
}

func (g *Generator) readTraceRecords(from, to time.Time) []traceRecord {
	return readNDJSON[traceRecord](g.cfg.AgentTracePath, from, to)
}

type timestamped interface {
	getTimestamp() time.Time
}

func (r tokenRecord) getTimestamp() time.Time { return r.Timestamp }
func (r traceRecord) getTimestamp() time.Time { return r.Timestamp }

func readNDJSON[T timestamped](path string, from, to time.Time) []T {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []T
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec T
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		ts := rec.getTimestamp()
		if !ts.Before(from) && ts.Before(to) {
			results = append(results, rec)
		}
	}
	return results
}

func (g *Generator) writeTokenSection(sb *strings.Builder, records []tokenRecord) {
	sb.WriteString("## Token Usage\n\n")
	if len(records) == 0 {
		sb.WriteString("No token usage recorded.\n\n")
		return
	}

	var totalIn, totalOut int
	var totalCost float64
	byModel := make(map[string][3]float64)

	for _, r := range records {
		totalIn += r.InputTokens
		totalOut += r.OutputTokens
		totalCost += r.Cost
		entry := byModel[r.Model]
		entry[0] += float64(r.InputTokens)
		entry[1] += float64(r.OutputTokens)
		entry[2] += r.Cost
		byModel[r.Model] = entry
	}

	sb.WriteString(fmt.Sprintf("| Metric | Value |\n|---|---|\n"))
	sb.WriteString(fmt.Sprintf("| Total requests | %d |\n", len(records)))
	sb.WriteString(fmt.Sprintf("| Input tokens | %d |\n", totalIn))
	sb.WriteString(fmt.Sprintf("| Output tokens | %d |\n", totalOut))
	sb.WriteString(fmt.Sprintf("| Estimated cost | $%.4f |\n\n", totalCost))

	if len(byModel) > 0 {
		sb.WriteString("**By model:**\n\n")
		sb.WriteString("| Model | Requests | In | Out | Cost |\n|---|---|---|---|---|\n")
		models := sortedKeys(byModel)
		for _, m := range models {
			count := 0
			for _, r := range records {
				if r.Model == m {
					count++
				}
			}
			v := byModel[m]
			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | $%.4f |\n", m, count, int(v[0]), int(v[1]), v[2]))
		}
		sb.WriteString("\n")
	}
}

func (g *Generator) writeToolSection(sb *strings.Builder, records []traceRecord) {
	sb.WriteString("## Tool Activity\n\n")
	if len(records) == 0 {
		sb.WriteString("No tool activity recorded.\n\n")
		return
	}

	toolCounts := make(map[string]int)
	toolDuration := make(map[string]int)
	for _, r := range records {
		toolCounts[r.Tool]++
		toolDuration[r.Tool] += r.DurationMs
	}

	sb.WriteString("| Tool | Calls | Total ms |\n|---|---|---|\n")
	tools := sortedKeysStr(toolCounts)
	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("| %s | %d | %d |\n", tool, toolCounts[tool], toolDuration[tool]))
	}
	sb.WriteString("\n")
}

func (g *Generator) writeSprintboardSection(sb *strings.Builder) {
	sb.WriteString("## Sprint Status\n\n")
	if g.cfg.SprintboardPath == "" {
		sb.WriteString("No sprintboard configured.\n\n")
		return
	}
	sb.WriteString(fmt.Sprintf("Sprintboard: %s\n\n", g.cfg.SprintboardPath))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysStr(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
