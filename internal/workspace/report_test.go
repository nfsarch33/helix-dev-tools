package workspace

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWriteHumanReport_RedactsPathsAndShowsScore(t *testing.T) {
	report := ScoreReport(AuditReport{
		GeneratedAt: time.Date(2026, 5, 6, 17, 0, 0, 0, time.UTC),
		Repos: []RepoStatus{{
			Alias:  "global-kb",
			Path:   "/Users/example/Code/global-kb",
			Branch: "main",
			Findings: []Finding{{
				Code:    FindingDirtyWorktree,
				Message: "modified files present",
			}},
		}},
	})

	var buf bytes.Buffer
	if err := WriteHumanReport(&buf, report); err != nil {
		t.Fatalf("WriteHumanReport: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Score: 75") {
		t.Fatalf("missing score in %q", out)
	}
	if strings.Contains(out, "/Users/example") {
		t.Fatalf("human report leaked path: %q", out)
	}
	if !strings.Contains(out, "global-kb") {
		t.Fatalf("missing alias: %q", out)
	}
}

func TestWriteJSONReport_EncodesTier(t *testing.T) {
	report := ScoreReport(AuditReport{Repos: []RepoStatus{{
		Alias:    "repo",
		Findings: []Finding{{Code: FindingDirtyWorktree}},
	}}})

	var buf bytes.Buffer
	if err := WriteJSONReport(&buf, report); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}
	var decoded Score
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Tier != TierYellow {
		t.Fatalf("tier = %s, want %s", decoded.Tier, TierYellow)
	}
}
