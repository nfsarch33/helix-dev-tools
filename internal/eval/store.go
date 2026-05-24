package eval

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// EvalStore persists eval run results in a SQLite database.
type EvalStore struct {
	db *sql.DB
}

func DefaultEvalDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "helix-dev-tools", "eval.db")
}

func OpenEvalStore(dbPath string) (*EvalStore, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create eval db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open eval db: %w", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	db.SetMaxOpenConns(1)

	s := &EvalStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *EvalStore) Close() error { return s.db.Close() }

func (s *EvalStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS eval_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			eval_id TEXT NOT NULL,
			eval_name TEXT,
			eval_type TEXT,
			pass BOOLEAN NOT NULL,
			score REAL,
			duration_ms INTEGER,
			criteria_json TEXT,
			error TEXT,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_eval_runs_run_id ON eval_runs(run_id);
		CREATE INDEX IF NOT EXISTS idx_eval_runs_created ON eval_runs(created_at);
	`)
	return err
}

// SaveResult persists a single eval result.
func (s *EvalStore) SaveResult(runID string, r EvalResult) error {
	criteriaJSON, _ := json.Marshal(r.Criteria)
	_, err := s.db.Exec(
		`INSERT INTO eval_runs (run_id, eval_id, eval_name, eval_type, pass, score, duration_ms, criteria_json, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID, r.EvalID, r.EvalName, string(r.EvalType), r.Pass, r.Score,
		r.DurationMS, string(criteriaJSON), r.Error,
		time.Now().Format(time.RFC3339),
	)
	return err
}

// RecentRuns returns the most recent N run summaries.
func (s *EvalStore) RecentRuns(limit int) ([]RunSummary, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`
		SELECT run_id, COUNT(*) as total, SUM(CASE WHEN pass THEN 1 ELSE 0 END) as passed,
		       AVG(score) as avg_score, MIN(created_at) as started
		FROM eval_runs
		GROUP BY run_id
		ORDER BY started DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []RunSummary
	for rows.Next() {
		var r RunSummary
		if err := rows.Scan(&r.RunID, &r.Total, &r.Passed, &r.AvgScore, &r.StartedAt); err != nil {
			return nil, err
		}
		r.PassRate = float64(r.Passed) / float64(r.Total)
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

type RunSummary struct {
	RunID     string  `json:"run_id"`
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	PassRate  float64 `json:"pass_rate"`
	AvgScore  float64 `json:"avg_score"`
	StartedAt string  `json:"started_at"`
}
