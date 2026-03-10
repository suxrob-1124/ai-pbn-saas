package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AgentSession records one AI agent editing session on a domain.
type AgentSession struct {
	ID           string
	DomainID     string
	CreatedBy    string
	CreatedAt    time.Time
	FinishedAt   sql.NullTime
	Status       string // running|done|error|stopped|rolled_back
	Summary      sql.NullString
	FilesChanged []string
	MessageCount int
	SnapshotTag  sql.NullString
}

// AgentSessionStore defines persistence operations for agent sessions.
type AgentSessionStore interface {
	Create(ctx context.Context, sess AgentSession) error
	Get(ctx context.Context, id string) (AgentSession, error)
	ListByDomain(ctx context.Context, domainID string, limit int) ([]AgentSession, error)
	Finish(ctx context.Context, id, status, summary string, filesChanged []string, messageCount int) error
	SetSnapshotTag(ctx context.Context, id, tag string) error
	// MarkStaleRunning marks sessions stuck in "running" state older than the
	// given threshold as "stopped". Returns the number of sessions updated.
	MarkStaleRunning(ctx context.Context, olderThan time.Duration) (int64, error)
}

type agentSessionSQLStore struct {
	db *sql.DB
}

// NewAgentSessionStore returns a new SQL-backed AgentSessionStore.
func NewAgentSessionStore(db *sql.DB) AgentSessionStore {
	return &agentSessionSQLStore{db: db}
}

func (s *agentSessionSQLStore) Create(ctx context.Context, sess AgentSession) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_sessions(id, domain_id, created_by, status)
		 VALUES($1, $2, $3, $4)`,
		sess.ID, sess.DomainID, sess.CreatedBy, sess.Status,
	)
	if err != nil {
		return fmt.Errorf("create agent session: %w", err)
	}
	return nil
}

func (s *agentSessionSQLStore) Get(ctx context.Context, id string) (AgentSession, error) {
	var sess AgentSession
	var filesRaw []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id, domain_id, created_by, created_at,
		        finished_at, status, summary, files_changed,
		        message_count, snapshot_tag
		 FROM agent_sessions WHERE id=$1`, id).
		Scan(
			&sess.ID, &sess.DomainID, &sess.CreatedBy, &sess.CreatedAt,
			&sess.FinishedAt, &sess.Status, &sess.Summary, &filesRaw,
			&sess.MessageCount, &sess.SnapshotTag,
		)
	if err != nil {
		if err == sql.ErrNoRows {
			return AgentSession{}, fmt.Errorf("agent session not found")
		}
		return AgentSession{}, fmt.Errorf("get agent session: %w", err)
	}
	if len(filesRaw) > 0 {
		_ = json.Unmarshal(filesRaw, &sess.FilesChanged)
	}
	return sess, nil
}

func (s *agentSessionSQLStore) ListByDomain(ctx context.Context, domainID string, limit int) ([]AgentSession, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, domain_id, created_by, created_at,
		        finished_at, status, summary, files_changed,
		        message_count, snapshot_tag
		 FROM agent_sessions
		 WHERE domain_id=$1
		 ORDER BY created_at DESC
		 LIMIT $2`, domainID, limit)
	if err != nil {
		return nil, fmt.Errorf("list agent sessions: %w", err)
	}
	defer rows.Close()
	var result []AgentSession
	for rows.Next() {
		var sess AgentSession
		var filesRaw []byte
		if err := rows.Scan(
			&sess.ID, &sess.DomainID, &sess.CreatedBy, &sess.CreatedAt,
			&sess.FinishedAt, &sess.Status, &sess.Summary, &filesRaw,
			&sess.MessageCount, &sess.SnapshotTag,
		); err != nil {
			return nil, err
		}
		if len(filesRaw) > 0 {
			_ = json.Unmarshal(filesRaw, &sess.FilesChanged)
		}
		result = append(result, sess)
	}
	return result, rows.Err()
}

func (s *agentSessionSQLStore) Finish(ctx context.Context, id, status, summary string, filesChanged []string, messageCount int) error {
	filesJSON, err := json.Marshal(filesChanged)
	if err != nil {
		return fmt.Errorf("marshal files_changed: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE agent_sessions
		 SET status=$1, summary=$2, files_changed=$3, message_count=$4, finished_at=NOW()
		 WHERE id=$5`,
		status, summary, filesJSON, messageCount, id,
	)
	if err != nil {
		return fmt.Errorf("finish agent session: %w", err)
	}
	return nil
}

func (s *agentSessionSQLStore) SetSnapshotTag(ctx context.Context, id, tag string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET snapshot_tag=$1 WHERE id=$2`,
		tag, id,
	)
	if err != nil {
		return fmt.Errorf("set snapshot tag: %w", err)
	}
	return nil
}

func (s *agentSessionSQLStore) MarkStaleRunning(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions
		 SET status='stopped', finished_at=NOW(), summary='Terminated: stale running session'
		 WHERE status='running' AND created_at < NOW() - $1::interval`,
		fmt.Sprintf("%d seconds", int(olderThan.Seconds())),
	)
	if err != nil {
		return 0, fmt.Errorf("mark stale running sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
