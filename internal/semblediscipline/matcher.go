package semblediscipline

import (
	"strings"
)

// Verdict classifies a shell command for Semble-first discipline.
type Verdict string

const (
	VerdictNotApplicable Verdict = "not_applicable"
	VerdictLiteralOK     Verdict = "literal_ok"
	VerdictExploratory   Verdict = "exploratory"
)

// Result is the outcome of classifying one shell command string.
type Result struct {
	Verdict Verdict
	Tool    string // rg, grep, find, ack, or empty
	Reason  string
}

// Classify inspects a shell command for exploratory code search patterns.
// Literal fixed-string searches (-F) and single-file targets are allowed.
func Classify(command string) Result {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return Result{Verdict: VerdictNotApplicable}
	}

	tool := detectSearchTool(cmd)
	if tool == "" {
		return Result{Verdict: VerdictNotApplicable}
	}

	if hasLiteralFlag(cmd) {
		return Result{Verdict: VerdictLiteralOK, Tool: tool, Reason: "literal flag (-F/--fixed-strings)"}
	}

	if hasExactFileTarget(cmd, tool) {
		return Result{Verdict: VerdictLiteralOK, Tool: tool, Reason: "single explicit file target"}
	}

	return Result{Verdict: VerdictExploratory, Tool: tool, Reason: "semantic/exploratory search; prefer semble search"}
}

func detectSearchTool(cmd string) string {
	switch {
	case toolAtStartOrCompound(cmd, "find"):
		return "find"
	case toolAtStartOrCompound(cmd, "rg"):
		return "rg"
	case toolAtStartOrCompound(cmd, "ack"):
		return "ack"
	case toolAtStartOrCompound(cmd, "grep"):
		return "grep"
	default:
		return ""
	}
}

func toolAtStartOrCompound(cmd, tool string) bool {
	if strings.HasPrefix(cmd, tool+" ") || strings.HasPrefix(cmd, tool+"\t") {
		return true
	}
	if strings.Contains(cmd, " "+tool+" ") {
		return true
	}
	for _, sep := range []string{";", "&&", "||", "|"} {
		if strings.Contains(cmd, sep+tool+" ") {
			return true
		}
	}
	return false
}

func hasLiteralFlag(cmd string) bool {
	tokens := strings.Fields(cmd)
	for _, tok := range tokens {
		switch tok {
		case "-F", "--fixed-strings", "-x", "--line-regexp":
			return true
		}
	}
	return false
}

func hasExactFileTarget(cmd string, tool string) bool {
	if tool == "find" {
		return false
	}
	if hasRecursiveFlag(cmd) {
		return false
	}
	last := lastShellToken(cmd)
	if last == "" {
		return false
	}
	if strings.ContainsAny(last, "*?[") {
		return false
	}
	return looksLikeFilePath(last)
}

func hasRecursiveFlag(cmd string) bool {
	tokens := strings.Fields(cmd)
	for _, tok := range tokens {
		switch tok {
		case "-r", "-R", "--recursive":
			return true
		}
	}
	return false
}

func lastShellToken(cmd string) string {
	// Strip common redirects and background operators from the tail.
	trimmed := cmd
	for _, suffix := range []string{" 2>&1", " 2>/dev/null", " >/dev/null", " &"} {
		if i := strings.Index(trimmed, suffix); i >= 0 {
			trimmed = trimmed[:i]
		}
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func looksLikeFilePath(token string) bool {
	if token == "." || token == ".." {
		return false
	}
	// Directory or repo-root paths are exploratory targets, not single-file literals.
	if hasSourceFileExtension(token) {
		return true
	}
	// Bare filename without path (e.g. root.go) is an explicit file target.
	if !strings.Contains(token, "/") && strings.Contains(token, ".") {
		return true
	}
	return false
}

func hasSourceFileExtension(token string) bool {
	exts := []string{".go", ".md", ".ts", ".tsx", ".js", ".py", ".rs", ".yaml", ".yml", ".json", ".toml", ".mod", ".sum"}
	for _, ext := range exts {
		if strings.HasSuffix(token, ext) {
			return true
		}
	}
	return false
}
