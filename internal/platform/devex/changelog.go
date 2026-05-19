package devex

import (
	"fmt"
	"strings"
	"time"
)

type CommitEntry struct {
	Hash    string
	Type    string
	Scope   string
	Subject string
	Date    time.Time
}

type ChangelogSection struct {
	Title   string
	Entries []CommitEntry
}

func ParseConventionalCommit(line string) (CommitEntry, bool) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		return CommitEntry{}, false
	}
	hash := parts[0]
	msg := parts[1]

	typeSep := strings.Index(msg, ":")
	if typeSep < 0 {
		return CommitEntry{}, false
	}

	typeScope := msg[:typeSep]
	subject := strings.TrimSpace(msg[typeSep+1:])

	var commitType, scope string
	if scopeStart := strings.Index(typeScope, "("); scopeStart >= 0 {
		commitType = typeScope[:scopeStart]
		scope = strings.TrimSuffix(typeScope[scopeStart+1:], ")")
	} else {
		commitType = typeScope
	}

	return CommitEntry{Hash: hash, Type: commitType, Scope: scope, Subject: subject}, true
}

func GenerateChangelog(commits []CommitEntry) string {
	sections := map[string]*ChangelogSection{
		"feat":     {Title: "Features"},
		"fix":      {Title: "Bug Fixes"},
		"refactor": {Title: "Refactoring"},
		"docs":     {Title: "Documentation"},
		"test":     {Title: "Tests"},
	}

	for _, c := range commits {
		sec, ok := sections[c.Type]
		if !ok {
			continue
		}
		sec.Entries = append(sec.Entries, c)
	}

	var sb strings.Builder
	for _, typ := range []string{"feat", "fix", "refactor", "docs", "test"} {
		sec := sections[typ]
		if len(sec.Entries) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", sec.Title))
		for _, e := range sec.Entries {
			if e.Scope != "" {
				sb.WriteString(fmt.Sprintf("- **%s**: %s (%s)\n", e.Scope, e.Subject, e.Hash[:7]))
			} else {
				sb.WriteString(fmt.Sprintf("- %s (%s)\n", e.Subject, e.Hash[:7]))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
