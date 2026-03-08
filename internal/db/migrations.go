package db

const migrationV1 = `
CREATE TABLE IF NOT EXISTS executions (
    id          TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    status      TEXT NOT NULL,
    mode        TEXT NOT NULL DEFAULT 'manual',
    started_at  DATETIME NOT NULL,
    stopped_at  DATETIME,
    finished    INTEGER NOT NULL DEFAULT 0,
    data_json   TEXT,
    custom_data TEXT,
    wait_till   DATETIME
);
CREATE INDEX IF NOT EXISTS idx_executions_workflow ON executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_executions_started ON executions(started_at DESC);

CREATE TABLE IF NOT EXISTS variables (
    id    TEXT PRIMARY KEY,
    key   TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    type  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tags (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS data_tables (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    columns    TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS data_table_rows (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    table_id   TEXT NOT NULL REFERENCES data_tables(id) ON DELETE CASCADE,
    data_json  TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_data_table_rows_table ON data_table_rows(table_id);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT 'New Chat',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_user ON chat_sessions(user_id);

CREATE TABLE IF NOT EXISTS chat_messages (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    timestamp  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_session ON chat_messages(session_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id          TEXT PRIMARY KEY,
    trace_id    TEXT NOT NULL DEFAULT '',
    timestamp   DATETIME NOT NULL,
    user_id     TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL,
    session_id  TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    meta_json   TEXT
);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_type ON audit_logs(type);
CREATE INDEX IF NOT EXISTS idx_audit_trace ON audit_logs(trace_id);
CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_logs(session_id);

CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL
);
`

var migrations = []struct {
	version int
	sql     string
}{
	{1, migrationV1},
}

func (d *DB) migrate() error {
	// Ensure schema_version table exists first.
	_, err := d.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version    INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`)
	if err != nil {
		return err
	}

	var current int
	row := d.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&current); err != nil {
		return err
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := d.db.Exec(m.sql); err != nil {
			return err
		}
		if _, err := d.db.Exec(
			"INSERT INTO schema_version (version, applied_at) VALUES (?, datetime('now'))",
			m.version,
		); err != nil {
			return err
		}
	}
	return nil
}
