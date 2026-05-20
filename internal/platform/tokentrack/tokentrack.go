package tokentrack

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type UsageRecord struct {
	Timestamp    time.Time `json:"ts"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	RequestID    string    `json:"request_id,omitempty"`
	AgentID      string    `json:"agent_id,omitempty"`
	Cost         float64   `json:"cost_usd"`
}

type ModelSummary struct {
	Requests     int     `json:"requests"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost_usd"`
}

type DailySummary struct {
	Date              string                  `json:"date"`
	TotalRequests     int                     `json:"total_requests"`
	TotalInputTokens  int                     `json:"total_input_tokens"`
	TotalOutputTokens int                     `json:"total_output_tokens"`
	TotalCost         float64                 `json:"total_cost_usd"`
	ByModel           map[string]ModelSummary `json:"by_model"`
}

type costRate struct {
	InputPer1K  float64
	OutputPer1K float64
}

type Tracker struct {
	mu        sync.Mutex
	path      string
	costRates map[string]costRate
}

func NewTracker(path string) (*Tracker, error) {
	dir := dirOf(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create log dir: %w", err)
		}
	}
	return &Tracker{
		path: path,
		costRates: map[string]costRate{
			"MiniMax-M2.7-highspeed": {InputPer1K: 0.001, OutputPer1K: 0.002},
		},
	}, nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}

func (t *Tracker) SetCostRate(model string, inputPer1K, outputPer1K float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.costRates[model] = costRate{InputPer1K: inputPer1K, OutputPer1K: outputPer1K}
}

func (t *Tracker) computeCost(rec *UsageRecord) {
	rate, ok := t.costRates[rec.Model]
	if !ok {
		rate = costRate{InputPer1K: 0.001, OutputPer1K: 0.002}
	}
	rec.Cost = float64(rec.InputTokens)*rate.InputPer1K/1000 +
		float64(rec.OutputTokens)*rate.OutputPer1K/1000
}

func (t *Tracker) Record(rec UsageRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.computeCost(&rec)

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	f, err := os.OpenFile(t.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

func (t *Tracker) readRecords() ([]UsageRecord, error) {
	f, err := os.Open(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []UsageRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec UsageRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}

func (t *Tracker) DailySummary(date time.Time) (DailySummary, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	return t.summarize(dayStart, dayEnd, dayStart.Format("2006-01-02"))
}

func (t *Tracker) TotalSince(since time.Time) (DailySummary, error) {
	return t.summarize(since, time.Now().Add(time.Hour), since.Format("2006-01-02")+" to now")
}

func (t *Tracker) summarize(from, to time.Time, label string) (DailySummary, error) {
	records, err := t.readRecords()
	if err != nil {
		return DailySummary{}, err
	}

	summary := DailySummary{
		Date:    label,
		ByModel: make(map[string]ModelSummary),
	}

	for _, rec := range records {
		if rec.Timestamp.Before(from) || !rec.Timestamp.Before(to) {
			continue
		}
		summary.TotalRequests++
		summary.TotalInputTokens += rec.InputTokens
		summary.TotalOutputTokens += rec.OutputTokens
		summary.TotalCost += rec.Cost

		ms := summary.ByModel[rec.Model]
		ms.Requests++
		ms.InputTokens += rec.InputTokens
		ms.OutputTokens += rec.OutputTokens
		ms.TotalCost += rec.Cost
		summary.ByModel[rec.Model] = ms
	}

	return summary, nil
}
