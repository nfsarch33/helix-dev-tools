// Package cli — git pre-commit fleet-scan handler.
//
// This hook scans staged content for forbidden Zendesk-AI gateway URLs
// in fleet code paths. The gateway boundary is "MacBook only"; fleet
// code (cylrl orchestrator, MC delegator, router, ironclaw engine,
// ironclaw-ops overlays) must never embed the URL or its subdomains.
// Documentation, research notes, and changelogs are not scanned —
// references in prose are legitimate and the human review there is
// sufficient.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// stagedFileScan is the unit of work that the runner consumes. The
// staged content is read into memory once per file (small files only;
// the deny-list pattern is short).
type stagedFileScan struct {
	Path    string
	Content []byte
}

// fleetCodePathPrefixes returns the canonical list of fleet root path
// prefixes. The order is stable so the test can pin it.
func fleetCodePathPrefixes() []string {
	return []string{
		"go/internal/cylrl/",
		"go/internal/mc/",
		"go/internal/router/",
		"ironclaw-ops/",
		"ironclaw/",
	}
}

// fleetForbiddenURLPatterns lists the literal substrings that must not
// appear in any fleet code path. Subdomains of cursor-ai.zendesk.com
// are all banned; the bare host string is enough since exact match is
// required.
var fleetForbiddenURLPatterns = []string{
	"cursor-ai.zendesk.com",
	"ai.zendesk.com/v1/cursor", // future-proof for a forked URL shape
}

// scanFleetForbiddenURLs returns a non-empty finding string when path
// is fleet-controlled AND content embeds a forbidden URL pattern.
// Empty return means clean.
func scanFleetForbiddenURLs(path string, content []byte) string {
	if !isFleetPath(path) {
		return ""
	}
	body := string(content)
	for _, pat := range fleetForbiddenURLPatterns {
		if strings.Contains(body, pat) {
			return fmt.Sprintf("%s: forbidden URL substring %q (fleet code MUST NOT embed Zendesk AI gateway URLs; gateway boundary is MacBook only)", path, pat)
		}
	}
	return ""
}

func isFleetPath(path string) bool {
	for _, prefix := range fleetCodePathPrefixes() {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// runFleetScan runs the deny check across a slice of staged files and
// returns the list of human-readable findings (empty when clean).
func runFleetScan(files []stagedFileScan) []string {
	findings := []string{}
	for _, f := range files {
		if msg := scanFleetForbiddenURLs(f.Path, f.Content); msg != "" {
			findings = append(findings, msg)
		}
	}
	return findings
}

var preCommitFleetScanExit = os.Exit
var preCommitFleetScanStderr io.Writer = os.Stderr
var preCommitFleetScanCwd = os.Getwd

var preCommitFleetScanCmd = &cobra.Command{
	Use:   "pre-commit-fleet-scan",
	Short: "Scan staged fleet code for forbidden Zendesk AI gateway URLs",
	Long: "Reads the list of staged files via `git diff --name-only --cached` and refuses\n" +
		"any commit that adds a forbidden Zendesk AI gateway URL to fleet code paths.\n" +
		"The gateway boundary is MacBook only; fleet code must reach the gateway through\n" +
		"the loopback proxy (127.0.0.1:9787) or the router seam, never the public URL.",
	RunE: runPreCommitFleetScan,
}

func init() {
	githookCmd.AddCommand(preCommitFleetScanCmd)
}

func runPreCommitFleetScan(_ *cobra.Command, _ []string) error {
	cwd, err := preCommitFleetScanCwd()
	if err != nil {
		fmt.Fprintf(preCommitFleetScanStderr, "ERROR: cannot resolve working directory: %v\n", err)
		preCommitFleetScanExit(1)
		return nil
	}
	files, err := loadStagedFleetFiles(cwd)
	if err != nil {
		fmt.Fprintf(preCommitFleetScanStderr, "ERROR: cannot load staged files: %v\n", err)
		preCommitFleetScanExit(1)
		return nil
	}
	findings := runFleetScan(files)
	if len(findings) == 0 {
		return nil
	}
	fmt.Fprintln(preCommitFleetScanStderr, "ERROR: cursor-tools fleet-scan found forbidden ZD gateway URLs in staged fleet code:")
	for _, f := range findings {
		fmt.Fprintln(preCommitFleetScanStderr, "  - "+f)
	}
	fmt.Fprintln(preCommitFleetScanStderr,
		"\nRemediation:\n"+
			"  - Replace the URL with the loopback proxy: 127.0.0.1:9787\n"+
			"  - Or route through the cluster router seam (no host strings in fleet code)\n"+
			"  - Tier-A subagent calls go through `cursor-tools tier-a metric record`")
	preCommitFleetScanExit(1)
	return nil
}

// loadStagedFleetFiles asks git for the list of staged files and reads
// the content of each one that lives under a fleet path. Untouched
// directories are silently ignored. Symlinks and binary blobs are
// passed through unchanged; the deny-list scan is purely substring
// based.
func loadStagedFleetFiles(cwd string) ([]stagedFileScan, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--cached")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	var files []stagedFileScan
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path == "" || !isFleetPath(path) {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		files = append(files, stagedFileScan{Path: path, Content: content})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return files, nil
}
