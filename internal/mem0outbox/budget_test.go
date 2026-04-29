package mem0outbox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBudget_ProjectedSpendBelowCeilingDoesNotFreeze(t *testing.T) {
	b := &Budget{USDMax: 50.0, FreezeRatio: 0.80, CostPerCapsuleUSD: 0.005}
	// Spent = $20, projected batch of 100 = $0.50, projected total = $20.50,
	// ceiling = 0.80 * $50 = $40. Should NOT freeze.
	frozen, projected := b.ShouldFreeze(20.0, 100)
	if frozen {
		t.Fatalf("want not frozen, got frozen (projected=%.4f)", projected)
	}
	want := 20.0 + 100*0.005
	if projected < want-1e-9 || projected > want+1e-9 {
		t.Fatalf("projected=%.6f want=%.6f", projected, want)
	}
}

func TestBudget_ProjectedSpendAtCeilingFreezes(t *testing.T) {
	b := &Budget{USDMax: 50.0, FreezeRatio: 0.80, CostPerCapsuleUSD: 0.005}
	// Spent = $39.80, batch of 50 = $0.25, projected total = $40.05,
	// ceiling = 0.80 * $50 = $40. Should freeze (projected > ceiling).
	frozen, projected := b.ShouldFreeze(39.80, 50)
	if !frozen {
		t.Fatalf("want frozen, got not frozen (projected=%.4f)", projected)
	}
}

func TestBudget_ZeroOrNegativeMaxNeverFreezes(t *testing.T) {
	b := &Budget{USDMax: 0, FreezeRatio: 0.80, CostPerCapsuleUSD: 0.005}
	frozen, _ := b.ShouldFreeze(1000.0, 100)
	if frozen {
		t.Fatalf("USDMax<=0 must disable budget gate")
	}
}

func TestBudget_FromEnv(t *testing.T) {
	t.Setenv("MEM0_PAYG_USD_MAX", "37.5")
	b := BudgetFromEnv(0.005)
	if b.USDMax < 37.4999 || b.USDMax > 37.5001 {
		t.Fatalf("USDMax=%.4f want 37.5", b.USDMax)
	}
	if b.FreezeRatio < 0.79 || b.FreezeRatio > 0.81 {
		t.Fatalf("FreezeRatio=%.4f want 0.80", b.FreezeRatio)
	}
}

func TestBudget_FromEnvDefaultsTo50(t *testing.T) {
	t.Setenv("MEM0_PAYG_USD_MAX", "")
	b := BudgetFromEnv(0.005)
	if b.USDMax != 50.0 {
		t.Fatalf("USDMax=%.4f want 50.0 (default)", b.USDMax)
	}
}

func TestFileLedger_ReadEmptyIsZero(t *testing.T) {
	dir := t.TempDir()
	l := NewFileLedger(filepath.Join(dir, "spent_usd"))
	got, err := l.Read()
	if err != nil {
		t.Fatalf("Read empty: %v", err)
	}
	if got != 0 {
		t.Fatalf("Read empty: got %.4f want 0", got)
	}
}

func TestFileLedger_AddPersists(t *testing.T) {
	dir := t.TempDir()
	l := NewFileLedger(filepath.Join(dir, "spent_usd"))
	if _, err := l.Add(1.234); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := l.Add(2.5); err != nil {
		t.Fatalf("Add: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "spent_usd"))
	if err != nil {
		t.Fatalf("read spent_usd: %v", err)
	}
	got, err := strconv.ParseFloat(strings.TrimSpace(string(body)), 64)
	if err != nil {
		t.Fatalf("parse spent_usd: %v", err)
	}
	want := 1.234 + 2.5
	if got < want-1e-6 || got > want+1e-6 {
		t.Fatalf("file=%.6f want=%.6f", got, want)
	}
}

func TestFileLedger_ReadAfterAdd(t *testing.T) {
	dir := t.TempDir()
	l := NewFileLedger(filepath.Join(dir, "spent_usd"))
	_, _ = l.Add(7.5)

	l2 := NewFileLedger(filepath.Join(dir, "spent_usd"))
	got, err := l2.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got < 7.4999 || got > 7.5001 {
		t.Fatalf("got %.6f want 7.5", got)
	}
}

func TestFlusher_FreezesAtBudgetCeilingBeforeAnyPush(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(filepath.Join(dir, "pending.jsonl"))
	defer w.Close()
	for i := 0; i < 50; i++ {
		_ = w.Append(Capsule{ID: "c-" + strconv.Itoa(i), Text: "t"})
	}

	var calls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()

	cli := NewMem0Client(upstream.URL, "tok")
	cli.HTTP = upstream.Client()

	ledger := NewFileLedger(filepath.Join(dir, "spent_usd"))
	if _, err := ledger.Add(40.0); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	budget := &Budget{USDMax: 50.0, FreezeRatio: 0.80, CostPerCapsuleUSD: 0.005}

	flusher := &Flusher{
		PendingPath: filepath.Join(dir, "pending.jsonl"),
		CursorPath:  filepath.Join(dir, "cursor"),
		Client:      cli,
		BatchSize:   50,
		Budget:      budget,
		Ledger:      ledger,
	}

	report, err := flusher.Flush(context.Background())
	if !errors.Is(err, ErrBudgetFrozen) {
		t.Fatalf("want ErrBudgetFrozen, got err=%v report=%+v", err, report)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("upstream must NOT be called when frozen pre-batch, got %d calls", got)
	}
	cursorBytes, _ := os.ReadFile(filepath.Join(dir, "cursor"))
	if strings.TrimSpace(string(cursorBytes)) != "" && strings.TrimSpace(string(cursorBytes)) != "0" {
		t.Fatalf("cursor must be unchanged when frozen pre-batch, got %q", string(cursorBytes))
	}
}

func TestFlusher_FreezesMidBatchOnceCeilingHit(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(filepath.Join(dir, "pending.jsonl"))
	defer w.Close()
	for i := 0; i < 100; i++ {
		_ = w.Append(Capsule{ID: "c-" + strconv.Itoa(i), Text: "t"})
	}

	var calls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()

	cli := NewMem0Client(upstream.URL, "tok")
	cli.HTTP = upstream.Client()

	ledger := NewFileLedger(filepath.Join(dir, "spent_usd"))
	if _, err := ledger.Add(39.95); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	budget := &Budget{USDMax: 50.0, FreezeRatio: 0.80, CostPerCapsuleUSD: 0.005}

	flusher := &Flusher{
		PendingPath: filepath.Join(dir, "pending.jsonl"),
		CursorPath:  filepath.Join(dir, "cursor"),
		Client:      cli,
		BatchSize:   50,
		Budget:      budget,
		Ledger:      ledger,
	}

	report, err := flusher.Flush(context.Background())
	if !errors.Is(err, ErrBudgetFrozen) {
		t.Fatalf("want ErrBudgetFrozen, got err=%v report=%+v", err, report)
	}
	if report.Flushed != 10 {
		t.Fatalf("want 10 pushed before freeze (39.95 + 10*0.005 = $40.00 = ceiling), got %d", report.Flushed)
	}
	if got := atomic.LoadInt32(&calls); got != 10 {
		t.Fatalf("want 10 upstream calls, got %d", got)
	}
}

func TestFlusher_LoadTest500CapsulesUnderBudget(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(filepath.Join(dir, "pending.jsonl"))
	defer w.Close()
	const n = 500
	for i := 0; i < n; i++ {
		_ = w.Append(Capsule{
			ID:     "load-" + strconv.Itoa(i),
			AppID:  "cursor-global-kb",
			UserID: "macbook-replica",
			Text:   "load capsule " + strconv.Itoa(i),
			Metadata: map[string]string{
				"kind": "evoloop_cycle",
				"i":    strconv.Itoa(i),
			},
		})
	}

	var calls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer upstream.Close()
	cli := NewMem0Client(upstream.URL, "tok")
	cli.HTTP = upstream.Client()

	ledger := NewFileLedger(filepath.Join(dir, "spent_usd"))
	budget := &Budget{USDMax: 50.0, FreezeRatio: 0.80, CostPerCapsuleUSD: 0.005}

	flusher := &Flusher{
		PendingPath: filepath.Join(dir, "pending.jsonl"),
		CursorPath:  filepath.Join(dir, "cursor"),
		Client:      cli,
		BatchSize:   100,
		Budget:      budget,
		Ledger:      ledger,
	}

	totalFlushed := 0
	iters := 0
	for {
		report, err := flusher.Flush(context.Background())
		iters++
		if err != nil && !errors.Is(err, ErrBudgetFrozen) {
			t.Fatalf("iter %d: unexpected err=%v report=%+v", iters, err, report)
		}
		totalFlushed += report.Flushed
		if errors.Is(err, ErrBudgetFrozen) {
			t.Fatalf("iter %d: budget should not freeze for 500@$0.005 = $2.50 vs ceiling $40", iters)
		}
		if report.Flushed == 0 {
			break
		}
		if iters > 20 {
			t.Fatalf("did not converge after %d iters", iters)
		}
	}
	if totalFlushed != n {
		t.Fatalf("want %d flushed, got %d", n, totalFlushed)
	}
	if got := atomic.LoadInt32(&calls); got != int32(n) {
		t.Fatalf("upstream calls=%d want %d", got, n)
	}
	finalSpend, err := ledger.Read()
	if err != nil {
		t.Fatalf("ledger read: %v", err)
	}
	want := float64(n) * 0.005
	if finalSpend < want-1e-6 || finalSpend > want+1e-6 {
		t.Fatalf("ledger total=%.6f want=%.6f", finalSpend, want)
	}
}
