// Package store maintains queryable local operational state. Raw evidence remains
// in the audit store so that state records cannot silently replace source output.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct{ db *sql.DB }

type Job struct {
	ID           string          `json:"id"`
	EngagementID string          `json:"engagement_id"`
	Operation    string          `json:"operation"`
	Target       string          `json:"target,omitempty"`
	Status       string          `json:"status"`
	StartedAt    time.Time       `json:"started_at"`
	EndedAt      *time.Time      `json:"ended_at,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	EvidenceID   string          `json:"evidence_id,omitempty"`
	EvidenceSHA  string          `json:"evidence_sha256,omitempty"`
	Error        string          `json:"error,omitempty"`
}

func Open(root string) (*Store, error) {
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(stateDir, "state.db")+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open state database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure state database: %w", err)
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS jobs (
 id TEXT PRIMARY KEY,
 engagement_id TEXT NOT NULL,
 operation TEXT NOT NULL,
 target TEXT,
 status TEXT NOT NULL,
 started_at TEXT NOT NULL,
 ended_at TEXT,
 result TEXT,
 evidence_id TEXT,
 evidence_sha256 TEXT,
 error TEXT
);
CREATE INDEX IF NOT EXISTS idx_jobs_started_at ON jobs(started_at DESC);
CREATE TABLE IF NOT EXISTS tools (
 name TEXT PRIMARY KEY,
 version TEXT NOT NULL,
 executable TEXT NOT NULL,
 sha256 TEXT NOT NULL,
 risk_level TEXT NOT NULL,
 enabled INTEGER NOT NULL,
 health_status TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize state schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Start(job Job) error {
	if job.ID == "" || job.EngagementID == "" || job.Operation == "" || job.Status == "" {
		return fmt.Errorf("job id, engagement id, operation, and status are required")
	}
	if job.StartedAt.IsZero() {
		job.StartedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO jobs(id, engagement_id, operation, target, status, started_at) VALUES (?, ?, ?, ?, ?, ?)`, job.ID, job.EngagementID, job.Operation, job.Target, job.Status, job.StartedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) Finish(id, status, evidenceID, evidenceSHA, errorText string, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal job result: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(`UPDATE jobs SET status=?, ended_at=?, result=?, evidence_id=?, evidence_sha256=?, error=? WHERE id=?`, status, now, string(encoded), evidenceID, evidenceSHA, errorText, id)
	return err
}

func (s *Store) RecentJobs(limit int) ([]Job, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, engagement_id, operation, target, status, started_at, ended_at, result, evidence_id, evidence_sha256, error FROM jobs ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var job Job
		var started string
		var ended sql.NullString
		var result sql.NullString
		var evidenceID, evidenceSHA, errText sql.NullString
		if err := rows.Scan(&job.ID, &job.EngagementID, &job.Operation, &job.Target, &job.Status, &started, &ended, &result, &evidenceID, &evidenceSHA, &errText); err != nil {
			return nil, err
		}
		job.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		if ended.Valid {
			parsed, parseErr := time.Parse(time.RFC3339Nano, ended.String)
			if parseErr == nil {
				job.EndedAt = &parsed
			}
		}
		if result.Valid {
			job.Result = json.RawMessage(result.String)
		}
		job.EvidenceID, job.EvidenceSHA, job.Error = evidenceID.String, evidenceSHA.String, errText.String
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) IntegrityCheck() (string, error) {
	var result string
	err := s.db.QueryRow(`PRAGMA integrity_check;`).Scan(&result)
	return result, err
}
