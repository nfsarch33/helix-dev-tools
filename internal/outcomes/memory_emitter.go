package outcomes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AppIDFleetOutcomes is the unified app namespace for agent_outcome capsules
// (see internal/evoloop/kinds.go for the canonical kinds and the v253 day 2
// namespace unification work).
const AppIDFleetOutcomes = "cursor-global-kb"

// DefaultMem0BaseURL is the managed Mem0 endpoint.
const DefaultMem0BaseURL = "https://api.mem0.ai"

// MemoryEmitterConfig configures the synchronous Mem0 sink. APIKey + UserID
// are required.
type MemoryEmitterConfig struct {
	APIKey     string
	UserID     string
	AppID      string
	BaseURL    string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
	HTTP       *http.Client
}

// MemoryEmitter publishes outcomes directly to Mem0 via the v1 memories API.
type MemoryEmitter struct {
	cfg  MemoryEmitterConfig
	http *http.Client
}

// NewMemoryEmitter creates a sink with sensible defaults applied.
func NewMemoryEmitter(cfg MemoryEmitterConfig) *MemoryEmitter {
	if cfg.AppID == "" {
		cfg.AppID = AppIDFleetOutcomes
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultMem0BaseURL
	} else {
		cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 2
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 200 * time.Millisecond
	}
	hc := cfg.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: cfg.Timeout}
	}
	return &MemoryEmitter{cfg: cfg, http: hc}
}

// Emit publishes a single outcome to Mem0. The payload mirrors the schema used
// by internal/coordination/client.go so the operational story stays uniform.
func (m *MemoryEmitter) Emit(ctx context.Context, o Outcome) error {
	o.Normalize()
	if err := o.Validate(); err != nil {
		return fmt.Errorf("memory emit: %w", err)
	}
	if m.cfg.APIKey == "" {
		return fmt.Errorf("memory emit: APIKey is required")
	}
	if m.cfg.UserID == "" {
		return fmt.Errorf("memory emit: UserID is required")
	}

	text := o.Mem0Text()
	payload := map[string]interface{}{
		"user_id": m.cfg.UserID,
		"app_id":  m.cfg.AppID,
		"text":    text,
		"messages": []map[string]string{
			{"role": "user", "content": text},
		},
		"metadata": o.Mem0Metadata(),
		"infer":    false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= m.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.cfg.RetryDelay * time.Duration(attempt)):
			}
		}
		err := m.doRequest(ctx, data)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableErr(err) {
			return err
		}
	}
	return lastErr
}

func (m *MemoryEmitter) doRequest(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.cfg.BaseURL+"/v1/memories/", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+m.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := m.http.Do(req)
	if err != nil {
		return retryable(fmt.Errorf("send request: %w", err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	statusErr := fmt.Errorf("mem0 status=%d body=%s",
		resp.StatusCode, strings.TrimSpace(string(body)))
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return retryable(statusErr)
	}
	return statusErr
}

type retryableError struct{ err error }

func (r *retryableError) Error() string { return r.err.Error() }
func (r *retryableError) Unwrap() error { return r.err }

func retryable(err error) error { return &retryableError{err: err} }

func isRetryableErr(err error) bool {
	var r *retryableError
	if err == nil {
		return false
	}
	if asRetryable(err, &r) {
		return true
	}
	return false
}

// asRetryable is a tiny helper to avoid importing errors only for As().
func asRetryable(err error, target **retryableError) bool {
	for err != nil {
		if r, ok := err.(*retryableError); ok {
			*target = r
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
