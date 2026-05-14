package domain

import "errors"

var (
	ErrRepoExcluded   = errors.New("reviewd: repo excluded from review")
	ErrNoNewCommits   = errors.New("reviewd: no new commits since last cycle")
	ErrCyclePending   = errors.New("reviewd: cycle already pending")
	ErrCycleEscalated = errors.New("reviewd: cycle escalated to operator")
	ErrIssueWriteDeny = errors.New("reviewd: tier does not allow issue writes")
)
