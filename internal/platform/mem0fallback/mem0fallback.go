package mem0fallback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	Text      string            `json:"text"`
	UserID    string            `json:"user_id"`
	AppID     string            `json:"app_id"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	Attempts  int               `json:"attempts"`
	LastError string            `json:"last_error,omitempty"`
}

type Outbox struct {
	mu      sync.Mutex
	path    string
	entries []Entry
}

func New(path string) (*Outbox, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	o := &Outbox{path: path}
	if err := o.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return o, nil
}

func DefaultPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "helix-dev-tools", "mem0-outbox.json")
}

func (o *Outbox) Enqueue(text, userID, appID string, metadata map[string]string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.entries = append(o.entries, Entry{
		Text:      text,
		UserID:    userID,
		AppID:     appID,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	})
	o.save()
}

func (o *Outbox) Pending() []Entry {
	o.mu.Lock()
	defer o.mu.Unlock()
	result := make([]Entry, len(o.entries))
	copy(result, o.entries)
	return result
}

func (o *Outbox) Len() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.entries)
}

func (o *Outbox) MarkFailed(idx int, err string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if idx < 0 || idx >= len(o.entries) {
		return
	}
	o.entries[idx].Attempts++
	o.entries[idx].LastError = err
	o.save()
}

func (o *Outbox) Remove(idx int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if idx < 0 || idx >= len(o.entries) {
		return
	}
	o.entries = append(o.entries[:idx], o.entries[idx+1:]...)
	o.save()
}

func (o *Outbox) Clear() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.entries = nil
	o.save()
}

func (o *Outbox) load() error {
	data, err := os.ReadFile(o.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &o.entries)
}

func (o *Outbox) save() {
	data, _ := json.MarshalIndent(o.entries, "", "  ")
	_ = os.WriteFile(o.path, data, 0600)
}
