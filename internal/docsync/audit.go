package docsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Finding struct {
	Code    string
	Path    string
	Message string
}

type Report struct {
	Root     string
	Findings []Finding
	Fixed    []string
}

type Options struct {
	RequirePublicFiles bool
}

func (r Report) OK() bool {
	return len(r.Findings) == 0
}

func AuditRepo(root string) (Report, error) {
	return AuditRepoWithOptions(root, Options{})
}

func AuditRepoWithOptions(root string, opts Options) (Report, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	report := Report{Root: root}
	version := repoVersion(root)

	report.requireFile(root, "README.md", "REQUIRED_FILE", "README.md is required")
	if opts.RequirePublicFiles && isCodeRepo(root) {
		report.requireFile(root, "LICENSE", "REQUIRED_FILE", "LICENSE is required")
		report.requireFile(root, "CONTRIBUTING.md", "REQUIRED_FILE", "CONTRIBUTING.md is required")
	}

	if version != "" {
		report.fileContains(root, "README.md", versionTokens(version), "README_VERSION", "README.md does not mention the current version")
		report.fileContains(root, "CHANGELOG.md", versionTokens(version), "CHANGELOG_VERSION", "CHANGELOG.md does not mention the current version")
		report.fileContains(root, filepath.Join("api", "openapi.yaml"), []string{"version: " + version, "version: \"" + version + "\""}, "OPENAPI_VERSION", "api/openapi.yaml version does not match VERSION")
	}

	if pkgVersion, ok := packageVersion(root); ok && version != "" && pkgVersion != version {
		report.Findings = append(report.Findings, Finding{
			Code:    "PACKAGE_VERSION",
			Path:    "package.json",
			Message: fmt.Sprintf("package.json version %q does not match VERSION %q", pkgVersion, version),
		})
	}

	report.auditADRIndex(root)
	return report, nil
}

func FixRepo(root string) (Report, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	report := Report{Root: root}
	version := repoVersion(root)
	if version != "" {
		fixed, err := updateREADMEVersion(root, version)
		if err != nil {
			return Report{}, err
		}
		if fixed {
			report.Fixed = append(report.Fixed, "README.md")
		}
		fixed, err = updateOpenAPIVersion(root, version)
		if err != nil {
			return Report{}, err
		}
		if fixed {
			report.Fixed = append(report.Fixed, filepath.Join("api", "openapi.yaml"))
		}
	}
	fixed, err := writeADRIndex(root)
	if err != nil {
		return Report{}, err
	}
	if fixed {
		report.Fixed = append(report.Fixed, filepath.Join("docs", "adr", "README.md"))
	}
	audit, err := AuditRepo(root)
	if err != nil {
		return Report{}, err
	}
	report.Findings = audit.Findings
	return report, nil
}

func (r *Report) requireFile(root, rel, code, message string) {
	if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
		r.Findings = append(r.Findings, Finding{Code: code, Path: rel, Message: message})
	}
}

func (r *Report) fileContains(root, rel string, tokens []string, code, message string) {
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return
	}
	text := string(raw)
	for _, token := range tokens {
		if strings.Contains(text, token) {
			return
		}
	}
	r.Findings = append(r.Findings, Finding{Code: code, Path: rel, Message: message})
}

func (r *Report) auditADRIndex(root string) {
	adrs, err := adrFiles(root)
	if err != nil || len(adrs) == 0 {
		return
	}
	indexPath := filepath.Join("docs", "adr", "README.md")
	raw, err := os.ReadFile(filepath.Join(root, indexPath))
	if err != nil {
		r.Findings = append(r.Findings, Finding{Code: "ADR_INDEX", Path: indexPath, Message: "ADR index is missing"})
		return
	}
	index := string(raw)
	for _, adr := range adrs {
		if !strings.Contains(index, adr) {
			r.Findings = append(r.Findings, Finding{Code: "ADR_INDEX", Path: indexPath, Message: "ADR index does not reference " + adr})
			return
		}
	}
}

func repoVersion(root string) string {
	if raw, err := os.ReadFile(filepath.Join(root, "VERSION")); err == nil {
		if v := strings.TrimSpace(string(raw)); v != "" {
			return strings.TrimPrefix(v, "v")
		}
	}
	if v, ok := packageVersion(root); ok {
		return strings.TrimPrefix(v, "v")
	}
	return ""
}

func packageVersion(root string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return "", false
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil || strings.TrimSpace(pkg.Version) == "" {
		return "", false
	}
	return strings.TrimPrefix(strings.TrimSpace(pkg.Version), "v"), true
}

func isCodeRepo(root string) bool {
	for _, rel := range []string{"VERSION", "package.json", "go.mod"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			return true
		}
	}
	return false
}

func versionTokens(version string) []string {
	return []string{version, "v" + strings.TrimPrefix(version, "v")}
}

func writeADRIndex(root string) (bool, error) {
	adrs, err := adrFiles(root)
	if err != nil || len(adrs) == 0 {
		return false, err
	}
	indexPath := filepath.Join(root, "docs", "adr", "README.md")
	var b strings.Builder
	b.WriteString("# ADR Index\n\n")
	for _, adr := range adrs {
		title := strings.TrimSuffix(adr, filepath.Ext(adr))
		b.WriteString("- [")
		b.WriteString(title)
		b.WriteString("](")
		b.WriteString(adr)
		b.WriteString(")\n")
	}
	return true, os.WriteFile(indexPath, []byte(b.String()), 0o644)
}

func updateREADMEVersion(root, version string) (bool, error) {
	path := filepath.Join(root, "README.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	lines := strings.Split(string(raw), "\n")
	changed := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Current release:") {
			suffix := "."
			if idx := strings.Index(line, ". See "); idx >= 0 {
				suffix = line[idx:]
			}
			lines[i] = "Current release: **v" + strings.TrimPrefix(version, "v") + "**" + suffix
			changed = lines[i] != line
			break
		}
	}
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func updateOpenAPIVersion(root, version string) (bool, error) {
	path := filepath.Join(root, "api", "openapi.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	lines := strings.Split(string(raw), "\n")
	inInfo := false
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if line == "info:" {
			inInfo = true
			continue
		}
		if inInfo && len(line) > 0 && line[0] != ' ' {
			break
		}
		if inInfo && strings.HasPrefix(trimmed, "version:") {
			next := leadingSpace(line) + "version: " + strings.TrimPrefix(version, "v")
			changed = next != line
			lines[i] = next
			break
		}
	}
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func leadingSpace(line string) string {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return line[:i]
		}
	}
	return line
}

func adrFiles(root string) ([]string, error) {
	dir := filepath.Join(root, "docs", "adr")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "README.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
