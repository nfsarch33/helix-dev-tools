package housekeeping

import "strings"

// Finding represents a single housekeeping item that needs attention.
type Finding struct {
	Repo   string `json:"repo"`
	Type   string `json:"type"`
	Branch string `json:"branch"`
}

// Summary aggregates finding counts by type.
type Summary struct {
	MergedRemote int `json:"merged_remote"`
	GoneLocal    int `json:"gone_local"`
	RepoCount    int `json:"repo_count"`
}

// ParseMergedBranches extracts branch names from `git branch -a --merged` output.
// It strips the `remotes/origin/` prefix and skips main, master, and HEAD.
func ParseMergedBranches(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "remotes/origin/") {
			continue
		}
		name := strings.TrimPrefix(trimmed, "remotes/origin/")
		if name == "main" || name == "master" || strings.HasPrefix(name, "HEAD") {
			continue
		}
		branches = append(branches, name)
	}
	return branches
}

// ParseGoneBranches extracts branch names from `git branch -vv` output where
// the tracking ref is marked as gone.
func ParseGoneBranches(output string) []string {
	var branches []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "*") {
			continue
		}
		if !strings.Contains(trimmed, ": gone]") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 1 {
			continue
		}
		branches = append(branches, fields[0])
	}
	return branches
}

// ClassifyFindings counts findings by type and distinct repos.
func ClassifyFindings(findings []Finding) Summary {
	var s Summary
	repos := make(map[string]bool)
	for _, f := range findings {
		repos[f.Repo] = true
		switch f.Type {
		case "merged_remote":
			s.MergedRemote++
		case "gone_local":
			s.GoneLocal++
		}
	}
	s.RepoCount = len(repos)
	return s
}
