package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/nfsarch33/helix-dev-tools/internal/hookinstaller"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type rebrandCategory string

const (
	catBrandName   rebrandCategory = "brand-name"
	catToolName    rebrandCategory = "tool-name"
	catDeprecated  rebrandCategory = "deprecated-name"
	catEnvVar      rebrandCategory = "env-var-prefix"
	catK8sLabel    rebrandCategory = "k8s-label"
	catDockerImage rebrandCategory = "docker-image"
	catGoModule    rebrandCategory = "go-module-path"
)

type rebrandRule struct {
	Pattern     string
	Category    rebrandCategory
	Replacement string
}

var rebrandRules = []rebrandRule{
	// Go module paths (checked first — longest match wins)
	{"github.com/nfsarch33/helixon-ops", catGoModule, "github.com/nfsarch33/helixon-ops"},
	{"github.com/nfsarch33/helixon-mcp", catGoModule, "github.com/nfsarch33/helixon-mcp"},
	{"github.com/nfsarch33/helixon-", catGoModule, "github.com/nfsarch33/helixon-"},
	{"github.com/nfsarch33/cylrl-", catGoModule, "github.com/nfsarch33/helixon-"},
	{"github.com/nfsarch33/cursor-global-kb", catGoModule, "github.com/nfsarch33/helixon-kb"},

	// Docker image names
	{"helixon/", catDockerImage, "helixon/"},
	{"cylrl/", catDockerImage, "helixon/"},

	// K8s labels
	{"helixon-system", catK8sLabel, "helixon-system"},
	{"cylrl-system", catK8sLabel, "helixon-system"},

	// Env var prefixes
	{"HELIXON_", catEnvVar, "HELIXON_"},
	{"CYLRL_", catEnvVar, "HELIXON_"},

	// Deprecated names
	{"cylrl", catDeprecated, "helixon"},
	{"CYLRL", catDeprecated, "HELIXON"},
	{"evomap", catDeprecated, "evospine"},
	{"EvoMap", catDeprecated, "EvoSpine"},

	// Tool names
	{"cursor-global-kb", catToolName, "helixon-kb"},

	// Brand names — case variations
	{"HELIXON", catBrandName, "HELIXON"},
	{"Helixon", catBrandName, "Helixon"},
	{"helixon", catBrandName, "helixon"},
}

type rebrandFinding struct {
	File        string          `json:"file"`
	Line        int             `json:"line"`
	Category    rebrandCategory `json:"category"`
	Match       string          `json:"match"`
	Replacement string          `json:"replacement"`
	Context     string          `json:"context,omitempty"`
}

// cliAllowlistEntry mirrors rebrandvalidator.AllowlistEntry but lives in the
// CLI package to avoid a cross-package import cycle.
type cliAllowlistEntry struct {
	File string `yaml:"file"`
	Term string `yaml:"term"`
}

type cliAllowlistFile struct {
	Entries []cliAllowlistEntry `yaml:"entries"`
}

// loadAllowlistFromDir reads .rebrand-allowlist.yaml from root if present.
// A missing file is not an error.
func loadAllowlistFromDir(root string) ([]cliAllowlistEntry, error) {
	path := filepath.Join(root, ".rebrand-allowlist.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var af cliAllowlistFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, err
	}
	return af.Entries, nil
}

// isSuppressedByCLIAllowlist reports whether a legacy-term finding in a file
// should be suppressed by an allowlist entry.
func isSuppressedByCLIAllowlist(relPath, term string, allowlist []cliAllowlistEntry) bool {
	base := filepath.Base(relPath)
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

// docRepoSkipCategories lists category types that doc-only repos legitimately
// contain: historical SOPs and handoffs reference old brand/tool names by
// design, so these categories are suppressed when --doc-repo is set.
var docRepoSkipCategories = map[rebrandCategory]bool{
	catBrandName:  true,
	catToolName:   true,
	catDeprecated: true,
}

var (
	rebrandRepo     string
	rebrandAllOwned bool
	rebrandFormat   string
	rebrandDir      string
	rebrandDocRepo  bool
)

var rebrandCmd = &cobra.Command{
	Use:   "rebrand",
	Short: "Helixon rebrand tooling",
}

var rebrandScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan repository for legacy brand terms that need rebranding",
	Long: "Scans tracked files for legacy terms (helixon, cursor-tools, cylrl, evomap)\n" +
		"and reports findings with suggested Helixon replacements.",
	SilenceUsage: true,
	RunE:         runRebrandScan,
}

var rebrandInstallHooksCmd = &cobra.Command{
	Use:          "install-hooks [repo-dir]",
	Short:        "Install rebrand pre-push git hook into a repository",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runRebrandInstallHooks,
}

var rebrandHookBinary string

func init() {
	rebrandScanCmd.Flags().StringVar(&rebrandRepo, "repo", "", "Repository path or runx alias")
	rebrandScanCmd.Flags().BoolVar(&rebrandAllOwned, "all-owned", false, "Scan all owned repositories")
	rebrandScanCmd.Flags().StringVar(&rebrandFormat, "format", "human", "Output format: human or json")
	rebrandScanCmd.Flags().StringVar(&rebrandDir, "dir", ".", "Directory to scan (defaults to cwd)")
	rebrandScanCmd.Flags().BoolVar(&rebrandDocRepo, "doc-repo", false, "Doc-only repo mode: skip brand-name, tool-name, and deprecated-name categories")
	rebrandInstallHooksCmd.Flags().StringVar(&rebrandHookBinary, "binary", "cursor-tools", "Binary name to embed in the hook script")
	rebrandCmd.AddCommand(rebrandScanCmd)
	rebrandCmd.AddCommand(rebrandInstallHooksCmd)
}

func runRebrandInstallHooks(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	if err := hookinstaller.InstallPrePushHook(dir, rebrandHookBinary); err != nil {
		return fmt.Errorf("install pre-push hook: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Installed pre-push hook in %s/.git/hooks/pre-push (binary: %s)\n", dir, rebrandHookBinary)
	return nil
}

// rebrandExitFunc is overridable in tests.
var rebrandExitFunc = os.Exit

func runRebrandScan(cmd *cobra.Command, _ []string) error {
	dir := rebrandDir
	if rebrandRepo != "" {
		dir = rebrandRepo
	}

	var (
		findings []rebrandFinding
		err      error
	)
	if rebrandDocRepo {
		findings, err = scanDirectoryDocRepo(dir)
	} else {
		findings, err = scanDirectory(dir)
	}
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if rebrandFormat == "json" {
		return writeRebrandJSON(w, findings)
	}
	writeRebrandHuman(w, findings)

	if len(findings) > 0 {
		rebrandExitFunc(2)
	}
	return nil
}

func scanDirectory(root string) ([]rebrandFinding, error) {
	allowlist, _ := loadAllowlistFromDir(root)

	var findings []rebrandFinding

	trackedFiles, err := getTrackedFiles(root)
	if err != nil {
		return scanDirectoryWalkWithAllowlist(root, allowlist)
	}
	for _, relPath := range trackedFiles {
		if relPath == ".rebrand-allowlist.yaml" {
			continue
		}
		fullPath := filepath.Join(root, relPath)
		fileFindings, err := scanFile(fullPath, relPath)
		if err != nil {
			continue
		}
		for _, f := range fileFindings {
			if !isSuppressedByCLIAllowlist(f.File, f.Match, allowlist) {
				findings = append(findings, f)
			}
		}
	}
	return findings, nil
}

// scanDirectoryDocRepo is like scanDirectory but uses scanFileDocRepo to skip
// brand-name, tool-name, and deprecated-name categories. Used when --doc-repo.
func scanDirectoryDocRepo(root string) ([]rebrandFinding, error) {
	var findings []rebrandFinding

	trackedFiles, err := getTrackedFiles(root)
	if err != nil {
		return scanDirectoryDocRepoWalk(root)
	}
	for _, relPath := range trackedFiles {
		fullPath := filepath.Join(root, relPath)
		fileFindings, err := scanFileDocRepo(fullPath, relPath)
		if err != nil {
			continue
		}
		findings = append(findings, fileFindings...)
	}
	return findings, nil
}

func scanDirectoryDocRepoWalk(root string) ([]rebrandFinding, error) {
	var findings []rebrandFinding
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == "vendor" || base == ".next" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return nil
		}
		fileFindings, err := scanFileDocRepo(path, rel)
		if err != nil {
			return nil
		}
		findings = append(findings, fileFindings...)
		return nil
	})
	return findings, err
}

// scanFileDocRepo scans a single file using only the categories that apply to
// doc-only repos (go-module-path, k8s-label, docker-image, env-var).
// Brand-name, tool-name, and deprecated-name are skipped.
func scanFileDocRepo(fullPath, relPath string) ([]rebrandFinding, error) {
	if isBinaryPath(relPath) {
		return nil, nil
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil && n == 0 {
		return nil, nil
	}
	if !utf8.Valid(header[:n]) {
		return nil, nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	var findings []rebrandFinding
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, rule := range rebrandRules {
			if docRepoSkipCategories[rule.Category] {
				continue
			}
			if idx := strings.Index(line, rule.Pattern); idx >= 0 {
				findings = append(findings, rebrandFinding{
					File:        relPath,
					Line:        lineNum,
					Category:    rule.Category,
					Match:       rule.Pattern,
					Replacement: rule.Replacement,
					Context:     strings.TrimSpace(line),
				})
				break
			}
		}
	}
	return findings, scanner.Err()
}

func scanDirectoryWalk(root string) ([]rebrandFinding, error) {
	allowlist, _ := loadAllowlistFromDir(root)
	return scanDirectoryWalkWithAllowlist(root, allowlist)
}

func scanDirectoryWalkWithAllowlist(root string, allowlist []cliAllowlistEntry) ([]rebrandFinding, error) {
	var findings []rebrandFinding
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == "vendor" || base == ".next" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return nil
		}
		if filepath.Base(rel) == ".rebrand-allowlist.yaml" {
			return nil
		}
		fileFindings, err := scanFile(path, rel)
		if err != nil {
			return nil
		}
		for _, f := range fileFindings {
			if !isSuppressedByCLIAllowlist(f.File, f.Match, allowlist) {
				findings = append(findings, f)
			}
		}
		return nil
	})
	return findings, err
}

func getTrackedFiles(root string) ([]string, error) {
	out, err := runCommandOutput(10_000_000_000, "git", "-C", root, "ls-files")
	if err != nil {
		return nil, err
	}
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}
	return files, scanner.Err()
}

func scanFile(fullPath, relPath string) ([]rebrandFinding, error) {
	if isBinaryPath(relPath) {
		return nil, nil
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Skip binary content by checking first 512 bytes.
	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil && n == 0 {
		return nil, nil
	}
	if !utf8.Valid(header[:n]) {
		return nil, nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	var findings []rebrandFinding
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, rule := range rebrandRules {
			if idx := strings.Index(line, rule.Pattern); idx >= 0 {
				findings = append(findings, rebrandFinding{
					File:        relPath,
					Line:        lineNum,
					Category:    rule.Category,
					Match:       rule.Pattern,
					Replacement: rule.Replacement,
					Context:     strings.TrimSpace(line),
				})
				break
			}
		}
	}
	return findings, scanner.Err()
}

func isBinaryPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
		".webp": true, ".svg": true, ".woff": true, ".woff2": true, ".ttf": true,
		".eot": true, ".otf": true, ".zip": true, ".tar": true, ".gz": true,
		".bz2": true, ".xz": true, ".7z": true, ".rar": true, ".pdf": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
		".o": true, ".a": true, ".pyc": true, ".class": true, ".jar": true,
		".wasm": true, ".mp3": true, ".mp4": true, ".wav": true, ".avi": true,
		".mov": true, ".sqlite": true, ".db": true,
	}
	return binaryExts[ext]
}

func writeRebrandHuman(w io.Writer, findings []rebrandFinding) {
	if len(findings) == 0 {
		fmt.Fprintln(w, "No legacy terms found.")
		return
	}
	fmt.Fprintf(w, "Found %d legacy term(s):\n\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(w, "%s:%d: %s %q -> %s\n", f.File, f.Line, f.Category, f.Match, f.Replacement)
	}
}

func writeRebrandJSON(w io.Writer, findings []rebrandFinding) error {
	output := struct {
		Count    int              `json:"count"`
		Findings []rebrandFinding `json:"findings"`
	}{
		Count:    len(findings),
		Findings: findings,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
