package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/eval"
)

func TestComputeEvalKPI_Empty(t *testing.T) {
	kpi := computeEvalKPI(nil)
	if kpi.TotalRuns != 0 {
		t.Errorf("expected 0 total runs, got %d", kpi.TotalRuns)
	}
	if kpi.PassRate != 0 {
		t.Errorf("expected 0 pass rate, got %f", kpi.PassRate)
	}
	if len(kpi.RecentRuns) != 0 {
		t.Errorf("expected empty recent runs, got %d", len(kpi.RecentRuns))
	}
	if kpi.ByEval == nil {
		t.Error("expected non-nil ByEval map")
	}
}

func TestComputeEvalKPI_AllPass(t *testing.T) {
	runs := []eval.RunSummary{
		{RunID: "run-1", Total: 5, Passed: 5, PassRate: 1.0, AvgScore: 0.95, StartedAt: "2026-05-26T09:00:00+10:00"},
		{RunID: "run-2", Total: 3, Passed: 3, PassRate: 1.0, AvgScore: 0.88, StartedAt: "2026-05-25T09:00:00+10:00"},
	}

	kpi := computeEvalKPI(runs)
	if kpi.TotalRuns != 2 {
		t.Errorf("expected 2 total runs, got %d", kpi.TotalRuns)
	}
	if kpi.PassRate != 1.0 {
		t.Errorf("expected 1.0 pass rate, got %f", kpi.PassRate)
	}
	if kpi.LastRunAt != "2026-05-26T09:00:00+10:00" {
		t.Errorf("unexpected last run at: %s", kpi.LastRunAt)
	}
	for _, r := range kpi.RecentRuns {
		if r.Verdict != "pass" {
			t.Errorf("expected pass verdict for %s, got %s", r.EvalID, r.Verdict)
		}
	}
}

func TestComputeEvalKPI_MixedResults(t *testing.T) {
	runs := []eval.RunSummary{
		{RunID: "run-1", Total: 5, Passed: 5, PassRate: 1.0, AvgScore: 0.95, StartedAt: "2026-05-26T09:00:00+10:00"},
		{RunID: "run-2", Total: 4, Passed: 2, PassRate: 0.5, AvgScore: 0.60, StartedAt: "2026-05-25T09:00:00+10:00"},
		{RunID: "run-3", Total: 3, Passed: 0, PassRate: 0.0, AvgScore: 0.10, StartedAt: "2026-05-24T09:00:00+10:00"},
	}

	kpi := computeEvalKPI(runs)
	if kpi.TotalRuns != 3 {
		t.Errorf("expected 3 total runs, got %d", kpi.TotalRuns)
	}

	expectedPassRate := float64(7) / float64(12)
	if kpi.PassRate < expectedPassRate-0.01 || kpi.PassRate > expectedPassRate+0.01 {
		t.Errorf("expected pass rate ~%.3f, got %.3f", expectedPassRate, kpi.PassRate)
	}

	if kpi.RecentRuns[0].Verdict != "pass" {
		t.Errorf("run-1 should be pass, got %s", kpi.RecentRuns[0].Verdict)
	}
	if kpi.RecentRuns[1].Verdict != "partial" {
		t.Errorf("run-2 should be partial, got %s", kpi.RecentRuns[1].Verdict)
	}
	if kpi.RecentRuns[2].Verdict != "fail" {
		t.Errorf("run-3 should be fail, got %s", kpi.RecentRuns[2].Verdict)
	}
}

func TestHandleEvalResults_NoDB(t *testing.T) {
	s := &Server{EvalDBPath: "/nonexistent/path/eval.db"}

	req := httptest.NewRequest(http.MethodGet, "/api/eval/results", nil)
	w := httptest.NewRecorder()

	s.handleEvalResults(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var kpi EvalKPI
	if err := json.NewDecoder(w.Body).Decode(&kpi); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if kpi.TotalRuns != 0 {
		t.Errorf("expected 0 total runs for missing DB, got %d", kpi.TotalRuns)
	}
	if kpi.ByEval == nil {
		t.Error("expected non-nil ByEval map")
	}
	if kpi.RecentRuns == nil {
		t.Error("expected non-nil RecentRuns slice")
	}
}

func TestHandleEvalResults_WithStore(t *testing.T) {
	dbPath := t.TempDir() + "/eval.db"

	store, err := eval.OpenEvalStore(dbPath)
	if err != nil {
		t.Fatalf("open eval store: %v", err)
	}

	if err := store.SaveResult("run-a", eval.EvalResult{
		EvalID: "e1", EvalName: "test-eval", EvalType: eval.EvalCapability,
		Pass: true, Score: 0.9, DurationMS: 100,
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}
	if err := store.SaveResult("run-a", eval.EvalResult{
		EvalID: "e2", EvalName: "test-eval-2", EvalType: eval.EvalCapability,
		Pass: true, Score: 0.8, DurationMS: 200,
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}
	if err := store.SaveResult("run-b", eval.EvalResult{
		EvalID: "e1", EvalName: "test-eval", EvalType: eval.EvalRegression,
		Pass: false, Score: 0.3, DurationMS: 150,
	}); err != nil {
		t.Fatalf("save result: %v", err)
	}
	store.Close()

	s := &Server{EvalDBPath: dbPath}
	req := httptest.NewRequest(http.MethodGet, "/api/eval/results", nil)
	w := httptest.NewRecorder()

	s.handleEvalResults(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var kpi EvalKPI
	if err := json.NewDecoder(w.Body).Decode(&kpi); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if kpi.TotalRuns != 2 {
		t.Errorf("expected 2 runs (run-a, run-b), got %d", kpi.TotalRuns)
	}
	if kpi.LastRunAt == "" {
		t.Error("expected non-empty LastRunAt")
	}
	if len(kpi.RecentRuns) != 2 {
		t.Errorf("expected 2 recent runs, got %d", len(kpi.RecentRuns))
	}
}

func TestHandleEvalResults_EmptyStore(t *testing.T) {
	dbPath := t.TempDir() + "/eval-empty.db"

	store, err := eval.OpenEvalStore(dbPath)
	if err != nil {
		t.Fatalf("open eval store: %v", err)
	}
	store.Close()

	s := &Server{EvalDBPath: dbPath}
	req := httptest.NewRequest(http.MethodGet, "/api/eval/results", nil)
	w := httptest.NewRecorder()

	s.handleEvalResults(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var kpi EvalKPI
	if err := json.NewDecoder(w.Body).Decode(&kpi); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if kpi.TotalRuns != 0 {
		t.Errorf("expected 0 total runs for empty store, got %d", kpi.TotalRuns)
	}
}
