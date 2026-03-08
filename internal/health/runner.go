package health

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
)

// Result tracks a single assertion.
type Result struct {
	Name   string
	Passed bool
	Detail string
}

// Suite groups assertions under a name.
type Suite struct {
	Name    string
	Results []Result
}

// Pass adds a passing assertion.
func (s *Suite) Pass(name string) {
	s.Results = append(s.Results, Result{Name: name, Passed: true})
}

// Fail adds a failing assertion with detail.
func (s *Suite) Fail(name, detail string) {
	s.Results = append(s.Results, Result{Name: name, Passed: false, Detail: detail})
}

// Assert adds a pass or fail based on condition.
func (s *Suite) Assert(name string, condition bool, detail string) {
	if condition {
		s.Pass(name)
	} else {
		s.Fail(name, detail)
	}
}

// AssertFileExists checks that a file or directory exists.
func (s *Suite) AssertFileExists(name, path string) {
	_, err := os.Stat(path)
	s.Assert(name, err == nil, fmt.Sprintf("not found: %s", path))
}

// AssertSymlink checks that path is a symbolic link.
func (s *Suite) AssertSymlink(name, path string) {
	info, err := os.Lstat(path)
	s.Assert(name, err == nil && info.Mode()&os.ModeSymlink != 0, fmt.Sprintf("not a symlink: %s", path))
}

// AssertFileContains checks that a file contains a substring.
func (s *Suite) AssertFileContains(name, path, substr string) {
	data, err := os.ReadFile(path)
	if err != nil {
		s.Fail(name, fmt.Sprintf("cannot read: %s", path))
		return
	}
	s.Assert(name, strings.Contains(string(data), substr), fmt.Sprintf("'%s' not found in %s", substr, filepath.Base(path)))
}

// AssertFileNotContains checks that a file does NOT contain a substring.
func (s *Suite) AssertFileNotContains(name, path, substr string) {
	data, err := os.ReadFile(path)
	if err != nil {
		s.Pass(name) // file not found = not containing
		return
	}
	s.Assert(name, !strings.Contains(string(data), substr), fmt.Sprintf("'%s' found in %s", substr, filepath.Base(path)))
}

// AssertFileMatches checks that file content matches a regex.
func (s *Suite) AssertFileMatches(name, path, pattern string) {
	data, err := os.ReadFile(path)
	if err != nil {
		s.Fail(name, fmt.Sprintf("cannot read: %s", path))
		return
	}
	re := regexp.MustCompile(pattern)
	s.Assert(name, re.Match(data), fmt.Sprintf("pattern '%s' not matched in %s", pattern, filepath.Base(path)))
}

// AssertDirMinCount checks a directory has at least n entries matching an optional suffix.
func (s *Suite) AssertDirMinCount(name, dir string, minCount int, suffix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		s.Fail(name, fmt.Sprintf("cannot read dir: %s", dir))
		return
	}
	count := 0
	for _, e := range entries {
		if suffix == "" || strings.HasSuffix(e.Name(), suffix) {
			count++
		}
	}
	s.Assert(name, count >= minCount, fmt.Sprintf("got %d, want >= %d in %s", count, minCount, dir))
}

// PassCount returns the number of passing assertions.
func (s *Suite) PassCount() int {
	c := 0
	for _, r := range s.Results {
		if r.Passed {
			c++
		}
	}
	return c
}

// Total returns the total number of assertions.
func (s *Suite) Total() int {
	return len(s.Results)
}

// Runner executes all suites and prints results.
type Runner struct {
	Paths  config.Paths
	suites []*Suite
}

// NewRunner creates a health check runner.
func NewRunner() *Runner {
	return &Runner{Paths: config.DefaultPaths()}
}

// Add registers a suite.
func (r *Runner) Add(s *Suite) {
	r.suites = append(r.suites, s)
}

// Run executes all suites and prints formatted results.
// Returns total pass and total count.
func (r *Runner) Run() (int, int) {
	fmt.Println()
	for i, s := range r.suites {
		fmt.Printf("  Suite %d: %s\n", i+1, s.Name)
		for _, res := range s.Results {
			if res.Passed {
				clilog.Pass("%s", res.Name)
			} else {
				clilog.Fail("%s -- %s", res.Name, res.Detail)
			}
		}
		fmt.Println()
	}

	clilog.Header("RESULTS")
	fmt.Println()
	fmt.Printf("%-52s %s\n", "Suite", "Pass  Total")
	clilog.Divider()

	totalPass := 0
	totalCount := 0
	for i, s := range r.suites {
		pass := s.PassCount()
		total := s.Total()
		totalPass += pass
		totalCount += total
		status := "PASS"
		if pass < total {
			status = "FAIL"
		}
		fmt.Printf("  Suite %d: %-42s %d/%d   %s\n", i+1, s.Name, pass, total, status)
	}

	clilog.Summary(totalPass, totalCount)

	return totalPass, totalCount
}
