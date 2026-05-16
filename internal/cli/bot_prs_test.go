package cli

import "testing"

func TestIsBotAuthor(t *testing.T) {
	tests := []struct {
		login string
		want  bool
	}{
		{"dependabot[bot]", true},
		{"github-actions[bot]", true},
		{"app/dependabot", true},
		{"nfsarch33", false},
		{"renovate[bot]", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isBotAuthor(tt.login); got != tt.want {
			t.Errorf("isBotAuthor(%q) = %v, want %v", tt.login, got, tt.want)
		}
	}
}

func TestClassifyPR(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"chore(deps): bump mistune from 3.1.4 to 3.2.1", "MERGE"},
		{"chore(deps): bump urllib3 from 2.3.0 to 2.7.0", "MERGE"},
		{"chore(deps): bump mako from 1.3.10 to 1.3.12", "MERGE"},
		{"chore(deps): bump authlib from 1.6.5 to 1.6.12", "MERGE"},
	}
	for _, tt := range tests {
		pr := ghPR{Title: tt.title, Author: "dependabot[bot]"}
		if got := classifyPR(pr); got != tt.want {
			t.Errorf("classifyPR(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestIsMajorVersionChange(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{"0.4.37", "0.8.0", false},
		{"3.1.4", "3.2.1", false},
		{"0.3.79", "1.3.3", true},
		{"2.3.0", "2.7.0", false},
		{"1.6.5", "1.6.12", false},
		{"3.1.45", "3.1.50", false},
	}
	for _, tt := range tests {
		if got := isMajorVersionChange(tt.from, tt.to); got != tt.want {
			t.Errorf("isMajorVersionChange(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestExtractMajor(t *testing.T) {
	tests := []struct {
		ver  string
		want string
	}{
		{"1.2.3", "1"},
		{"v2.3.0", "2"},
		{"0.4.37", "0"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := extractMajor(tt.ver); got != tt.want {
			t.Errorf("extractMajor(%q) = %q, want %q", tt.ver, got, tt.want)
		}
	}
}
