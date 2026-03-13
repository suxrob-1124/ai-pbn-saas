package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AgentSessionEvent is a single append-only delta emitted per agent iteration.
// EventType="chat_entry": PayloadJSON is a serialised chatLogEntry for the UI.
type AgentSessionEvent struct {
	ID          string
	SessionID   string
	Seq         int
	EventType   string
	PayloadJSON []byte
	CreatedAt   time.Time
}

// AgentSession records one AI agent editing session on a domain.
type AgentSession struct {
	ID                 string
	DomainID           string
	CreatedBy          string
	CreatedAt          time.Time
	FinishedAt         sql.NullTime
	Status             string // running|done|error|stopped|rolled_back
	Summary            sql.NullString
	FilesChanged       []string
	MessageCount       int
	SnapshotTag        sql.NullString
	MessagesJSON       []byte // Anthropic conversation history (for agent resume)
	ChatLogJSON        []byte // UI chat log (AgentChatMessage[] format)
	PreExistingFileIDs []string // file IDs that existed before the agent started
	DiagnosticsJSON    []byte   // agentDiagnostics JSON (PR5)
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
	SaveMessages(ctx context.Context, id string, messagesJSON, chatLogJSON []byte) error
	SavePreFileIDs(ctx context.Context, id string, fileIDs []string) error
	// AppendEvent appends a single delta event to the session's event log.
	// ON CONFLICT (session_id, seq) DO NOTHING makes it safe to retry.
	AppendEvent(ctx context.Context, sessionID string, seq int, eventType string, payload []byte) error
	// ListEvents returns all events for a session ordered by seq ascending.
	ListEvents(ctx context.Context, sessionID string) ([]AgentSessionEvent, error)
	// NextEventSeq returns MAX(seq)+1 for the session, or 0 if no events exist.
	// Using MAX(seq)+1 rather than COUNT(*) correctly handles gaps that can arise
	// when an AppendEvent call fails mid-session, ensuring resumed sessions never
	// collide with or silently overwrite already-persisted events.
	NextEventSeq(ctx context.Context, sessionID string) (int, error)
	// SaveDiagnostics persists the agentDiagnostics JSON for a session.
	SaveDiagnostics(ctx context.Context, id string, diagJSON []byte) error
	// PurgeOlderThan hard-deletes finished sessions older than retentionDays days.
	PurgeOlderThan(ctx context.Context, retentionDays int) (int64, error)
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
	var filesRaw, preFileIDsRaw []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id, domain_id, created_by, created_at,
		        finished_at, status, summary, files_changed,
		        message_count, snapshot_tag, messages_json, chat_log_json,
		        pre_file_ids, diagnostics_json
		 FROM agent_sessions WHERE id=$1`, id).
		Scan(
			&sess.ID, &sess.DomainID, &sess.CreatedBy, &sess.CreatedAt,
			&sess.FinishedAt, &sess.Status, &sess.Summary, &filesRaw,
			&sess.MessageCount, &sess.SnapshotTag, &sess.MessagesJSON, &sess.ChatLogJSON,
			&preFileIDsRaw, &sess.DiagnosticsJSON,
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
	if len(preFileIDsRaw) > 0 {
		_ = json.Unmarshal(preFileIDsRaw, &sess.PreExistingFileIDs)
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
		        message_count, snapshot_tag, messages_json, chat_log_json,
		        pre_file_ids, diagnostics_json
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
		var filesRaw, preFileIDsRaw []byte
		if err := rows.Scan(
			&sess.ID, &sess.DomainID, &sess.CreatedBy, &sess.CreatedAt,
			&sess.FinishedAt, &sess.Status, &sess.Summary, &filesRaw,
			&sess.MessageCount, &sess.SnapshotTag, &sess.MessagesJSON, &sess.ChatLogJSON,
			&preFileIDsRaw, &sess.DiagnosticsJSON,
		); err != nil {
			return nil, err
		}
		if len(filesRaw) > 0 {
			_ = json.Unmarshal(filesRaw, &sess.FilesChanged)
		}
		if len(preFileIDsRaw) > 0 {
			_ = json.Unmarshal(preFileIDsRaw, &sess.PreExistingFileIDs)
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

func (s *agentSessionSQLStore) SaveMessages(ctx context.Context, id string, messagesJSON, chatLogJSON []byte) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET messages_json=$1, chat_log_json=$2 WHERE id=$3`,
		messagesJSON, chatLogJSON, id,
	)
	if err != nil {
		return fmt.Errorf("save agent session messages: %w", err)
	}
	return nil
}

func (s *agentSessionSQLStore) SavePreFileIDs(ctx context.Context, id string, fileIDs []string) error {
	idsJSON, err := json.Marshal(fileIDs)
	if err != nil {
		return fmt.Errorf("marshal pre_file_ids: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET pre_file_ids=$1 WHERE id=$2`,
		idsJSON, id,
	)
	if err != nil {
		return fmt.Errorf("save pre_file_ids: %w", err)
	}
	return nil
}

func (s *agentSessionSQLStore) AppendEvent(ctx context.Context, sessionID string, seq int, eventType string, payload []byte) error {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_session_events(id, session_id, seq, event_type, payload_json)
		 VALUES($1, $2, $3, $4, $5)
		 ON CONFLICT (session_id, seq) DO NOTHING`,
		id, sessionID, seq, eventType, payload,
	)
	if err != nil {
		return fmt.Errorf("append agent session event: %w", err)
	}
	return nil
}

func (s *agentSessionSQLStore) NextEventSeq(ctx context.Context, sessionID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq)+1, 0) FROM agent_session_events WHERE session_id=$1`, sessionID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("next event seq: %w", err)
	}
	return n, nil
}

func (s *agentSessionSQLStore) ListEvents(ctx context.Context, sessionID string) ([]AgentSessionEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, seq, event_type, payload_json, created_at
		 FROM agent_session_events
		 WHERE session_id=$1
		 ORDER BY seq ASC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list agent session events: %w", err)
	}
	defer rows.Close()
	var result []AgentSessionEvent
	for rows.Next() {
		var ev AgentSessionEvent
		if err := rows.Scan(&ev.ID, &ev.SessionID, &ev.Seq, &ev.EventType, &ev.PayloadJSON, &ev.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, ev)
	}
	return result, rows.Err()
}

func (s *agentSessionSQLStore) SaveDiagnostics(ctx context.Context, id string, diagJSON []byte) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET diagnostics_json=$1 WHERE id=$2`,
		diagJSON, id,
	)
	if err != nil {
		return fmt.Errorf("save agent session diagnostics: %w", err)
	}
	return nil
}

func (s *agentSessionSQLStore) PurgeOlderThan(ctx context.Context, retentionDays int) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_sessions WHERE status != 'running' AND created_at < NOW() - make_interval(days => $1)`,
		retentionDays,
	)
	if err != nil {
		return 0, fmt.Errorf("purge agent sessions: %w", err)
	}
	return res.RowsAffected()
}
