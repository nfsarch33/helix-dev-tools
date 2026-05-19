package devex

import (
	"strings"
	"testing"
)

func TestParseConventionalCommit_WithScope(t *testing.T) {
	entry, ok := ParseConventionalCommit("abc1234 feat(platform): add registry package")
	if !ok {
		t.Fatal("expected successful parse")
	}
	if entry.Type != "feat" {
		t.Errorf("expected feat, got %s", entry.Type)
	}
	if entry.Scope != "platform" {
		t.Errorf("expected platform, got %s", entry.Scope)
	}
	if entry.Subject != "add registry package" {
		t.Errorf("unexpected subject: %s", entry.Subject)
	}
}

func TestParseConventionalCommit_NoScope(t *testing.T) {
	entry, ok := ParseConventionalCommit("def5678 fix: resolve timeout issue")
	if !ok {
		t.Fatal("expected parse success")
	}
	if entry.Type != "fix" || entry.Scope != "" {
		t.Errorf("unexpected: type=%s scope=%s", entry.Type, entry.Scope)
	}
}

func TestParseConventionalCommit_Invalid(t *testing.T) {
	_, ok := ParseConventionalCommit("abc1234 not a conventional commit")
	if ok {
		t.Error("expected parse failure for non-conventional commit")
	}
}

func TestGenerateChangelog(t *testing.T) {
	commits := []CommitEntry{
		{Hash: "abc1234567", Type: "feat", Scope: "api", Subject: "add health endpoint"},
		{Hash: "def5678901", Type: "fix", Subject: "resolve null pointer"},
		{Hash: "ghi9012345", Type: "feat", Scope: "db", Subject: "add connection pooling"},
	}

	log := GenerateChangelog(commits)
	if !strings.Contains(log, "### Features") {
		t.Error("missing Features section")
	}
	if !strings.Contains(log, "health endpoint") {
		t.Error("missing feat entry")
	}
	if !strings.Contains(log, "### Bug Fixes") {
		t.Error("missing Bug Fixes section")
	}
}

func TestGenerateChangelog_Empty(t *testing.T) {
	log := GenerateChangelog(nil)
	if log != "" {
		t.Errorf("expected empty changelog, got: %s", log)
	}
}
