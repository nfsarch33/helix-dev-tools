package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func WriteHumanReport(w io.Writer, score Score) error {
	if _, err := fmt.Fprintf(w, "Workspace cleanliness\nScore: %d (%s)\nFindings: %d\n\n", score.Score, score.Tier, score.Findings); err != nil {
		return err
	}
	for _, repo := range score.Repos {
		if _, err := fmt.Fprintf(w, "%-28s branch=%-24s ahead=%d behind=%d findings=%d\n", repo.Alias, repo.Branch, repo.Ahead, repo.Behind, len(repo.Findings)); err != nil {
			return err
		}
		for _, finding := range repo.Findings {
			if _, err := fmt.Fprintf(w, "  - %-22s %s\n", finding.Code, sanitizeMessage(finding.Message)); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteJSONReport(w io.Writer, score Score) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(score)
}

func WriteNDJSONReport(w io.Writer, score Score) error {
	raw, err := json.Marshal(score)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(raw))
	return err
}

func sanitizeMessage(message string) string {
	fields := strings.Fields(message)
	for i, field := range fields {
		if strings.HasPrefix(field, "/") || strings.Contains(field, "/Users/") || strings.Contains(field, "/home/") {
			fields[i] = "<path>"
		}
	}
	return strings.Join(fields, " ")
}
