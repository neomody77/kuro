package db

import (
	"fmt"
	"time"

	"github.com/neomody77/kuro/internal/pipeline"
)

// VariableStore implements variable storage backed by SQLite.
type VariableStore struct {
	db *DB
}

// NewVariableStore creates a new SQLite-backed variable store.
func NewVariableStore(db *DB) *VariableStore {
	return &VariableStore{db: db}
}

func (s *VariableStore) List() []pipeline.Variable {
	rows, err := s.db.db.Query("SELECT id, key, value, type FROM variables ORDER BY key")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []pipeline.Variable
	for rows.Next() {
		var v pipeline.Variable
		if err := rows.Scan(&v.ID, &v.Key, &v.Value, &v.Type); err != nil {
			continue
		}
		result = append(result, v)
	}
	return result
}

func (s *VariableStore) Get(id string) (*pipeline.Variable, error) {
	var v pipeline.Variable
	err := s.db.db.QueryRow("SELECT id, key, value, type FROM variables WHERE id = ?", id).
		Scan(&v.ID, &v.Key, &v.Value, &v.Type)
	if err != nil {
		return nil, fmt.Errorf("variable %q not found", id)
	}
	return &v, nil
}

func (s *VariableStore) Create(v pipeline.Variable) (*pipeline.Variable, error) {
	if v.ID == "" {
		v.ID = fmt.Sprintf("var_%d", time.Now().UnixMilli())
	}
	_, err := s.db.db.Exec(
		"INSERT INTO variables (id, key, value, type) VALUES (?, ?, ?, ?)",
		v.ID, v.Key, v.Value, v.Type,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *VariableStore) Update(id string, v pipeline.Variable) (*pipeline.Variable, error) {
	res, err := s.db.db.Exec(
		"UPDATE variables SET key = ?, value = ?, type = ? WHERE id = ?",
		v.Key, v.Value, v.Type, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("variable %q not found", id)
	}
	v.ID = id
	return &v, nil
}

func (s *VariableStore) Delete(id string) error {
	res, err := s.db.db.Exec("DELETE FROM variables WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("variable %q not found", id)
	}
	return nil
}
