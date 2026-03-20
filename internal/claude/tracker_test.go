package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageHasTokenCounts(t *testing.T) {
	tests := []struct {
		name   string
		usage  Usage
		expect bool
	}{
		{"empty", Usage{}, false},
		{"input only", Usage{InputTokens: 100}, true},
		{"output only", Usage{OutputTokens: 50}, true},
		{"both", Usage{InputTokens: 100, OutputTokens: 50}, true},
		{"proxy only", Usage{PromptBytes: 500, OutputBytes: 200}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.usage.HasTokenCounts(); got != tt.expect {
				t.Errorf("HasTokenCounts() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestAppendAndReadUsage(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	u := Usage{
		Timestamp:   now,
		Model:       "test-model",
		PromptBytes: 100,
		OutputBytes: 200,
		DurationMs:  1500,
		Backend:     "bedrock",
	}

	if err := AppendUsage(dir, u); err != nil {
		t.Fatalf("AppendUsage: %v", err)
	}

	expectedPath := UsageFilePath(dir, now)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected file %s to exist: %v", expectedPath, err)
	}

	records, err := ReadUsage(expectedPath)
	if err != nil {
		t.Fatalf("ReadUsage: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Model != "test-model" {
		t.Errorf("Model = %q, want %q", records[0].Model, "test-model")
	}
	if records[0].PromptBytes != 100 {
		t.Errorf("PromptBytes = %d, want 100", records[0].PromptBytes)
	}
}

func TestAppendMultiple(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		u := Usage{
			Timestamp:   now.Add(time.Duration(i) * time.Hour),
			PromptBytes: (i + 1) * 100,
			DurationMs:  int64(i * 500),
			Backend:     "bedrock",
		}
		if err := AppendUsage(dir, u); err != nil {
			t.Fatalf("AppendUsage[%d]: %v", i, err)
		}
	}

	records, err := ReadUsage(UsageFilePath(dir, now))
	if err != nil {
		t.Fatalf("ReadUsage: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

func TestReadUsageNotExist(t *testing.T) {
	records, err := ReadUsage(filepath.Join(t.TempDir(), "nonexistent.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil, got %d records", len(records))
	}
}

func TestDetectBackend(t *testing.T) {
	orig := LookupEnv
	defer func() { LookupEnv = orig }()

	t.Run("bedrock", func(t *testing.T) {
		LookupEnv = func(key string) (string, bool) {
			if key == "CLAUDE_CODE_USE_BEDROCK" {
				return "true", true
			}
			return "", false
		}
		if got := detectBackend(); got != "bedrock" {
			t.Errorf("detectBackend() = %q, want %q", got, "bedrock")
		}
	})

	t.Run("anthropic", func(t *testing.T) {
		LookupEnv = func(key string) (string, bool) {
			if key == "ANTHROPIC_API_KEY" {
				return "sk-test", true
			}
			return "", false
		}
		if got := detectBackend(); got != "anthropic" {
			t.Errorf("detectBackend() = %q, want %q", got, "anthropic")
		}
	})

	t.Run("unknown", func(t *testing.T) {
		LookupEnv = func(key string) (string, bool) { return "", false }
		if got := detectBackend(); got != "unknown" {
			t.Errorf("detectBackend() = %q, want %q", got, "unknown")
		}
	})
}

func TestParseTokenInfo(t *testing.T) {
	tests := []struct {
		name         string
		stderr       string
		expectInput  int
		expectOutput int
		expectCost   float64
	}{
		{
			"with tokens",
			"1,234 input tokens\n567 output tokens\n$0.0045",
			1234, 567, 0.0045,
		},
		{
			"no token info",
			"some random output",
			0, 0, 0,
		},
		{
			"cache tokens",
			"100 cache read tokens\n50 cache write tokens",
			0, 0, 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var u Usage
			parseTokenInfo(tt.stderr, &u)
			if u.InputTokens != tt.expectInput {
				t.Errorf("InputTokens = %d, want %d", u.InputTokens, tt.expectInput)
			}
			if u.OutputTokens != tt.expectOutput {
				t.Errorf("OutputTokens = %d, want %d", u.OutputTokens, tt.expectOutput)
			}
			if u.Cost != tt.expectCost {
				t.Errorf("Cost = %f, want %f", u.Cost, tt.expectCost)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(%q,10) = %q", "short", got)
	}
	long := "a very long string that exceeds the limit"
	got := truncate(long, 10)
	if len(got) != 13 { // 10 + "..."
		t.Errorf("truncate length = %d, want 13", len(got))
	}
}

func TestUsageFilePath(t *testing.T) {
	ts := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	got := UsageFilePath("/tmp/usage", ts)
	want := "/tmp/usage/2026-03-20.jsonl"
	if got != want {
		t.Errorf("UsageFilePath = %q, want %q", got, want)
	}
}
