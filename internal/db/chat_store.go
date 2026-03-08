package db

import (
	"fmt"
	"time"
)

// ChatSession represents a chat session stored in SQLite.
type ChatSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// ChatMessage represents a chat message stored in SQLite.
type ChatMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatStore provides chat persistence backed by SQLite.
type ChatStore struct {
	db *DB
}

// NewChatStore creates a new SQLite-backed chat store.
func NewChatStore(db *DB) *ChatStore {
	return &ChatStore{db: db}
}

func (s *ChatStore) CreateSession(sess ChatSession) error {
	_, err := s.db.db.Exec(
		"INSERT OR IGNORE INTO chat_sessions (id, user_id, title, created_at) VALUES (?, ?, ?, ?)",
		sess.ID, sess.UserID, sess.Title, sess.CreatedAt,
	)
	return err
}

func (s *ChatStore) ListSessions(userID string) ([]ChatSession, error) {
	rows, err := s.db.db.Query(
		"SELECT id, user_id, title, created_at FROM chat_sessions WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChatSession
	for rows.Next() {
		var sess ChatSession
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.Title, &sess.CreatedAt); err != nil {
			continue
		}
		result = append(result, sess)
	}
	return result, rows.Err()
}

func (s *ChatStore) DeleteSession(sessionID string) error {
	_, err := s.db.db.Exec("DELETE FROM chat_sessions WHERE id = ?", sessionID)
	return err
}

func (s *ChatStore) UpdateSessionTitle(sessionID, title string) error {
	_, err := s.db.db.Exec("UPDATE chat_sessions SET title = ? WHERE id = ?", title, sessionID)
	return err
}

func (s *ChatStore) AppendMessage(msg ChatMessage) error {
	_, err := s.db.db.Exec(
		"INSERT INTO chat_messages (id, session_id, role, content, timestamp) VALUES (?, ?, ?, ?, ?)",
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.Timestamp,
	)
	return err
}

func (s *ChatStore) LoadMessages(sessionID string) ([]ChatMessage, error) {
	rows, err := s.db.db.Query(
		"SELECT id, session_id, role, content, timestamp FROM chat_messages WHERE session_id = ? ORDER BY timestamp",
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.Timestamp); err != nil {
			continue
		}
		result = append(result, msg)
	}
	return result, rows.Err()
}

func (s *ChatStore) GetSession(sessionID string) (*ChatSession, error) {
	var sess ChatSession
	err := s.db.db.QueryRow(
		"SELECT id, user_id, title, created_at FROM chat_sessions WHERE id = ?", sessionID,
	).Scan(&sess.ID, &sess.UserID, &sess.Title, &sess.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	return &sess, nil
}
