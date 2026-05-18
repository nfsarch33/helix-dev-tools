package sprintboard

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type TicketStatus string

const (
	StatusBacklog       TicketStatus = "backlog"
	StatusReady         TicketStatus = "ready"
	StatusInProgress    TicketStatus = "in_progress"
	StatusReview        TicketStatus = "review"
	StatusDone          TicketStatus = "done"
	StatusBlocked       TicketStatus = "blocked"
	StatusReadyHandoff  TicketStatus = "ready_for_handoff"
)

type SprintStatus string

const (
	SprintPlanned  SprintStatus = "planned"
	SprintActive   SprintStatus = "active"
	SprintClosed   SprintStatus = "closed"
)

type Sprint struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Status     SprintStatus `json:"status"`
	OwnerAgent string       `json:"owner_agent"`
	Theme      string       `json:"theme,omitempty"`
	StartAt    time.Time    `json:"start_at,omitempty"`
	EndAt      time.Time    `json:"end_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

type Ticket struct {
	ID                 string       `json:"id"`
	SprintID           string       `json:"sprint_id,omitempty"`
	Title              string       `json:"title"`
	Description        string       `json:"description,omitempty"`
	Status             TicketStatus `json:"status"`
	OwnerAgent         string       `json:"owner_agent,omitempty"`
	Priority           int          `json:"priority"`
	AcceptanceCriteria string       `json:"acceptance_criteria,omitempty"`
	HandoffDocPath     string       `json:"handoff_doc_path,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

type Transition struct {
	ID         int64        `json:"id"`
	TicketID   string       `json:"ticket_id"`
	FromStatus TicketStatus `json:"from_status"`
	ToStatus   TicketStatus `json:"to_status"`
	AgentID    string       `json:"agent_id"`
	Note       string       `json:"note,omitempty"`
	Timestamp  time.Time    `json:"timestamp"`
}

type Handoff struct {
	ID          int64     `json:"id"`
	TicketID    string    `json:"ticket_id"`
	FromAgent   string    `json:"from_agent"`
	ToAgent     string    `json:"to_agent"`
	ContextPath string    `json:"context_path,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

func DefaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "helix-dev-tools", "sprintboard.db")
}

func Open(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sprints (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'planned',
		owner_agent TEXT,
		theme TEXT,
		start_at TEXT,
		end_at TEXT,
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tickets (
		id TEXT PRIMARY KEY,
		sprint_id TEXT,
		title TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL DEFAULT 'backlog',
		owner_agent TEXT,
		priority INTEGER DEFAULT 0,
		acceptance_criteria TEXT,
		handoff_doc_path TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		FOREIGN KEY (sprint_id) REFERENCES sprints(id)
	);

	CREATE TABLE IF NOT EXISTS ticket_transitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ticket_id TEXT NOT NULL,
		from_status TEXT NOT NULL,
		to_status TEXT NOT NULL,
		agent_id TEXT,
		note TEXT,
		timestamp TEXT NOT NULL,
		FOREIGN KEY (ticket_id) REFERENCES tickets(id)
	);

	CREATE TABLE IF NOT EXISTS handoffs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ticket_id TEXT NOT NULL,
		from_agent TEXT NOT NULL,
		to_agent TEXT NOT NULL,
		context_path TEXT,
		created_at TEXT NOT NULL,
		FOREIGN KEY (ticket_id) REFERENCES tickets(id)
	);

	CREATE INDEX IF NOT EXISTS idx_tickets_sprint ON tickets(sprint_id);
	CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
	CREATE INDEX IF NOT EXISTS idx_tickets_owner ON tickets(owner_agent);
	CREATE INDEX IF NOT EXISTS idx_transitions_ticket ON ticket_transitions(ticket_id);
	CREATE INDEX IF NOT EXISTS idx_handoffs_ticket ON handoffs(ticket_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) CreateSprint(sp Sprint) error {
	if sp.CreatedAt.IsZero() {
		sp.CreatedAt = time.Now()
	}
	if sp.Status == "" {
		sp.Status = SprintPlanned
	}

	_, err := s.db.Exec(
		`INSERT INTO sprints (id, name, status, owner_agent, theme, start_at, end_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sp.ID, sp.Name, sp.Status, sp.OwnerAgent, sp.Theme,
		formatTime(sp.StartAt), formatTime(sp.EndAt), formatTime(sp.CreatedAt),
	)
	return err
}

func (s *Store) ListSprints() ([]Sprint, error) {
	rows, err := s.db.Query(`SELECT id, name, status, owner_agent, theme, start_at, end_at, created_at FROM sprints ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sprints []Sprint
	for rows.Next() {
		var sp Sprint
		var startAt, endAt, createdAt string
		err := rows.Scan(&sp.ID, &sp.Name, &sp.Status, &sp.OwnerAgent, &sp.Theme, &startAt, &endAt, &createdAt)
		if err != nil {
			return nil, err
		}
		sp.StartAt = parseTime(startAt)
		sp.EndAt = parseTime(endAt)
		sp.CreatedAt = parseTime(createdAt)
		sprints = append(sprints, sp)
	}
	return sprints, rows.Err()
}

func (s *Store) GetSprint(id string) (Sprint, error) {
	var sp Sprint
	var startAt, endAt, createdAt string
	err := s.db.QueryRow(
		`SELECT id, name, status, owner_agent, theme, start_at, end_at, created_at FROM sprints WHERE id = ?`, id,
	).Scan(&sp.ID, &sp.Name, &sp.Status, &sp.OwnerAgent, &sp.Theme, &startAt, &endAt, &createdAt)
	if err != nil {
		return Sprint{}, fmt.Errorf("sprint %q not found: %w", id, err)
	}
	sp.StartAt = parseTime(startAt)
	sp.EndAt = parseTime(endAt)
	sp.CreatedAt = parseTime(createdAt)
	return sp, nil
}

func (s *Store) CreateTicket(t Ticket) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.CreatedAt
	}
	if t.Status == "" {
		t.Status = StatusBacklog
	}

	_, err := s.db.Exec(
		`INSERT INTO tickets (id, sprint_id, title, description, status, owner_agent, priority, acceptance_criteria, handoff_doc_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.SprintID, t.Title, t.Description, t.Status, t.OwnerAgent,
		t.Priority, t.AcceptanceCriteria, t.HandoffDocPath,
		formatTime(t.CreatedAt), formatTime(t.UpdatedAt),
	)
	return err
}

func (s *Store) ListTickets(sprintID string) ([]Ticket, error) {
	query := `SELECT id, sprint_id, title, description, status, owner_agent, priority, acceptance_criteria, handoff_doc_path, created_at, updated_at FROM tickets`
	var args []interface{}
	if sprintID != "" {
		query += ` WHERE sprint_id = ?`
		args = append(args, sprintID)
	}
	query += ` ORDER BY priority DESC, created_at ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		var createdAt, updatedAt string
		err := rows.Scan(&t.ID, &t.SprintID, &t.Title, &t.Description, &t.Status,
			&t.OwnerAgent, &t.Priority, &t.AcceptanceCriteria, &t.HandoffDocPath,
			&createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		t.CreatedAt = parseTime(createdAt)
		t.UpdatedAt = parseTime(updatedAt)
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (s *Store) UpdateTicket(id string, status TicketStatus, agentID string, note string) error {
	var oldStatus string
	err := s.db.QueryRow(`SELECT status FROM tickets WHERE id = ?`, id).Scan(&oldStatus)
	if err != nil {
		return fmt.Errorf("ticket %q not found: %w", id, err)
	}

	now := time.Now()
	_, err = s.db.Exec(`UPDATE tickets SET status = ?, updated_at = ? WHERE id = ?`,
		status, formatTime(now), id)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT INTO ticket_transitions (ticket_id, from_status, to_status, agent_id, note, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, oldStatus, status, agentID, note, formatTime(now),
	)
	return err
}

func (s *Store) AssignTicket(id string, agent string) error {
	res, err := s.db.Exec(`UPDATE tickets SET owner_agent = ?, updated_at = ? WHERE id = ?`,
		agent, formatTime(time.Now()), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("ticket %q not found", id)
	}
	return nil
}

func (s *Store) CreateHandoff(h Handoff) error {
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(
		`INSERT INTO handoffs (ticket_id, from_agent, to_agent, context_path, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		h.TicketID, h.FromAgent, h.ToAgent, h.ContextPath, formatTime(h.CreatedAt),
	)
	return err
}

func (s *Store) ListHandoffs(ticketID string) ([]Handoff, error) {
	rows, err := s.db.Query(
		`SELECT id, ticket_id, from_agent, to_agent, context_path, created_at FROM handoffs WHERE ticket_id = ? ORDER BY created_at DESC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var handoffs []Handoff
	for rows.Next() {
		var h Handoff
		var createdAt string
		err := rows.Scan(&h.ID, &h.TicketID, &h.FromAgent, &h.ToAgent, &h.ContextPath, &createdAt)
		if err != nil {
			return nil, err
		}
		h.CreatedAt = parseTime(createdAt)
		handoffs = append(handoffs, h)
	}
	return handoffs, rows.Err()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
