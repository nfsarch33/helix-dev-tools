package mem0bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Signal represents a cross-machine Mem0 coordination signal.
type Signal struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Ticket    string    `json:"ticket"`
	Summary   string    `json:"summary"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`
}

// OutboxEntry wraps a Signal with delivery state.
type OutboxEntry struct {
	Signal    Signal    `json:"signal"`
	Attempts  int       `json:"attempts"`
	LastTried time.Time `json:"last_tried"`
	Delivered bool      `json:"delivered"`
}

// Bridge coordinates Mem0 signal delivery with Git KB fallback.
type Bridge struct {
	mem0URL string
	kbDir   string

	mu     sync.Mutex
	outbox []OutboxEntry
}

// New creates a Bridge with the given Mem0 URL and Git KB fallback directory.
func New(mem0URL, kbDir string) *Bridge {
	return &Bridge{
		mem0URL: mem0URL,
		kbDir:   kbDir,
	}
}

// Enqueue adds a signal to the outbox for delivery.
func (b *Bridge) Enqueue(s Signal) {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.outbox = append(b.outbox, OutboxEntry{Signal: s})
}

// Flush attempts to deliver all pending outbox entries to Mem0.
// Failed deliveries fall back to Git KB. Returns count delivered.
func (b *Bridge) Flush() (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delivered := 0
	var lastErr error

	for i := range b.outbox {
		entry := &b.outbox[i]
		if entry.Delivered {
			continue
		}

		entry.Attempts++
		entry.LastTried = time.Now()

		if err := b.sendToMem0(entry.Signal); err != nil {
			if kbErr := b.writeToGitKB(entry.Signal); kbErr != nil {
				lastErr = fmt.Errorf("mem0: %w; git-kb: %v", err, kbErr)
				continue
			}
			entry.Delivered = true
			delivered++
			continue
		}
		entry.Delivered = true
		delivered++
	}

	return delivered, lastErr
}

// PendingCount returns the number of undelivered outbox entries.
func (b *Bridge) PendingCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, e := range b.outbox {
		if !e.Delivered {
			count++
		}
	}
	return count
}

// Subscribe polls Mem0 for new signals matching the given recipient.
// It falls back to scanning the Git KB directory when Mem0 is unreachable.
func (b *Bridge) Subscribe(recipient string) ([]Signal, error) {
	signals, err := b.pollMem0(recipient)
	if err != nil {
		return b.readFromGitKB(recipient)
	}
	return signals, nil
}

// Reconcile merges signals from Mem0 and Git KB, deduplicating by Signal.ID.
func (b *Bridge) Reconcile(recipient string) ([]Signal, error) {
	mem0Signals, _ := b.pollMem0(recipient)
	kbSignals, _ := b.readFromGitKB(recipient)

	seen := map[string]bool{}
	var merged []Signal

	for _, s := range mem0Signals {
		if !seen[s.ID] {
			seen[s.ID] = true
			merged = append(merged, s)
		}
	}
	for _, s := range kbSignals {
		if !seen[s.ID] {
			seen[s.ID] = true
			merged = append(merged, s)
		}
	}

	return merged, nil
}

func (b *Bridge) sendToMem0(s Signal) error {
	payload := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": fmt.Sprintf("handoff signal from=%s to=%s ticket=%s branch=%s: %s", s.From, s.To, s.Ticket, s.Branch, s.Summary)},
		},
		"user_id":   s.To,
		"metadata":  map[string]string{"signal_id": s.ID, "from": s.From, "ticket": s.Ticket, "branch": s.Branch},
		"app_id":    "cursor-coordination",
		"infer":     false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := http.Post(b.mem0URL+"/v1/memories/", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("mem0 returned %d", resp.StatusCode)
	}
	return nil
}

func (b *Bridge) writeToGitKB(s Signal) error {
	if b.kbDir == "" {
		return fmt.Errorf("no kbDir configured")
	}
	if err := os.MkdirAll(b.kbDir, 0755); err != nil {
		return err
	}
	filename := filepath.Join(b.kbDir, fmt.Sprintf("%s-%s.json", s.CreatedAt.Format("2006-01-02T150405"), s.ID))
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func (b *Bridge) pollMem0(recipient string) ([]Signal, error) {
	url := fmt.Sprintf("%s/v1/memories/?user_id=%s&app_id=cursor-coordination", b.mem0URL, recipient)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mem0 returned %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Metadata map[string]string `json:"metadata"`
			Memory   string            `json:"memory"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	var signals []Signal
	for _, r := range result.Results {
		if r.Metadata["signal_id"] != "" {
			signals = append(signals, Signal{
				ID:      r.Metadata["signal_id"],
				From:    r.Metadata["from"],
				To:      recipient,
				Ticket:  r.Metadata["ticket"],
				Branch:  r.Metadata["branch"],
				Summary: r.Memory,
			})
		}
	}
	return signals, nil
}

func (b *Bridge) readFromGitKB(recipient string) ([]Signal, error) {
	if b.kbDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(b.kbDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var signals []Signal
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(b.kbDir, e.Name()))
		if err != nil {
			continue
		}
		var s Signal
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		if s.To == recipient {
			signals = append(signals, s)
		}
	}
	return signals, nil
}
