package db

import (
	"fmt"
	"time"

	"github.com/neomody77/kuro/internal/pipeline"
)

// TagStore implements tag storage backed by SQLite.
type TagStore struct {
	db *DB
}

// NewTagStore creates a new SQLite-backed tag store.
func NewTagStore(db *DB) *TagStore {
	return &TagStore{db: db}
}

func (s *TagStore) List() []pipeline.Tag {
	rows, err := s.db.db.Query("SELECT id, name, created_at, updated_at FROM tags ORDER BY name")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []pipeline.Tag
	for rows.Next() {
		var t pipeline.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		result = append(result, t)
	}
	return result
}

func (s *TagStore) Get(id string) (*pipeline.Tag, error) {
	var t pipeline.Tag
	err := s.db.db.QueryRow("SELECT id, name, created_at, updated_at FROM tags WHERE id = ?", id).
		Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("tag %q not found", id)
	}
	return &t, nil
}

func (s *TagStore) Create(t pipeline.Tag) (*pipeline.Tag, error) {
	now := time.Now()
	if t.ID == "" {
		t.ID = fmt.Sprintf("tag_%d", now.UnixMilli())
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err := s.db.db.Exec(
		"INSERT INTO tags (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		t.ID, t.Name, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *TagStore) Update(id string, t pipeline.Tag) (*pipeline.Tag, error) {
	t.UpdatedAt = time.Now()
	res, err := s.db.db.Exec(
		"UPDATE tags SET name = ?, updated_at = ? WHERE id = ?",
		t.Name, t.UpdatedAt, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("tag %q not found", id)
	}
	t.ID = id
	// Preserve original created_at
	existing, _ := s.Get(id)
	if existing != nil {
		t.CreatedAt = existing.CreatedAt
	}
	return &t, nil
}

func (s *TagStore) Delete(id string) error {
	res, err := s.db.db.Exec("DELETE FROM tags WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tag %q not found", id)
	}
	return nil
}
