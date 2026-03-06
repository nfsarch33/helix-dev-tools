package learnings

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Pattern represents a parsed row from PATTERNS.md.
type Pattern struct {
	ID              string
	Description     string
	Confidence      float64
	Applications    int
	ApplicationsRaw string
	Category        string
	Created         string
	RawLine         string
}

var patternLineRe = regexp.MustCompile(
	`\|\s*(pat-\d+)\s*\|\s*(.+?)\s*\|\s*([\d.]+)\s*\|\s*(\S+)\s*\|\s*(\S+)\s*\|\s*(\S+)\s*\|`,
)

// ParsePatterns reads PATTERNS.md and returns a map of pat-ID -> Pattern.
func ParsePatterns(filepath string) (map[string]Pattern, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Pattern{}, nil
		}
		return nil, err
	}
	return parsePatternLines(string(data))
}

func parsePatternLines(content string) (map[string]Pattern, error) {
	result := make(map[string]Pattern)
	for _, line := range strings.Split(content, "\n") {
		m := patternLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		appsRaw := strings.TrimSpace(m[4])
		apps := 0
		cleaned := strings.TrimSuffix(appsRaw, "+")
		if n, err := strconv.Atoi(cleaned); err == nil {
			apps = n
		}
		conf, _ := strconv.ParseFloat(m[3], 64)
		result[m[1]] = Pattern{
			ID:              m[1],
			Description:     strings.TrimSpace(m[2]),
			Confidence:      conf,
			Applications:    apps,
			ApplicationsRaw: appsRaw,
			Category:        strings.TrimSpace(m[5]),
			Created:         strings.TrimSpace(m[6]),
			RawLine:         strings.TrimRight(line, "\n"),
		}
	}
	return result, nil
}

// Entry represents a section from ERRORS.md / LEARNINGS.md / FEATURE_REQUESTS.md.
type Entry struct {
	Date        string
	Category    string
	Content     string
	Fingerprint string
}

var entrySectionRe = regexp.MustCompile(`^## \[(.+?)\]\s*Category:\s*(\S+)`)

// ParseEntries reads an entry-style markdown file and returns sections.
func ParseEntries(filepath string) ([]Entry, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseEntrySections(string(data)), nil
}

func parseEntrySections(content string) []Entry {
	sectionRe := regexp.MustCompile(`(?m)^## \[`)
	parts := sectionRe.Split(content, -1)

	var entries []Entry
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fullSection := "## [" + part
		m := entrySectionRe.FindStringSubmatch(fullSection)
		if m == nil {
			continue
		}
		fp := strings.Join(strings.Fields(fullSection[:min(200, len(fullSection))]), " ")
		entries = append(entries, Entry{
			Date:        m[1],
			Category:    m[2],
			Content:     fullSection,
			Fingerprint: fp,
		})
	}
	return entries
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatPatternLine formats a Pattern as a markdown table row.
func FormatPatternLine(p Pattern) string {
	return fmt.Sprintf("| %s | %s | %.2f | %s | %s | %s | global |",
		p.ID, p.Description, p.Confidence, p.ApplicationsRaw, p.Category, p.Created)
}
