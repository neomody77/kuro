package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/neomody77/kuro/internal/pipeline"
)

// DataTableStore implements data table storage backed by SQLite.
type DataTableStore struct {
	db *DB
}

// NewDataTableStore creates a new SQLite-backed data table store.
func NewDataTableStore(db *DB) *DataTableStore {
	return &DataTableStore{db: db}
}

func (s *DataTableStore) ListTables() []pipeline.DataTable {
	rows, err := s.db.db.Query("SELECT id, name, columns, created_at, updated_at FROM data_tables ORDER BY name")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []pipeline.DataTable
	for rows.Next() {
		var t pipeline.DataTable
		var colJSON string
		if err := rows.Scan(&t.ID, &t.Name, &colJSON, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(colJSON), &t.Columns)
		result = append(result, t)
	}
	return result
}

func (s *DataTableStore) GetTable(id string) (*pipeline.DataTable, error) {
	var t pipeline.DataTable
	var colJSON string
	err := s.db.db.QueryRow(
		"SELECT id, name, columns, created_at, updated_at FROM data_tables WHERE id = ?", id,
	).Scan(&t.ID, &t.Name, &colJSON, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("table %q not found", id)
	}
	json.Unmarshal([]byte(colJSON), &t.Columns)
	return &t, nil
}

func (s *DataTableStore) CreateTable(t pipeline.DataTable) (*pipeline.DataTable, error) {
	now := time.Now()
	if t.ID == "" {
		t.ID = fmt.Sprintf("dt_%d", now.UnixMilli())
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	colJSON, _ := json.Marshal(t.Columns)
	_, err := s.db.db.Exec(
		"INSERT INTO data_tables (id, name, columns, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		t.ID, t.Name, string(colJSON), t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *DataTableStore) UpdateTable(id string, t pipeline.DataTable) (*pipeline.DataTable, error) {
	t.UpdatedAt = time.Now()
	colJSON, _ := json.Marshal(t.Columns)
	res, err := s.db.db.Exec(
		"UPDATE data_tables SET name = ?, columns = ?, updated_at = ? WHERE id = ?",
		t.Name, string(colJSON), t.UpdatedAt, id,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("table %q not found", id)
	}
	t.ID = id
	return &t, nil
}

func (s *DataTableStore) DeleteTable(id string) error {
	res, err := s.db.db.Exec("DELETE FROM data_tables WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("table %q not found", id)
	}
	return nil
}

func (s *DataTableStore) ListRows(tableID string) []pipeline.DataTableRow {
	rows, err := s.db.db.Query(
		"SELECT id, data_json, created_at, updated_at FROM data_table_rows WHERE table_id = ? ORDER BY id",
		tableID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []pipeline.DataTableRow
	for rows.Next() {
		var r pipeline.DataTableRow
		var dataJSON string
		if err := rows.Scan(&r.ID, &dataJSON, &r.CreatedAt, &r.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(dataJSON), &r.Data)
		result = append(result, r)
	}
	return result
}

func (s *DataTableStore) InsertRows(tableID string, newRows []pipeline.DataTableRow) ([]pipeline.DataTableRow, error) {
	now := time.Now()
	var inserted []pipeline.DataTableRow
	for _, r := range newRows {
		r.CreatedAt = now
		r.UpdatedAt = now
		dataJSON, _ := json.Marshal(r.Data)
		res, err := s.db.db.Exec(
			"INSERT INTO data_table_rows (table_id, data_json, created_at, updated_at) VALUES (?, ?, ?, ?)",
			tableID, string(dataJSON), r.CreatedAt, r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		r.ID = int(id)
		inserted = append(inserted, r)
	}
	return inserted, nil
}

func (s *DataTableStore) UpdateRow(tableID string, rowID int, data map[string]any) (*pipeline.DataTableRow, error) {
	now := time.Now()
	dataJSON, _ := json.Marshal(data)
	res, err := s.db.db.Exec(
		"UPDATE data_table_rows SET data_json = ?, updated_at = ? WHERE id = ? AND table_id = ?",
		string(dataJSON), now, rowID, tableID,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("row %d not found in table %q", rowID, tableID)
	}
	return &pipeline.DataTableRow{ID: rowID, Data: data, UpdatedAt: now}, nil
}

func (s *DataTableStore) DeleteRow(tableID string, rowID int) error {
	res, err := s.db.db.Exec(
		"DELETE FROM data_table_rows WHERE id = ? AND table_id = ?", rowID, tableID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("row %d not found in table %q", rowID, tableID)
	}
	return nil
}
