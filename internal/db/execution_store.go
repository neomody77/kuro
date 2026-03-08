package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/neomody77/kuro/internal/pipeline"
)

// ExecutionStore implements pipeline.ExecutionStore backed by SQLite.
type ExecutionStore struct {
	db *DB
}

// NewExecutionStore creates a new SQLite-backed execution store.
func NewExecutionStore(db *DB) *ExecutionStore {
	return &ExecutionStore{db: db}
}

func (s *ExecutionStore) SaveExecution(exec *pipeline.Execution) error {
	dataJSON, _ := json.Marshal(exec.Data)
	customJSON, _ := json.Marshal(exec.CustomData)

	var stoppedAt *time.Time
	if exec.StoppedAt != nil {
		stoppedAt = exec.StoppedAt
	}

	_, err := s.db.db.Exec(`
		INSERT OR REPLACE INTO executions
			(id, workflow_id, status, mode, started_at, stopped_at, finished, data_json, custom_data, wait_till)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.ID, exec.WorkflowID, exec.Status, exec.Mode,
		exec.StartedAt, stoppedAt, exec.Finished,
		string(dataJSON), string(customJSON), exec.WaitTill,
	)
	return err
}

func (s *ExecutionStore) GetExecution(id string) (*pipeline.Execution, error) {
	row := s.db.db.QueryRow(`
		SELECT id, workflow_id, status, mode, started_at, stopped_at, finished, data_json, custom_data, wait_till
		FROM executions WHERE id = ?`, id)
	return scanExecution(row)
}

func (s *ExecutionStore) ListExecutions(workflowID string, limit int) ([]*pipeline.Execution, error) {
	var rows *sql.Rows
	var err error

	if workflowID != "" {
		rows, err = s.db.db.Query(`
			SELECT id, workflow_id, status, mode, started_at, stopped_at, finished, data_json, custom_data, wait_till
			FROM executions WHERE workflow_id = ? ORDER BY started_at DESC LIMIT ?`,
			workflowID, limit)
	} else {
		rows, err = s.db.db.Query(`
			SELECT id, workflow_id, status, mode, started_at, stopped_at, finished, data_json, custom_data, wait_till
			FROM executions ORDER BY started_at DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*pipeline.Execution
	for rows.Next() {
		exec, err := scanExecutionRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, exec)
	}
	return result, rows.Err()
}

func (s *ExecutionStore) DeleteExecution(id string) error {
	_, err := s.db.db.Exec("DELETE FROM executions WHERE id = ?", id)
	return err
}

// ClearExecutions deletes all executions for a workflow (or all if workflowID is empty).
func (s *ExecutionStore) ClearExecutions(workflowID string) (int, error) {
	var res sql.Result
	var err error
	if workflowID != "" {
		res, err = s.db.db.Exec("DELETE FROM executions WHERE workflow_id = ?", workflowID)
	} else {
		res, err = s.db.db.Exec("DELETE FROM executions")
	}
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func scanExecution(row *sql.Row) (*pipeline.Execution, error) {
	var exec pipeline.Execution
	var stoppedAt sql.NullTime
	var waitTill sql.NullTime
	var dataJSON, customJSON sql.NullString

	err := row.Scan(
		&exec.ID, &exec.WorkflowID, &exec.Status, &exec.Mode,
		&exec.StartedAt, &stoppedAt, &exec.Finished,
		&dataJSON, &customJSON, &waitTill,
	)
	if err != nil {
		return nil, fmt.Errorf("execution not found: %w", err)
	}

	if stoppedAt.Valid {
		exec.StoppedAt = &stoppedAt.Time
	}
	if waitTill.Valid {
		exec.WaitTill = &waitTill.Time
	}
	if dataJSON.Valid && dataJSON.String != "" && dataJSON.String != "null" {
		json.Unmarshal([]byte(dataJSON.String), &exec.Data)
	}
	if customJSON.Valid && customJSON.String != "" && customJSON.String != "null" {
		json.Unmarshal([]byte(customJSON.String), &exec.CustomData)
	}
	return &exec, nil
}

func scanExecutionRows(rows *sql.Rows) (*pipeline.Execution, error) {
	var exec pipeline.Execution
	var stoppedAt sql.NullTime
	var waitTill sql.NullTime
	var dataJSON, customJSON sql.NullString

	err := rows.Scan(
		&exec.ID, &exec.WorkflowID, &exec.Status, &exec.Mode,
		&exec.StartedAt, &stoppedAt, &exec.Finished,
		&dataJSON, &customJSON, &waitTill,
	)
	if err != nil {
		return nil, err
	}

	if stoppedAt.Valid {
		exec.StoppedAt = &stoppedAt.Time
	}
	if waitTill.Valid {
		exec.WaitTill = &waitTill.Time
	}
	if dataJSON.Valid && dataJSON.String != "" && dataJSON.String != "null" {
		json.Unmarshal([]byte(dataJSON.String), &exec.Data)
	}
	if customJSON.Valid && customJSON.String != "" && customJSON.String != "null" {
		json.Unmarshal([]byte(customJSON.String), &exec.CustomData)
	}
	return &exec, nil
}
