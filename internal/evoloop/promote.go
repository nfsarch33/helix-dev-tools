package evoloop

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/mem0outbox"
)

// PromotionState distinguishes the outcomes the promoter can produce
// for a single candidate capsule.
type PromotionState string

const (
	// PromotionStatePromoted means the candidate satisfied the
	// "improved cycles + non-negative mean delta" rule and the TDD
	// gate command exited successfully.
	PromotionStatePromoted PromotionState = "promoted"
	// PromotionStateGateFailed means the capsule looks like a valid
	// candidate but the TDD gate command failed, so promotion is
	// withheld.
	PromotionStateGateFailed PromotionState = "gate_failed"
	// PromotionStateSkipped means the capsule was inspected but did
	// not meet the candidate rule (no improved cycles, negative mean
	// delta, missing required metadata, or already promoted).
	PromotionStateSkipped PromotionState = "skipped"
	// PromotionStateRolledBack means the rolling-window check fired
	// for a previously-promoted capsule and a rollback record was
	// emitted.
	PromotionStateRolledBack PromotionState = "rolled_back"
)

// PromotionDecision is the deterministic output of a single capsule
// evaluation. It is JSON-serialisable so the CLI can emit machine-
// readable evidence without round-tripping through the Mem0 schema.
type PromotionDecision struct {
	CapsuleID    string         `json:"capsule_id"`
	Machine      string         `json:"machine"`
	Day          string         `json:"day,omitempty"`
	Kind         CapsuleKind    `json:"kind"`
	State        PromotionState `json:"state"`
	Reason       string         `json:"reason,omitempty"`
	GateExitCode int            `json:"gate_exit_code,omitempty"`
	GateStdout   string         `json:"gate_stdout,omitempty"`
	GateStderr   string         `json:"gate_stderr,omitempty"`
	LastKPI      float64        `json:"last_kpi,omitempty"`
	MeanDelta    float64        `json:"mean_delta,omitempty"`
	Improved     int            `json:"improved,omitempty"`
	RolledBack   int            `json:"rolled_back,omitempty"`
}

// PromotionCriteria tunes which capsules are eligible for promotion.
// Defaults are conservative: at least one cycle must have improved and
// the mean KPI delta must be non-negative.
type PromotionCriteria struct {
	MinImproved  int
	MinMeanDelta float64
	OnlyMachines []string
}

// DefaultPromotionCriteria returns the conservative default.
func DefaultPromotionCriteria() PromotionCriteria {
	return PromotionCriteria{
		MinImproved:  1,
		MinMeanDelta: 0.0,
	}
}

// IsPromotionCandidate returns true when the capsule looks like a
// rollup that genuinely shows improvement and is therefore worth
// promoting (subject to the TDD gate).
func IsPromotionCandidate(c Capsule, crit PromotionCriteria) bool {
	if c.Kind != KindRollup {
		return false
	}
	if c.Improved < crit.MinImproved {
		return false
	}
	if c.MeanDelta < crit.MinMeanDelta {
		return false
	}
	if len(crit.OnlyMachines) > 0 {
		match := false
		for _, m := range crit.OnlyMachines {
			if m == c.Machine {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

// RegressionWindow is the rolling-window stats used by the rollback
// detector. Mean and Stdev are computed across LastKPI samples in the
// inspected window.
type RegressionWindow struct {
	Machine string
	From    time.Time
	To      time.Time
	Samples int
	Mean    float64
	Stdev   float64
	Latest  float64
}

// RollingKPIWindow computes the rolling-window stats over rollup
// capsules emitted by the supplied machine within [from, to]. Returns
// zero-valued stats when fewer than two samples are available (a single
// reading cannot regress against itself).
func RollingKPIWindow(rollups []Capsule, machine string, from, to time.Time) RegressionWindow {
	w := RegressionWindow{Machine: machine, From: from, To: to}
	xs := make([]float64, 0, len(rollups))
	var latest float64
	var latestAt time.Time
	for _, c := range rollups {
		if c.Kind != KindRollup {
			continue
		}
		if machine != "" && c.Machine != machine {
			continue
		}
		if !c.CreatedAt.IsZero() && (c.CreatedAt.Before(from) || c.CreatedAt.After(to)) {
			continue
		}
		xs = append(xs, c.LastKPI)
		if c.CreatedAt.After(latestAt) {
			latest = c.LastKPI
			latestAt = c.CreatedAt
		}
	}
	if len(xs) < 2 {
		return w
	}
	sum := 0.0
	for _, v := range xs {
		sum += v
	}
	mean := sum / float64(len(xs))
	var sq float64
	for _, v := range xs {
		d := v - mean
		sq += d * d
	}
	stdev := math.Sqrt(sq / float64(len(xs)-1))
	w.Samples = len(xs)
	w.Mean = mean
	w.Stdev = stdev
	w.Latest = latest
	return w
}

// IsRegression reports whether the latest sample falls below the
// rolling-window mean by at least sigma standard deviations. Windows
// with fewer than two samples never regress; windows with stdev=0 only
// regress when the latest sample is strictly below the mean (avoiding
// division by zero while still flagging hard regressions on flat
// histories).
func IsRegression(w RegressionWindow, sigma float64) bool {
	if w.Samples < 2 {
		return false
	}
	if w.Stdev == 0 {
		return w.Latest < w.Mean
	}
	delta := w.Mean - w.Latest
	return delta >= sigma*w.Stdev
}

// BuildPromotionCapsule constructs the outbox entry for a promoted
// capsule. The caller supplies a deterministic now() source so tests
// can pin the timestamp.
func BuildPromotionCapsule(d PromotionDecision, source Capsule, userID string, now func() time.Time) mem0outbox.Capsule {
	if now == nil {
		now = time.Now
	}
	ts := now().UTC()
	id := fmt.Sprintf("%d-evoloop-promotion-%s", ts.Unix(), source.ID)
	text := strings.TrimSpace(fmt.Sprintf(
		"evoloop promotion %s machine=%s day=%s improved=%d rolled_back=%d mean_delta=%+.3f last_kpi=%.3f gate_exit=%d",
		source.ID, source.Machine, source.Day, source.Improved, source.RolledBack,
		source.MeanDelta, source.LastKPI, d.GateExitCode))
	meta := map[string]string{
		"kind":           "evoloop_promotion",
		"source_capsule": source.ID,
		"machine":        source.Machine,
		"day":            source.Day,
		"improved":       fmt.Sprintf("%d", source.Improved),
		"rolled_back":    fmt.Sprintf("%d", source.RolledBack),
		"mean_delta":     fmt.Sprintf("%.6f", source.MeanDelta),
		"last_kpi":       fmt.Sprintf("%.6f", source.LastKPI),
		"gate_exit_code": fmt.Sprintf("%d", d.GateExitCode),
		"actor":          "cursor-tools-evoloop-promote",
	}
	return mem0outbox.Capsule{
		ID:        id,
		AppID:     "cursor-global-kb",
		UserID:    userID,
		Text:      text,
		Metadata:  meta,
		CreatedAt: ts,
	}
}

// BuildRollbackCapsule constructs an outbox entry that records a
// rollback decision against a previously-promoted capsule. It mirrors
// the promotion shape so downstream readers can correlate the two.
func BuildRollbackCapsule(promotionID string, w RegressionWindow, sigma float64, userID string, now func() time.Time) mem0outbox.Capsule {
	if now == nil {
		now = time.Now
	}
	ts := now().UTC()
	id := fmt.Sprintf("%d-evoloop-rollback-%s", ts.Unix(), promotionID)
	text := fmt.Sprintf("evoloop rollback %s machine=%s mean=%.3f stdev=%.3f latest=%.3f sigma_threshold=%.2f",
		promotionID, w.Machine, w.Mean, w.Stdev, w.Latest, sigma)
	meta := map[string]string{
		"kind":              "evoloop_rollback",
		"source_promotion":  promotionID,
		"machine":           w.Machine,
		"window_mean":       fmt.Sprintf("%.6f", w.Mean),
		"window_stdev":      fmt.Sprintf("%.6f", w.Stdev),
		"window_latest_kpi": fmt.Sprintf("%.6f", w.Latest),
		"window_samples":    fmt.Sprintf("%d", w.Samples),
		"sigma_threshold":   fmt.Sprintf("%.6f", sigma),
		"actor":             "cursor-tools-evoloop-promote",
	}
	return mem0outbox.Capsule{
		ID:        id,
		AppID:     "cursor-global-kb",
		UserID:    userID,
		Text:      text,
		Metadata:  meta,
		CreatedAt: ts,
	}
}

// AlreadyPromoted reports whether a previously-emitted capsule from
// the given fleet snapshot already promoted source.ID. The check is
// purely metadata-driven so it works against the same Mem0 stream the
// reader pulls from.
func AlreadyPromoted(prior []Capsule, sourceID string) bool {
	for _, c := range prior {
		if c.Metadata == nil {
			continue
		}
		if c.Metadata["kind"] == "evoloop_promotion" && c.Metadata["source_capsule"] == sourceID {
			return true
		}
	}
	return false
}

// SortDecisionsByCapsuleID sorts a slice of promotion decisions in a
// deterministic order so JSON snapshots are stable across runs.
func SortDecisionsByCapsuleID(d []PromotionDecision) {
	sort.SliceStable(d, func(i, j int) bool { return d[i].CapsuleID < d[j].CapsuleID })
}

// FindPromotionForRollup returns the most recent prior promotion
// capsule whose metadata.source_capsule matches sourceID, or nil if
// none exists. Used to thread rollback capsules back to the original
// promotion they should invalidate.
func FindPromotionForRollup(prior []Capsule, sourceID string) *Capsule {
	var best *Capsule
	for i := range prior {
		c := &prior[i]
		if c.Metadata == nil {
			continue
		}
		if c.Metadata["kind"] != "evoloop_promotion" {
			continue
		}
		if c.Metadata["source_capsule"] != sourceID {
			continue
		}
		if best == nil || c.CreatedAt.After(best.CreatedAt) {
			best = c
		}
	}
	return best
}
