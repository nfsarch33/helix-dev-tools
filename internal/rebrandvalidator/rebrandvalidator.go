package rebrandvalidator

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Finding represents a single legacy-term occurrence in a scanned file.
type Finding struct {
	FilePath   string `json:"file_path"`
	Line       string `json:"line"`
	LegacyTerm string `json:"legacy_term"`
	Context    string `json:"context"`
	LineNumber int    `json:"line_number"`
}

// AllowlistEntry suppresses a specific legacy term in files matching a glob
// pattern. If Term matches a finding's LegacyTerm and File glob matches the
// file path, the finding is suppressed rather than reported.
type AllowlistEntry struct {
	File string `yaml:"file"`
	Term string `yaml:"term"`
}

// allowlistFile is the YAML shape of .rebrand-allowlist.yaml.
type allowlistFile struct {
	Entries []AllowlistEntry `yaml:"entries"`
}

// LoadAllowlistYAML reads a .rebrand-allowlist.yaml file and returns its
// entries. A missing file is not an error -- allowlists are opt-in.
func LoadAllowlistYAML(path string) ([]AllowlistEntry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var af allowlistFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, err
	}
	return af.Entries, nil
}

// isSuppressed reports whether a finding should be suppressed by the allowlist.
func isSuppressed(filePath, term string, allowlist []AllowlistEntry) bool {
	base := filepath.Base(filePath)
	for _, e := range allowlist {
		if e.Term != term {
			continue
		}
		matched, err := filepath.Match(e.File, base)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// ScanResult aggregates findings from a directory scan.
type ScanResult struct {
	Findings        []Finding `json:"findings"`
	FileCount       int       `json:"file_count"`
	FindingCount    int       `json:"finding_count"`
	SuppressedCount int       `json:"suppressed_count"`
}

// HasFindings reports whether the scan produced any non-suppressed findings.
func (r ScanResult) HasFindings() bool { return r.FindingCount > 0 }

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
// Allowlist entries suppress specific term+file-glob combinations: suppressed
// findings increment SuppressedCount but are not added to Findings.
func ScanDirectory(root string, excludeDirs []string, allowlist []AllowlistEntry) (ScanResult, error) {
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

		for _, f := range findings {
			if isSuppressed(f.FilePath, f.LegacyTerm, allowlist) {
				result.SuppressedCount++
			} else {
				result.Findings = append(result.Findings, f)
				result.FindingCount++
			}
		}

		return nil
	})

	return result, err
}
