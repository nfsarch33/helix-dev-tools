// Package mem0outbox implements an append-only NDJSON outbox for Mem0
// capsules with a byte-cursor-based flusher. It exists so the EvoLoop
// daemon (and other Mem0 writers) can keep emitting capsules locally
// during Mem0 quota outages or transient failures, and a separate
// background daemon can drain the outbox when Mem0 returns to health.
//
// Storage layout (per node):
//
//   - <root>/pending.jsonl            append-only NDJSON of capsules
//   - <root>/cursor                   ASCII byte offset of next-to-flush
//
// The cursor is a single integer (UTF-8) representing the number of
// pending.jsonl bytes that have been successfully posted to Mem0. A
// successful flush moves the cursor forward; a 429 freezes the cursor
// where the failure happened so a resumed flush picks up exactly where
// it left off without re-uploading already-flushed entries.
//
// This package never deletes pending.jsonl. A separate compactor (out
// of scope here) can rotate the file once cursor == size and the
// daemon's idle window has elapsed; until then the file is the durable
// authority that ADR-0004 calls out.
package mem0outbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrRateLimited is returned by Flusher.Flush when Mem0 responds with
// HTTP 429. Callers (typically a daemon) MUST honour the Retry-After
// duration on the returned FlushReport.
var ErrRateLimited = errors.New("mem0 rate limited (HTTP 429)")

// Capsule is the on-disk shape of a Mem0 capsule queued for upload.
// Fields mirror what the EvoLoop daemon already publishes so the flush
// is a transparent re-post.
type Capsule struct {
	ID       string            `json:"id,omitempty"`
	AppID    string            `json:"app_id"`
	UserID   string            `json:"user_id"`
	Text     string            `json:"text"`
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt, when present, is preserved verbatim so a delayed
	// flush still represents the capsule's original wall-clock time.
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// Writer appends capsules to a NDJSON outbox file. The Writer is safe
// for concurrent use; callers may share a single Writer across
// goroutines.
type Writer struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// NewWriter opens path for append, creating it (and the parent
// directory) if needed. The file is opened with O_APPEND so multiple
// processes writing concurrently do not interleave inside a line.
func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir outbox dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open outbox: %w", err)
	}
	return &Writer{path: path, f: f}, nil
}

// Append serialises c as one NDJSON line and writes it to disk. The
// write is fsync'd before Append returns so a crash mid-write cannot
// silently lose the capsule.
func (w *Writer) Append(c Capsule) error {
	if w == nil || w.f == nil {
		return fmt.Errorf("outbox writer not open")
	}
	line, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal capsule: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write outbox line: %w", err)
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("sync outbox: %w", err)
	}
	return nil
}

// Close releases the underlying file descriptor.
func (w *Writer) Close() error {
	if w == nil || w.f == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	err := w.f.Close()
	w.f = nil
	return err
}

// Reader inspects an outbox file. Tail returns the most recent N
// capsules across the entire file (not just the unflushed tail) so
// `cursor-tools evoloop recent --include-outbox` can surface local
// data even when Mem0 has the same capsule already.
type Reader struct {
	pendingPath string
	cursorPath  string
}

// NewReader constructs a Reader for the given outbox + cursor paths.
// cursorPath may be empty if the caller never intends to call
// PendingTail.
func NewReader(pendingPath, cursorPath string) *Reader {
	return &Reader{pendingPath: pendingPath, cursorPath: cursorPath}
}

// Tail returns the most recent N capsules from the outbox, deduped by
// ID (the most recent occurrence wins) and sorted newest-first.
func (r *Reader) Tail(n int) ([]Capsule, error) {
	if n <= 0 {
		return nil, nil
	}
	caps, err := r.readAll()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]int, len(caps))
	for i := range caps {
		key := caps[i].ID
		if key == "" {
			key = "@line:" + strconv.Itoa(i)
		}
		seen[key] = i // last occurrence wins
	}
	indices := make([]int, 0, len(seen))
	for _, idx := range seen {
		indices = append(indices, idx)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(indices)))
	picked := make([]Capsule, 0, len(indices))
	for _, idx := range indices {
		picked = append(picked, caps[idx])
	}
	if len(picked) > n {
		picked = picked[:n]
	}
	return picked, nil
}

func (r *Reader) readAll() ([]Capsule, error) {
	f, err := os.Open(r.pendingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var caps []Capsule
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var c Capsule
		if err := json.Unmarshal(line, &c); err != nil {
			continue
		}
		caps = append(caps, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return caps, nil
}

// FlushReport summarises one Flush call. Flushed counts capsules that
// successfully made it to Mem0; Skipped counts entries that could not
// be parsed and were advanced past anyway. RetryAfter is non-zero only
// when ErrRateLimited was returned.
type FlushReport struct {
	Flushed    int
	Skipped    int
	RetryAfter time.Duration
}

// Mem0Pusher is the surface Flusher needs from a Mem0 client. The
// production implementation lives in this package (Mem0Client); tests
// inject their own.
type Mem0Pusher interface {
	Push(ctx context.Context, c Capsule) error
}

// Flusher drains the outbox into Mem0. One Flusher per outbox file.
//
// Optional Budget + Ledger gate the flush against a PAYG USD cap. When
// either is nil the flusher behaves identically to the pre-budget
// release (only ErrRateLimited and corruption produce non-nil err).
//
// Optional Breaker enforces the v259 W2 D4 circuit-breaker contract:
// after TripThreshold consecutive 429s the breaker holds Open for a
// backoff window during which Flush returns ErrCircuitOpen without
// touching pending.jsonl. The breaker is nil-safe; when nil the
// flusher behaves as if no breaker were configured.
type Flusher struct {
	PendingPath string
	CursorPath  string
	Client      Mem0Pusher
	BatchSize   int

	// Budget describes the PAYG-cap policy. Nil disables the gate.
	Budget *Budget
	// Ledger persists cumulative USD spend. Nil disables the gate.
	Ledger SpendLedger
	// Breaker enforces the circuit-breaker contract. Nil disables
	// the gate (back-compat for pre-v259 callers).
	Breaker *CircuitBreaker
}

// Flush reads pending capsules starting at the cursor offset, posts
// each to Mem0 (one POST per capsule), and advances the cursor after
// every success. On HTTP 429 the cursor is left at the failure point
// and ErrRateLimited is returned with a populated RetryAfter.
//
// When Flusher.Budget and Flusher.Ledger are both non-nil, Flush also
// gates the batch against the PAYG-cap policy:
//
//   - Pre-batch: project the cost of pushing BatchSize capsules. If
//     that projected total >= FreezeRatio * USDMax, return
//     ErrBudgetFrozen WITHOUT opening pending.jsonl. The cursor is
//     not advanced.
//   - Mid-batch: after each successful push, increment the ledger by
//     CostPerCapsuleUSD and re-check. If the new total >= ceiling,
//     stop pushing further capsules in this batch and return
//     ErrBudgetFrozen with FlushReport.Flushed reflecting the partial
//     drain.
func (f *Flusher) Flush(ctx context.Context) (FlushReport, error) {
	report := FlushReport{}
	if f.Client == nil {
		return report, errors.New("flusher: nil Mem0 client")
	}

	limit := f.BatchSize
	if limit <= 0 {
		limit = 100
	}

	// Pre-batch breaker gate. Open or HalfOpen-with-issued-probe
	// short-circuits the flush so the daemon honours the backoff
	// window. We only check Allow() here; per-push outcomes feed
	// RecordSuccess / RecordFailure inside the loop.
	if f.Breaker != nil && !f.Breaker.Allow() {
		return report, ErrCircuitOpen
	}

	// Pre-batch budget gate. The flusher refuses to even open
	// pending.jsonl when the operator has already burned the entire
	// FreezeRatio share of USDMax. Pre-batch fires only on "already
	// at the ceiling" -- the per-push gate below handles the case
	// where a partial drain is still safe.
	if f.budgetGateActive() {
		spent, err := f.Ledger.Read()
		if err != nil {
			return report, fmt.Errorf("read spend ledger: %w", err)
		}
		if frozen, projected := f.Budget.ShouldFreeze(spent, 0); frozen {
			return report, fmt.Errorf("pre-batch spend $%.4f >= ceiling $%.4f: %w", projected, f.Budget.Ceiling(), ErrBudgetFrozen)
		}
	}

	in, err := os.Open(f.PendingPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return report, nil
		}
		return report, fmt.Errorf("open pending: %w", err)
	}
	defer in.Close()

	cursor, err := readCursor(f.CursorPath)
	if err != nil {
		return report, fmt.Errorf("read cursor: %w", err)
	}
	if cursor > 0 {
		if _, err := in.Seek(cursor, io.SeekStart); err != nil {
			return report, fmt.Errorf("seek to cursor: %w", err)
		}
	}

	br := bufio.NewReader(in)
	offset := cursor
	processed := 0
	for processed < limit {
		line, err := br.ReadBytes('\n')
		if len(line) == 0 && errors.Is(err, io.EOF) {
			break
		}
		// Track the offset BEFORE attempting upload so a 429 leaves
		// the cursor on the failed line, not past it.
		lineLen := int64(len(line))
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			offset += lineLen
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		var c Capsule
		if jerr := json.Unmarshal(trimmed, &c); jerr != nil {
			report.Skipped++
			offset += lineLen
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		pushErr := f.Client.Push(ctx, c)
		if pushErr != nil {
			var rl *RateLimitedError
			if errors.As(pushErr, &rl) {
				if f.Breaker != nil {
					f.Breaker.RecordFailure()
				}
				if writeErr := writeCursor(f.CursorPath, offset); writeErr != nil {
					return report, fmt.Errorf("write cursor on 429: %w", writeErr)
				}
				report.RetryAfter = rl.RetryAfter
				return report, fmt.Errorf("after %d flushed: %w", report.Flushed, ErrRateLimited)
			}
			return report, fmt.Errorf("push capsule (offset=%d): %w", offset, pushErr)
		}
		if f.Breaker != nil {
			f.Breaker.RecordSuccess()
		}
		offset += lineLen
		report.Flushed++
		processed++
		if writeErr := writeCursor(f.CursorPath, offset); writeErr != nil {
			return report, fmt.Errorf("write cursor: %w", writeErr)
		}

		// Mid-batch budget gate. The cursor has already advanced past
		// this push; we only stop the *next* push, never roll back a
		// successful one. This guarantees the ledger's running total
		// is always a lower bound on actual spend.
		if f.budgetGateActive() {
			newSpend, addErr := f.Ledger.Add(f.Budget.CostPerCapsuleUSD)
			if addErr != nil {
				return report, fmt.Errorf("add to spend ledger: %w", addErr)
			}
			if newSpend >= f.Budget.Ceiling() {
				return report, fmt.Errorf("post-push spend $%.4f >= ceiling $%.4f: %w", newSpend, f.Budget.Ceiling(), ErrBudgetFrozen)
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}
	return report, nil
}

// budgetGateActive reports whether the flusher is configured for
// PAYG-cap enforcement. Both Budget and Ledger must be non-nil for
// the gate to fire.
func (f *Flusher) budgetGateActive() bool {
	return f != nil && f.Budget != nil && f.Ledger != nil && f.Budget.USDMax > 0
}

func readCursor(path string) (int64, error) {
	if path == "" {
		return 0, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	s := strings.TrimSpace(string(body))
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

func writeCursor(path string, off int64) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.FormatInt(off, 10)), 0o644)
}

// RateLimitedError signals an HTTP 429 from Mem0 with a populated
// Retry-After. Flusher.Flush wraps it in ErrRateLimited so callers can
// errors.Is it.
type RateLimitedError struct {
	RetryAfter time.Duration
}

// Error reports the error message.
func (e *RateLimitedError) Error() string {
	if e == nil {
		return "rate limited"
	}
	return fmt.Sprintf("mem0 rate limited; retry after %s", e.RetryAfter)
}

// Mem0Client is a thin Mem0 v1 client used by the flusher. It POSTs a
// canonical envelope to /v1/memories/ on every Push.
type Mem0Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewMem0Client returns a Mem0 client targeted at baseURL (no trailing
// slash) authenticating with the given API token.
func NewMem0Client(baseURL, apiKey string) *Mem0Client {
	return &Mem0Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Push uploads one capsule to Mem0. It returns *RateLimitedError on
// HTTP 429 (caller must errors.As).
func (c *Mem0Client) Push(ctx context.Context, cap Capsule) error {
	envelope := map[string]any{
		"messages": []map[string]string{
			{"role": "user", "content": cap.Text},
		},
		"app_id":   cap.AppID,
		"user_id":  cap.UserID,
		"metadata": cap.Metadata,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/memories/", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+c.APIKey)

	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		secs, _ := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
		if secs <= 0 {
			secs = 30
		}
		return &RateLimitedError{RetryAfter: time.Duration(secs) * time.Second}
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return fmt.Errorf("mem0 push: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
