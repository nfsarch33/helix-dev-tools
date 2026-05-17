package rebrandvalidator

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Finding represents a single legacy-term occurrence in a scanned file.
type Finding struct {
	FilePath   string `json:"file_path"`
	Line       string `json:"line"`
	LegacyTerm string `json:"legacy_term"`
	Context    string `json:"context"`
	LineNumber int    `json:"line_number"`
}

// ScanResult aggregates findings from a directory scan.
type ScanResult struct {
	Findings     []Finding `json:"findings"`
	FileCount    int       `json:"file_count"`
	FindingCount int       `json:"finding_count"`
}

// DefaultLegacyTerms returns the canonical list of terms that must be rebranded.
func DefaultLegacyTerms() []string {
	return []string{
		"ironclaw",
		"cursor-tools",
		"cursor-global-kb",
		"cursor_tools",
		"cylrl",
		"IronClaw",
		"IRONCLAW",
	}
}

// ScanFile scans a single file for legacy term occurrences.
// Binary files (containing null bytes in the first 512 bytes) are skipped.
func ScanFile(path string, terms []string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	peek := data
	if len(peek) > 512 {
		peek = peek[:512]
	}
	if bytes.ContainsRune(peek, 0) {
		return nil, nil
	}

	var findings []Finding
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, term := range terms {
			if strings.Contains(line, term) {
				findings = append(findings, Finding{
					FilePath:   path,
					Line:       line,
					LegacyTerm: term,
					Context:    line,
					LineNumber: lineNum,
				})
			}
		}
	}

	return findings, scanner.Err()
}

// ScanDirectory walks root recursively, scanning each file for legacy terms.
// Directories whose base name appears in excludeDirs are skipped entirely.
func ScanDirectory(root string, excludeDirs []string) (ScanResult, error) {
	excluded := make(map[string]bool, len(excludeDirs))
	for _, d := range excludeDirs {
		excluded[d] = true
	}

	var result ScanResult
	terms := DefaultLegacyTerms()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if path != root && excluded[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		result.FileCount++

		findings, scanErr := ScanFile(path, terms)
		if scanErr != nil {
			return scanErr
		}
		result.Findings = append(result.Findings, findings...)
		result.FindingCount += len(findings)

		return nil
	})

	return result, err
}
