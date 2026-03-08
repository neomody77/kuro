package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/neomody77/kuro/internal/audit"
)

// AuditStore implements audit.Store backed by SQLite.
type AuditStore struct {
	db *DB
}

// NewAuditStore creates a new SQLite-backed audit store.
func NewAuditStore(db *DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) Log(entry audit.Entry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit_%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	var metaJSON sql.NullString
	if entry.Meta != nil {
		data, _ := json.Marshal(entry.Meta)
		metaJSON = sql.NullString{String: string(data), Valid: true}
	}

	_, err := s.db.db.Exec(`
		INSERT INTO audit_logs (id, trace_id, timestamp, user_id, type, session_id, action, detail, duration_ms, meta_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.TraceID, entry.Timestamp, entry.UserID,
		entry.Type, entry.SessionID, entry.Action, entry.Detail,
		entry.DurationMs, metaJSON,
	)
	return err
}

func (s *AuditStore) Query(filter audit.QueryFilter) ([]audit.Entry, int, error) {
	var where []string
	var args []any

	if filter.Type != "" {
		where = append(where, "type = ?")
		args = append(args, filter.Type)
	}
	if filter.TraceID != "" {
		where = append(where, "trace_id = ?")
		args = append(args, filter.TraceID)
	}
	if filter.SessionID != "" {
		where = append(where, "session_id = ?")
		args = append(args, filter.SessionID)
	}
	if filter.UserID != "" {
		where = append(where, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if !filter.Since.IsZero() {
		where = append(where, "timestamp >= ?")
		args = append(args, filter.Since)
	}
	if !filter.Until.IsZero() {
		where = append(where, "timestamp <= ?")
		args = append(args, filter.Until)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs " + whereClause
	if err := s.db.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch results
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset

	query := fmt.Sprintf(
		"SELECT id, trace_id, timestamp, user_id, type, session_id, action, detail, duration_ms, meta_json FROM audit_logs %s ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		whereClause,
	)
	queryArgs := append(args, limit, offset)

	rows, err := s.db.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []audit.Entry
	for rows.Next() {
		var e audit.Entry
		var metaJSON sql.NullString
		if err := rows.Scan(
			&e.ID, &e.TraceID, &e.Timestamp, &e.UserID,
			&e.Type, &e.SessionID, &e.Action, &e.Detail,
			&e.DurationMs, &metaJSON,
		); err != nil {
			continue
		}
		if metaJSON.Valid && metaJSON.String != "" {
			json.Unmarshal([]byte(metaJSON.String), &e.Meta)
		}
		result = append(result, e)
	}
	return result, total, rows.Err()
}
