package debuglog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	logPath   = "/mnt/f/onedrive/repo/biz-stack/.cursor/debug-812135.log"
	sessionID = "812135"
)

type entry struct {
	SessionID    string      `json:"sessionId"`
	RunID        string      `json:"runId"`
	HypothesisID string      `json:"hypothesisId"`
	Location     string      `json:"location"`
	Message      string      `json:"message"`
	Data         interface{} `json:"data,omitempty"`
	Timestamp    int64       `json:"timestamp"`
}

func Write(runID, hypothesisID, location, message string, data interface{}) {
	payload, err := json.Marshal(entry{
		SessionID:    sessionID,
		RunID:        runID,
		HypothesisID: hypothesisID,
		Location:     location,
		Message:      message,
		Data:         data,
		Timestamp:    time.Now().UnixMilli(),
	})
	if err != nil {
		return
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(payload)
}
