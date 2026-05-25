package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/nfsarch33/helix-dev-tools/internal/eval"
)

// EvalResult is a single eval grader verdict for the dashboard API.
type EvalResult struct {
	Timestamp string  `json:"timestamp"`
	EvalID    string  `json:"eval_id"`
	EvalName  string  `json:"eval_name"`
	Verdict   string  `json:"verdict"`
	Score     float64 `json:"score"`
	Details   string  `json:"details,omitempty"`
}

// EvalKPI aggregates eval results for the dashboard API.
type EvalKPI struct {
	TotalRuns  int            `json:"total_runs"`
	PassRate   float64        `json:"pass_rate"`
	ByEval     map[string]int `json:"by_eval"`
	RecentRuns []EvalResult   `json:"recent_runs"`
	LastRunAt  string         `json:"last_run_at,omitempty"`
}

func (s *Server) evalDBPath() string {
	if s.EvalDBPath != "" {
		return s.EvalDBPath
	}
	return eval.DefaultEvalDBPath()
}

func (s *Server) handleEvalResults(w http.ResponseWriter, r *http.Request) {
	store, err := eval.OpenEvalStore(s.evalDBPath())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emptyEvalKPI())
		return
	}
	defer store.Close()

	runs, err := store.RecentRuns(20)
	if err != nil || len(runs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emptyEvalKPI())
		return
	}

	kpi := computeEvalKPI(runs)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(kpi)
}

func emptyEvalKPI() EvalKPI {
	return EvalKPI{
		ByEval:     map[string]int{},
		RecentRuns: []EvalResult{},
	}
}

func computeEvalKPI(runs []eval.RunSummary) EvalKPI {
	kpi := EvalKPI{
		TotalRuns:  len(runs),
		ByEval:     make(map[string]int),
		RecentRuns: make([]EvalResult, 0, len(runs)),
	}

	if len(runs) == 0 {
		return kpi
	}

	totalPassed := 0
	totalEvals := 0
	for _, r := range runs {
		totalPassed += r.Passed
		totalEvals += r.Total

		verdict := "fail"
		if r.PassRate >= 1.0 {
			verdict = "pass"
		} else if r.PassRate > 0 {
			verdict = "partial"
		}

		kpi.RecentRuns = append(kpi.RecentRuns, EvalResult{
			Timestamp: r.StartedAt,
			EvalID:    r.RunID,
			EvalName:  r.RunID,
			Verdict:   verdict,
			Score:     r.AvgScore,
		})
		kpi.ByEval[r.RunID]++
	}

	if totalEvals > 0 {
		kpi.PassRate = float64(totalPassed) / float64(totalEvals)
	}
	kpi.LastRunAt = runs[0].StartedAt

	return kpi
}
