package dailyreport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateWithNoData(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token-usage.ndjson")
	tracePath := filepath.Join(dir, "agenttrace.ndjson")
	os.WriteFile(tokenPath, nil, 0644)
	os.WriteFile(tracePath, nil, 0644)

	gen := NewGenerator(Config{
		TokenUsagePath:  tokenPath,
		AgentTracePath:  tracePath,
		SprintboardPath: "",
	})
	report, err := gen.Generate(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "# Daily Agent Status Report") {
		t.Error("missing report header")
	}
}

func TestGenerateIncludesTokenCosts(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token-usage.ndjson")
	tracePath := filepath.Join(dir, "agenttrace.ndjson")
	os.WriteFile(tokenPath, []byte(
		`{"ts":"2026-05-20T10:00:00Z","model":"MiniMax-M2.7-highspeed","provider":"minimax","input_tokens":5000,"output_tokens":2000,"cost_usd":0.009}`+"\n"+
			`{"ts":"2026-05-20T11:00:00Z","model":"MiniMax-M2.7-highspeed","provider":"minimax","input_tokens":3000,"output_tokens":1000,"cost_usd":0.005}`+"\n",
	), 0644)
	os.WriteFile(tracePath, nil, 0644)

	gen := NewGenerator(Config{
		TokenUsagePath:  tokenPath,
		AgentTracePath:  tracePath,
		SprintboardPath: "",
	})
	report, err := gen.Generate(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "8,000") && !strings.Contains(report, "8000") {
		t.Errorf("report should show total input tokens: %s", report)
	}
}

func TestGenerateIncludesToolCounts(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token-usage.ndjson")
	tracePath := filepath.Join(dir, "agenttrace.ndjson")
	os.WriteFile(tokenPath, nil, 0644)
	os.WriteFile(tracePath, []byte(
		`{"ts":"2026-05-20T09:00:00Z","tool":"shell","duration_ms":150}`+"\n"+
			`{"ts":"2026-05-20T09:01:00Z","tool":"shell","duration_ms":200}`+"\n"+
			`{"ts":"2026-05-20T09:02:00Z","tool":"read","duration_ms":50}`+"\n",
	), 0644)

	gen := NewGenerator(Config{
		TokenUsagePath:  tokenPath,
		AgentTracePath:  tracePath,
		SprintboardPath: "",
	})
	report, err := gen.Generate(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "shell") {
		t.Errorf("report should mention shell tool: %s", report)
	}
}

func TestGenerateFormatsMarkdown(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token-usage.ndjson")
	tracePath := filepath.Join(dir, "agenttrace.ndjson")
	os.WriteFile(tokenPath, nil, 0644)
	os.WriteFile(tracePath, nil, 0644)

	gen := NewGenerator(Config{
		TokenUsagePath: tokenPath,
		AgentTracePath: tracePath,
	})
	report, _ := gen.Generate(time.Now())
	if !strings.Contains(report, "## Token Usage") {
		t.Error("missing token usage section")
	}
	if !strings.Contains(report, "## Tool Activity") {
		t.Error("missing tool activity section")
	}
}

func TestGenerateMissingFiles(t *testing.T) {
	gen := NewGenerator(Config{
		TokenUsagePath:  "/nonexistent/token.ndjson",
		AgentTracePath:  "/nonexistent/trace.ndjson",
		SprintboardPath: "",
	})
	report, err := gen.Generate(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "# Daily Agent Status Report") {
		t.Error("should still produce a report with missing files")
	}
}

func TestGenerateFiltersByDate(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token-usage.ndjson")
	tracePath := filepath.Join(dir, "agenttrace.ndjson")
	os.WriteFile(tokenPath, []byte(
		`{"ts":"2026-05-19T10:00:00Z","model":"old","provider":"x","input_tokens":9999,"output_tokens":9999,"cost_usd":99.0}`+"\n"+
			`{"ts":"2026-05-20T10:00:00Z","model":"today","provider":"x","input_tokens":100,"output_tokens":50,"cost_usd":0.001}`+"\n",
	), 0644)
	os.WriteFile(tracePath, nil, 0644)

	gen := NewGenerator(Config{
		TokenUsagePath: tokenPath,
		AgentTracePath: tracePath,
	})
	report, err := gen.Generate(time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(report, "9999") || strings.Contains(report, "9,999") {
		t.Error("should not include previous day data")
	}
}
