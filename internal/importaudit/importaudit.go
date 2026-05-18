// Package importaudit scans Go source trees for files containing a specific
// import path prefix, enabling pre-migration audits before a module rename.
package importaudit

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AuditResult aggregates the output of a ScanDirectory call.
type AuditResult struct {
	// Files lists the paths of .go files that contain the old import prefix.
	Files []string
	// FileCount is the total number of .go files scanned.
	FileCount int
	// OldPathCount is the number of distinct .go files that reference the old prefix.
	OldPathCount int
}

// HasOldPaths returns true when at least one file references the old import
// path prefix.
func (r AuditResult) HasOldPaths() bool { return r.OldPathCount > 0 }

// ScanDirectory walks root recursively, scanning each .go file for lines
// containing oldPrefix. Returns an AuditResult summarising the findings.
func ScanDirectory(root, oldPrefix string) (AuditResult, error) {
	var result AuditResult

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		result.FileCount++

		found, scanErr := fileContains(path, oldPrefix)
		if scanErr != nil {
			return scanErr
		}
		if found {
			result.Files = append(result.Files, path)
			result.OldPathCount++
		}
		return nil
	})

	return result, err
}

// fileContains returns true when path contains at least one line that includes
// needle as a substring.
func fileContains(path, needle string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), needle) {
			return true, nil
		}
	}
	return false, scanner.Err()
}
