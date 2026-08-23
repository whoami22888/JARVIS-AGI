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

// ConversationMessage is a local, timestamped chat record. Content is stored
// only under the configured local data root.
type ConversationMessage struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Memory is a concise, attributable local fact selected for chat context.
type Memory struct {
	ID         int64      `json:"id"`
	Category   string     `json:"category"`
	MemoryKey  string     `json:"memory_key"`
	Value      string     `json:"value"`
	Source     string     `json:"source"`
	Confidence float64    `json:"confidence"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// Activity is a visible timeline item for the local operator.
type Activity struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Summary   string          `json:"summary"`
	Detail    json.RawMessage `json:"detail,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// AgentTask is an approval-gated local orchestration record.
type AgentTask struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	CommandID   string          `json:"command_id,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	Status      string          `json:"status"`
	RequestedAt time.Time       `json:"requested_at"`
	ApprovedAt  *time.Time      `json:"approved_at,omitempty"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// Skill is a local draft or approved knowledge/tool manifest. Skills remain
// inert data records; approving a skill never executes supplied content.
type Skill struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
CREATE TABLE IF NOT EXISTS conversation_messages (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 role TEXT NOT NULL,
 content TEXT NOT NULL,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_conversation_created_at ON conversation_messages(created_at DESC);
CREATE TABLE IF NOT EXISTS memories (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 category TEXT NOT NULL,
 memory_key TEXT NOT NULL,
 value TEXT NOT NULL,
 source TEXT NOT NULL,
 confidence REAL NOT NULL,
 created_at TEXT NOT NULL,
 expires_at TEXT,
 UNIQUE(category, memory_key)
);
CREATE TABLE IF NOT EXISTS activities (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL,
 summary TEXT NOT NULL,
 detail TEXT,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activities_created_at ON activities(created_at DESC);
CREATE TABLE IF NOT EXISTS agent_tasks (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL,
 title TEXT NOT NULL,
 command_id TEXT,
 input TEXT,
 status TEXT NOT NULL,
 requested_at TEXT NOT NULL,
 approved_at TEXT,
 started_at TEXT,
 completed_at TEXT,
 result TEXT,
 error TEXT
);
CREATE INDEX IF NOT EXISTS idx_agent_tasks_requested_at ON agent_tasks(requested_at DESC);
CREATE TABLE IF NOT EXISTS skills (
 id TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 description TEXT NOT NULL,
 body TEXT NOT NULL,
 status TEXT NOT NULL,
 source TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_skills_updated_at ON skills(updated_at DESC);
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
			if parsed, parseErr := time.Parse(time.RFC3339Nano, ended.String); parseErr == nil {
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

func (s *Store) AddMessage(role, content string) (ConversationMessage, error) {
	if role == "" || content == "" {
		return ConversationMessage{}, fmt.Errorf("message role and content are required")
	}
	now := time.Now().UTC()
	result, err := s.db.Exec(`INSERT INTO conversation_messages(role, content, created_at) VALUES (?, ?, ?)`, role, content, now.Format(time.RFC3339Nano))
	if err != nil {
		return ConversationMessage{}, err
	}
	id, _ := result.LastInsertId()
	return ConversationMessage{ID: id, Role: role, Content: content, CreatedAt: now}, nil
}

func (s *Store) RecentMessages(limit int) ([]ConversationMessage, error) {
	if limit < 1 || limit > 200 {
		limit = 40
	}
	rows, err := s.db.Query(`SELECT id, role, content, created_at FROM (SELECT id, role, content, created_at FROM conversation_messages ORDER BY id DESC LIMIT ?) ORDER BY id ASC`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []ConversationMessage
	for rows.Next() {
		var item ConversationMessage
		var created string
		if err := rows.Scan(&item.ID, &item.Role, &item.Content, &created); err != nil {
			return nil, err
		}
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		messages = append(messages, item)
	}
	return messages, rows.Err()
}

func (s *Store) UpsertMemory(memory Memory) (Memory, error) {
	if memory.Category == "" || memory.MemoryKey == "" || memory.Value == "" || memory.Source == "" {
		return Memory{}, fmt.Errorf("memory category, key, value, and source are required")
	}
	if memory.Confidence < 0 || memory.Confidence > 1 {
		return Memory{}, fmt.Errorf("memory confidence must be between 0 and 1")
	}
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = time.Now().UTC()
	}
	var expiry any
	if memory.ExpiresAt != nil {
		expiry = memory.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.Exec(`INSERT INTO memories(category, memory_key, value, source, confidence, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(category, memory_key) DO UPDATE SET value=excluded.value, source=excluded.source, confidence=excluded.confidence, created_at=excluded.created_at, expires_at=excluded.expires_at`, memory.Category, memory.MemoryKey, memory.Value, memory.Source, memory.Confidence, memory.CreatedAt.UTC().Format(time.RFC3339Nano), expiry)
	return memory, err
}

func (s *Store) RecentMemories(limit int) ([]Memory, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, category, memory_key, value, source, confidence, created_at, expires_at FROM memories WHERE expires_at IS NULL OR expires_at > ? ORDER BY created_at DESC LIMIT ?`, time.Now().UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var memories []Memory
	for rows.Next() {
		var item Memory
		var created string
		var expires sql.NullString
		if err := rows.Scan(&item.ID, &item.Category, &item.MemoryKey, &item.Value, &item.Source, &item.Confidence, &created, &expires); err != nil {
			return nil, err
		}
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		if expires.Valid {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, expires.String); parseErr == nil {
				item.ExpiresAt = &parsed
			}
		}
		memories = append(memories, item)
	}
	return memories, rows.Err()
}

func (s *Store) RecordActivity(activity Activity) (Activity, error) {
	if activity.ID == "" || activity.Kind == "" || activity.Summary == "" {
		return Activity{}, fmt.Errorf("activity id, kind, and summary are required")
	}
	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now().UTC()
	}
	if activity.Detail == nil {
		activity.Detail = json.RawMessage(`{}`)
	}
	_, err := s.db.Exec(`INSERT INTO activities(id, kind, summary, detail, created_at) VALUES (?, ?, ?, ?, ?)`, activity.ID, activity.Kind, activity.Summary, string(activity.Detail), activity.CreatedAt.UTC().Format(time.RFC3339Nano))
	return activity, err
}

func (s *Store) RecentActivities(limit int) ([]Activity, error) {
	if limit < 1 || limit > 200 {
		limit = 60
	}
	rows, err := s.db.Query(`SELECT id, kind, summary, detail, created_at FROM activities ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var activities []Activity
	for rows.Next() {
		var item Activity
		var detail sql.NullString
		var created string
		if err := rows.Scan(&item.ID, &item.Kind, &item.Summary, &detail, &created); err != nil {
			return nil, err
		}
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		if detail.Valid {
			item.Detail = json.RawMessage(detail.String)
		}
		activities = append(activities, item)
	}
	return activities, rows.Err()
}

func (s *Store) CreateTask(task AgentTask) (AgentTask, error) {
	if task.ID == "" || task.Kind == "" || task.Title == "" || task.Status == "" {
		return AgentTask{}, fmt.Errorf("task id, kind, title, and status are required")
	}
	if task.RequestedAt.IsZero() {
		task.RequestedAt = time.Now().UTC()
	}
	if task.Input == nil {
		task.Input = json.RawMessage(`{}`)
	}
	_, err := s.db.Exec(`INSERT INTO agent_tasks(id, kind, title, command_id, input, status, requested_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, task.ID, task.Kind, task.Title, task.CommandID, string(task.Input), task.Status, task.RequestedAt.UTC().Format(time.RFC3339Nano))
	return task, err
}

func (s *Store) TaskByID(id string) (AgentTask, error) {
	var task AgentTask
	var input, result, taskError sql.NullString
	var approved, started, completed sql.NullString
	var requested string
	err := s.db.QueryRow(`SELECT id, kind, title, command_id, input, status, requested_at, approved_at, started_at, completed_at, result, error FROM agent_tasks WHERE id=?`, id).Scan(&task.ID, &task.Kind, &task.Title, &task.CommandID, &input, &task.Status, &requested, &approved, &started, &completed, &result, &taskError)
	if err != nil {
		return AgentTask{}, err
	}
	task.RequestedAt, _ = time.Parse(time.RFC3339Nano, requested)
	if input.Valid {
		task.Input = json.RawMessage(input.String)
	}
	if result.Valid {
		task.Result = json.RawMessage(result.String)
	}
	task.Error = taskError.String
	for _, pair := range []struct {
		source      sql.NullString
		destination **time.Time
	}{{approved, &task.ApprovedAt}, {started, &task.StartedAt}, {completed, &task.CompletedAt}} {
		if pair.source.Valid {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, pair.source.String); parseErr == nil {
				*pair.destination = &parsed
			}
		}
	}
	return task, nil
}

func (s *Store) RecentTasks(limit int) ([]AgentTask, error) {
	if limit < 1 || limit > 200 {
		limit = 60
	}
	rows, err := s.db.Query(`SELECT id, kind, title, command_id, input, status, requested_at, approved_at, started_at, completed_at, result, error FROM agent_tasks ORDER BY requested_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []AgentTask
	for rows.Next() {
		var task AgentTask
		var input, result, taskError sql.NullString
		var approved, started, completed sql.NullString
		var requested string
		if err := rows.Scan(&task.ID, &task.Kind, &task.Title, &task.CommandID, &input, &task.Status, &requested, &approved, &started, &completed, &result, &taskError); err != nil {
			return nil, err
		}
		task.RequestedAt, _ = time.Parse(time.RFC3339Nano, requested)
		if input.Valid {
			task.Input = json.RawMessage(input.String)
		}
		if result.Valid {
			task.Result = json.RawMessage(result.String)
		}
		task.Error = taskError.String
		for _, pair := range []struct {
			source      sql.NullString
			destination **time.Time
		}{{approved, &task.ApprovedAt}, {started, &task.StartedAt}, {completed, &task.CompletedAt}} {
			if pair.source.Valid {
				if parsed, parseErr := time.Parse(time.RFC3339Nano, pair.source.String); parseErr == nil {
					*pair.destination = &parsed
				}
			}
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) SetTaskStatus(id, status, errorText string, result any) (AgentTask, error) {
	if id == "" || status == "" {
		return AgentTask{}, fmt.Errorf("task id and status are required")
	}
	var resultJSON any
	if result != nil {
		encoded, err := json.Marshal(result)
		if err != nil {
			return AgentTask{}, fmt.Errorf("marshal task result: %w", err)
		}
		resultJSON = string(encoded)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`UPDATE agent_tasks SET status=?, approved_at=CASE WHEN ?='APPROVED' THEN ? ELSE approved_at END, started_at=CASE WHEN ?='RUNNING' THEN ? ELSE started_at END, completed_at=CASE WHEN ? IN ('COMPLETED','FAILED','CANCELLED','TAKEOVER') THEN ? ELSE completed_at END, result=COALESCE(?, result), error=? WHERE id=?`, status, status, now, status, now, status, now, resultJSON, errorText, id)
	if err != nil {
		return AgentTask{}, err
	}
	return s.TaskByID(id)
}

func (s *Store) CreateSkill(skill Skill) (Skill, error) {
	if skill.ID == "" || skill.Name == "" || skill.Description == "" || skill.Body == "" || skill.Status == "" || skill.Source == "" {
		return Skill{}, fmt.Errorf("skill id, name, description, body, status, and source are required")
	}
	now := time.Now().UTC()
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = now
	}
	if skill.UpdatedAt.IsZero() {
		skill.UpdatedAt = now
	}
	_, err := s.db.Exec(`INSERT INTO skills(id, name, description, body, status, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, skill.ID, skill.Name, skill.Description, skill.Body, skill.Status, skill.Source, skill.CreatedAt.UTC().Format(time.RFC3339Nano), skill.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return skill, err
}

func (s *Store) SetSkillStatus(id, status string) (Skill, error) {
	if id == "" || status == "" {
		return Skill{}, fmt.Errorf("skill id and status are required")
	}
	_, err := s.db.Exec(`UPDATE skills SET status=?, updated_at=? WHERE id=?`, status, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return Skill{}, err
	}
	return s.SkillByID(id)
}

func (s *Store) SkillByID(id string) (Skill, error) {
	var item Skill
	var created, updated string
	err := s.db.QueryRow(`SELECT id, name, description, body, status, source, created_at, updated_at FROM skills WHERE id=?`, id).Scan(&item.ID, &item.Name, &item.Description, &item.Body, &item.Status, &item.Source, &created, &updated)
	if err != nil {
		return Skill{}, err
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return item, nil
}

func (s *Store) RecentSkills(limit int) ([]Skill, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(`SELECT id, name, description, body, status, source, created_at, updated_at FROM skills ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var skills []Skill
	for rows.Next() {
		var item Skill
		var created, updated string
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Body, &item.Status, &item.Source, &created, &updated); err != nil {
			return nil, err
		}
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		skills = append(skills, item)
	}
	return skills, rows.Err()
}

func (s *Store) IntegrityCheck() (string, error) {
	var result string
	err := s.db.QueryRow(`PRAGMA integrity_check;`).Scan(&result)
	return result, err
}
