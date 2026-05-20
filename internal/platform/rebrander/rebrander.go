package rebrander

import (
	"os"
	"path/filepath"
	"strings"
)

type Rule struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type Finding struct {
	Path  string `json:"path"`
	Rule  Rule   `json:"rule"`
	Count int    `json:"count"`
}

type Scanner struct {
	Rules []Rule
}

func NewScanner(rules []Rule) *Scanner {
	return &Scanner{Rules: rules}
}

func (s *Scanner) Scan(root string) ([]Finding, error) {
	var findings []Finding
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if isBinary(data) {
			return nil
		}
		content := string(data)
		for _, rule := range s.Rules {
			count := strings.Count(content, rule.Old)
			if count > 0 {
				findings = append(findings, Finding{
					Path:  path,
					Rule:  rule,
					Count: count,
				})
			}
		}
		return nil
	})
	return findings, err
}

func (s *Scanner) Apply(path string, rule Rule) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	content := string(data)
	count := strings.Count(content, rule.Old)
	if count == 0 {
		return 0, nil
	}
	replaced := strings.ReplaceAll(content, rule.Old, rule.New)
	if err := os.WriteFile(path, []byte(replaced), 0644); err != nil {
		return 0, err
	}
	return count, nil
}

func isBinary(data []byte) bool {
	checkLen := 512
	if len(data) < checkLen {
		checkLen = len(data)
	}
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}
