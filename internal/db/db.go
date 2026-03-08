// Package db manages SQLite connections, queries, and schema migrations.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB with convenience methods.
type DB struct {
	db *sql.DB
}

// Open opens a SQLite database at the given path and runs migrations.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db: create dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: migrate: %w", err)
	}
	return d, nil
}

// OpenMemory opens an in-memory SQLite database (for testing).
func OpenMemory() (*DB, error) {
	sqlDB, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// SQL returns the underlying *sql.DB.
func (d *DB) SQL() *sql.DB {
	return d.db
}

// UserDBCache lazily opens and caches per-user DB connections.
type UserDBCache struct {
	mu      sync.Mutex
	dataDir string
	dbs     map[string]*DB
}

// NewUserDBCache creates a cache for per-user databases.
func NewUserDBCache(dataDir string) *UserDBCache {
	return &UserDBCache{
		dataDir: dataDir,
		dbs:     make(map[string]*DB),
	}
}

// Get returns the DB for the given username, opening it if needed.
func (c *UserDBCache) Get(username string) (*DB, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if d, ok := c.dbs[username]; ok {
		return d, nil
	}

	path := filepath.Join(c.dataDir, "users", username, "data", "kuro.db")
	d, err := Open(path)
	if err != nil {
		return nil, err
	}
	c.dbs[username] = d
	return d, nil
}

// Close closes all cached database connections.
func (c *UserDBCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range c.dbs {
		d.Close()
	}
	c.dbs = make(map[string]*DB)
}
