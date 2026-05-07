package worktreemetrics

import "strings"

// AdoptionResult reports how many parallel-agent sessions used runx-managed
// worktrees during a rolling window.
type AdoptionResult struct {
	ParallelSessions int
	WorktreeSessions int
	Rate             float64
	BelowThreshold   bool
}

// Session is the minimal evidence record used for worktree adoption audits.
type Session struct {
	ID       string
	Parallel bool
	Worktree bool
}

func Adoption(sessions []Session, threshold float64) AdoptionResult {
	result := AdoptionResult{}
	for _, session := range sessions {
		if !session.Parallel {
			continue
		}
		result.ParallelSessions++
		if session.Worktree {
			result.WorktreeSessions++
		}
	}
	if result.ParallelSessions > 0 {
		result.Rate = float64(result.WorktreeSessions) / float64(result.ParallelSessions)
	}
	result.BelowThreshold = result.Rate < threshold
	return result
}

// ParseRunxWorktreeList recognizes the text emitted by `runx worktree list`.
func ParseRunxWorktreeList(raw string) []Session {
	var sessions []Session
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "no runx worktrees") {
			continue
		}
		if strings.Contains(line, "branch=") {
			sessions = append(sessions, Session{ID: line, Parallel: true, Worktree: true})
		}
	}
	return sessions
}
