package conflictdetect

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type ConflictType string

const (
	ConflictFile   ConflictType = "file"
	ConflictBranch ConflictType = "branch"
)

type Conflict struct {
	Type       ConflictType `json:"type"`
	Resource   string       `json:"resource"`
	AgentA     string       `json:"agent_a"`
	AgentB     string       `json:"agent_b"`
	DetectedAt time.Time    `json:"detected_at"`
}

type FileLock struct {
	FilePath  string    `json:"file_path"`
	AgentID   string    `json:"agent_id"`
	ClaimedAt time.Time `json:"claimed_at"`
}

type Detector struct {
	locks     []FileLock
	conflicts []Conflict
	mu        sync.Mutex
}

func NewDetector() *Detector {
	return &Detector{
		locks:     []FileLock{},
		conflicts: []Conflict{},
	}
}

func (d *Detector) ClaimFile(agentID, filePath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, lock := range d.locks {
		if lock.FilePath == filePath && lock.AgentID != agentID {
			d.conflicts = append(d.conflicts, Conflict{
				Type:       ConflictFile,
				Resource:   filePath,
				AgentA:     lock.AgentID,
				AgentB:     agentID,
				DetectedAt: time.Now(),
			})
			return fmt.Errorf("file %s is already claimed by agent %s", filePath, lock.AgentID)
		}
	}

	for i, lock := range d.locks {
		if lock.FilePath == filePath && lock.AgentID == agentID {
			d.locks = append(d.locks[:i], d.locks[i+1:]...)
			break
		}
	}

	d.locks = append(d.locks, FileLock{
		FilePath:  filePath,
		AgentID:   agentID,
		ClaimedAt: time.Now(),
	})

	return nil
}

func (d *Detector) ReleaseFile(agentID, filePath string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, lock := range d.locks {
		if lock.FilePath == filePath && lock.AgentID == agentID {
			d.locks = append(d.locks[:i], d.locks[i+1:]...)
			return
		}
	}
}

func (d *Detector) ActiveConflicts() []Conflict {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make([]Conflict, len(d.conflicts))
	copy(result, d.conflicts)
	return result
}

func (d *Detector) LogConflict(c Conflict, logPath string) error {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(c)
}