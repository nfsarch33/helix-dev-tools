package mem0outbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// ErrBudgetFrozen is returned by Flusher.Flush when the projected
// USD spend exceeds the configured FreezeRatio of the per-period
// PAYG cap. Callers (typically a daemon) MUST stop pushing capsules
// to Mem0 until either: (a) the operator raises USDMax, (b) the
// next billing window resets the ledger, or (c) a separate process
// rotates the spent_usd ledger file.
//
// ErrBudgetFrozen is fundamentally different from ErrRateLimited:
// rate-limit is upstream-driven and self-resolving; budget-frozen
// is operator-driven and only resolves on operator action.
var ErrBudgetFrozen = errors.New("mem0 outbox frozen at PAYG budget ceiling")

// Budget describes the PAYG-cap policy for a single Mem0 outbox
// flusher. The values map directly to the Sprint v257 W1 D4-5
// requirement:
//
//	USDMax            = MEM0_PAYG_USD_MAX env var (default $50.00)
//	FreezeRatio       = freeze cursor at this fraction of USDMax
//	                    (default 0.80 -- "freeze at 80 % projected spend")
//	CostPerCapsuleUSD = per-capsule cost estimate used to project
//	                    the spend impact of the next batch before any
//	                    upstream call is made
//
// USDMax <= 0 disables the budget gate entirely (the flusher behaves
// exactly as it did before this change).
type Budget struct {
	USDMax            float64
	FreezeRatio       float64
	CostPerCapsuleUSD float64
}

// ShouldFreeze returns (true, projectedTotal) if processing batchSize
// more capsules at the current spent level would push the projected
// total at or above the FreezeRatio fraction of USDMax. The flusher
// uses this to gate-check before opening the pending file and to
// re-check after each successful push.
func (b *Budget) ShouldFreeze(spentUSD float64, batchSize int) (bool, float64) {
	if b == nil || b.USDMax <= 0 {
		return false, spentUSD
	}
	if batchSize < 0 {
		batchSize = 0
	}
	projected := spentUSD + float64(batchSize)*b.CostPerCapsuleUSD
	ceiling := b.USDMax * b.FreezeRatio
	if ceiling <= 0 {
		return false, projected
	}
	return projected >= ceiling, projected
}

// Ceiling reports the absolute USD ceiling (USDMax * FreezeRatio).
func (b *Budget) Ceiling() float64 {
	if b == nil || b.USDMax <= 0 {
		return 0
	}
	return b.USDMax * b.FreezeRatio
}

// BudgetFromEnv constructs a Budget from environment variables:
//
//	MEM0_PAYG_USD_MAX        absolute cap, defaults to $50.00
//	MEM0_PAYG_FREEZE_RATIO   freeze fraction, defaults to 0.80
//
// costPerCapsuleUSD is supplied by the caller (typically the CLI
// flag) so the operator can dial the conservativeness of the
// projection without redeploying the binary.
func BudgetFromEnv(costPerCapsuleUSD float64) *Budget {
	usdMax := 50.0
	if raw := strings.TrimSpace(os.Getenv("MEM0_PAYG_USD_MAX")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			usdMax = v
		}
	}
	ratio := 0.80
	if raw := strings.TrimSpace(os.Getenv("MEM0_PAYG_FREEZE_RATIO")); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 && v <= 1.0 {
			ratio = v
		}
	}
	if costPerCapsuleUSD <= 0 {
		costPerCapsuleUSD = 0.005
	}
	return &Budget{USDMax: usdMax, FreezeRatio: ratio, CostPerCapsuleUSD: costPerCapsuleUSD}
}

// SpendLedger persists cumulative USD spend across daemon restarts.
// Production callers use FileLedger; tests can supply their own.
type SpendLedger interface {
	Read() (float64, error)
	Add(delta float64) (float64, error)
}

// FileLedger persists the running USD spend total in an ASCII-decoded
// text file at Path. The file format is the Go default float64
// representation followed by a single trailing newline. Reads of a
// missing file return 0 (the steady-state on a fresh outbox).
//
// FileLedger is safe for concurrent use within a single process; cross
// process safety relies on the same per-machine assumption that the
// outbox already makes (one flusher daemon per machine).
type FileLedger struct {
	mu   sync.Mutex
	Path string
}

// NewFileLedger returns a FileLedger backed by path.
func NewFileLedger(path string) *FileLedger {
	return &FileLedger{Path: path}
}

// Read returns the current USD spend total. A missing file returns
// 0, which is also the canonical "fresh ledger" signal.
func (l *FileLedger) Read() (float64, error) {
	if l == nil || l.Path == "" {
		return 0, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.readLocked()
}

func (l *FileLedger) readLocked() (float64, error) {
	body, err := os.ReadFile(l.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read ledger %s: %w", l.Path, err)
	}
	s := strings.TrimSpace(string(body))
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse ledger %s (%q): %w", l.Path, s, err)
	}
	return v, nil
}

// Add increments the persisted total by delta and returns the new
// total. Add(0) is a normalising read: it materialises a fresh file
// containing 0.0000.
func (l *FileLedger) Add(delta float64) (float64, error) {
	if l == nil || l.Path == "" {
		return 0, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	cur, err := l.readLocked()
	if err != nil {
		return 0, err
	}
	next := cur + delta
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
		return 0, fmt.Errorf("mkdir ledger dir: %w", err)
	}
	body := []byte(strconv.FormatFloat(next, 'f', 6, 64) + "\n")
	tmp := l.Path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return 0, fmt.Errorf("write ledger tmp: %w", err)
	}
	if err := os.Rename(tmp, l.Path); err != nil {
		return 0, fmt.Errorf("rename ledger: %w", err)
	}
	return next, nil
}
