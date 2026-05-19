package racedetect

import (
	"fmt"
	"sync"
	"time"
)

type ClaimAttempt struct {
	TicketID  string
	AgentID   string
	Timestamp time.Time
	Success   bool
	Error     string
}

type RaceReport struct {
	TotalAttempts    int
	SuccessfulClaims int
	RejectedClaims   int
	DoubleClaims     int
	Attempts         []ClaimAttempt
}

type ClaimFunc func(ticketID, agentID string) error

func SimulateConcurrentClaims(ticketID string, agents []string, claimFn ClaimFunc) RaceReport {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var attempts []ClaimAttempt

	for _, agent := range agents {
		wg.Add(1)
		go func(a string) {
			defer wg.Done()
			err := claimFn(ticketID, a)
			mu.Lock()
			attempts = append(attempts, ClaimAttempt{
				TicketID:  ticketID,
				AgentID:   a,
				Timestamp: time.Now(),
				Success:   err == nil,
				Error:     errStr(err),
			})
			mu.Unlock()
		}(agent)
	}
	wg.Wait()

	report := RaceReport{TotalAttempts: len(attempts), Attempts: attempts}
	for _, a := range attempts {
		if a.Success {
			report.SuccessfulClaims++
		} else {
			report.RejectedClaims++
		}
	}
	if report.SuccessfulClaims > 1 {
		report.DoubleClaims = report.SuccessfulClaims - 1
	}
	return report
}

func ValidateAtomicity(report RaceReport) error {
	if report.DoubleClaims > 0 {
		return fmt.Errorf("RACE CONDITION: %d double claims detected on same ticket", report.DoubleClaims)
	}
	if report.SuccessfulClaims != 1 {
		return fmt.Errorf("expected exactly 1 successful claim, got %d", report.SuccessfulClaims)
	}
	return nil
}

func DetectFileConflicts(paths []string, agentEdits map[string][]string) []string {
	fileOwners := make(map[string][]string)
	for agent, files := range agentEdits {
		for _, f := range files {
			fileOwners[f] = append(fileOwners[f], agent)
		}
	}

	var conflicts []string
	for file, owners := range fileOwners {
		if len(owners) > 1 {
			conflicts = append(conflicts, fmt.Sprintf("%s: edited by %v", file, owners))
		}
	}
	return conflicts
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
