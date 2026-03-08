package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// WorkflowStore loads and saves workflow definitions from disk.
type WorkflowStore struct {
	dir string
}

// NewWorkflowStore creates a store rooted at the given directory.
func NewWorkflowStore(dir string) *WorkflowStore {
	return &WorkflowStore{dir: dir}
}

// List returns all workflow definitions in the store.
func (s *WorkflowStore) List() ([]*Workflow, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	var workflows []*Workflow
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var w Workflow
		if err := json.Unmarshal(data, &w); err != nil {
			continue
		}
		workflows = append(workflows, &w)
	}
	return workflows, nil
}

// Get reads a single workflow by ID.
func (s *WorkflowStore) Get(id string) (*Workflow, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("get workflow %q: %w", id, err)
	}
	var w Workflow
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parse workflow %q: %w", id, err)
	}
	return &w, nil
}

// Save writes a workflow definition to disk.
func (s *WorkflowStore) Save(w *Workflow) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create workflow dir: %w", err)
	}
	w.UpdatedAt = time.Now()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = w.UpdatedAt
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow: %w", err)
	}
	path := filepath.Join(s.dir, w.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

// Delete removes a workflow definition from disk.
func (s *WorkflowStore) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete workflow %q: %w", id, err)
	}
	return nil
}

// --- Legacy aliases ---

// PipelineStore is a legacy alias for WorkflowStore.
type PipelineStore = WorkflowStore

// NewPipelineStore creates a workflow store (legacy alias).
func NewPipelineStore(dir string) *PipelineStore {
	return NewWorkflowStore(dir)
}

// --- Execution Store ---

// ExecutionStore persists workflow execution history.
type ExecutionStore interface {
	SaveExecution(exec *Execution) error
	GetExecution(id string) (*Execution, error)
	ListExecutions(workflowID string, limit int) ([]*Execution, error)
	DeleteExecution(id string) error
	ClearExecutions(workflowID string) (int, error)
}

// JSONExecutionStore implements ExecutionStore using JSON files on disk.
type JSONExecutionStore struct {
	mu  sync.Mutex
	dir string
}

// NewJSONExecutionStore creates an execution store that writes to the given directory.
func NewJSONExecutionStore(dir string) *JSONExecutionStore {
	return &JSONExecutionStore{dir: dir}
}

// SaveExecution persists an execution to a JSON file.
func (s *JSONExecutionStore) SaveExecution(exec *Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create execution dir: %w", err)
	}
	data, err := json.MarshalIndent(exec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal execution: %w", err)
	}
	path := filepath.Join(s.dir, exec.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

// GetExecution loads an execution by ID.
func (s *JSONExecutionStore) GetExecution(id string) (*Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("get execution %q: %w", id, err)
	}
	var exec Execution
	if err := json.Unmarshal(data, &exec); err != nil {
		return nil, fmt.Errorf("unmarshal execution: %w", err)
	}
	return &exec, nil
}

// ListExecutions returns recent executions, optionally filtered by workflow ID.
func (s *JSONExecutionStore) ListExecutions(workflowID string, limit int) ([]*Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list executions: %w", err)
	}

	var execs []*Execution
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var exec Execution
		if err := json.Unmarshal(data, &exec); err != nil {
			continue
		}
		if workflowID == "" || exec.WorkflowID == workflowID {
			execs = append(execs, &exec)
		}
	}

	sort.Slice(execs, func(i, j int) bool {
		return execs[i].StartedAt.After(execs[j].StartedAt)
	})

	if limit > 0 && len(execs) > limit {
		execs = execs[:limit]
	}
	return execs, nil
}

// DeleteExecution removes an execution file.
func (s *JSONExecutionStore) DeleteExecution(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete execution %q: %w", id, err)
	}
	return nil
}

// ClearExecutions deletes all executions, optionally filtered by workflowID.
func (s *JSONExecutionStore) ClearExecutions(workflowID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())

		if workflowID != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var exec Execution
			if json.Unmarshal(data, &exec) != nil || exec.WorkflowID != workflowID {
				continue
			}
		}

		if os.Remove(path) == nil {
			count++
		}
	}
	return count, nil
}

// --- Legacy aliases for RunStore ---

// RunStore is a legacy interface alias.
type RunStore = ExecutionStore

// JSONRunStore is a legacy alias.
type JSONRunStore = JSONExecutionStore

// NewJSONRunStore creates an execution store (legacy alias).
func NewJSONRunStore(dir string) *JSONRunStore {
	return NewJSONExecutionStore(dir)
}

// --- Variable Store ---

// VariableRepository defines the interface for variable storage.
type VariableRepository interface {
	List() []Variable
	Get(id string) (*Variable, error)
	Create(v Variable) (*Variable, error)
	Update(id string, v Variable) (*Variable, error)
	Delete(id string) error
}

// VariableStore manages key-value variables.
type VariableStore struct {
	mu   sync.Mutex
	dir  string
	vars []Variable
}

// NewVariableStore creates a variable store backed by a JSON file.
func NewVariableStore(dir string) *VariableStore {
	return &VariableStore{dir: dir}
}

func (s *VariableStore) path() string {
	return filepath.Join(s.dir, "variables.json")
}

func (s *VariableStore) load() {
	data, err := os.ReadFile(s.path())
	if err != nil {
		s.vars = nil
		return
	}
	json.Unmarshal(data, &s.vars)
}

func (s *VariableStore) save() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s.vars, "", "  ")
	return os.WriteFile(s.path(), data, 0o644)
}

// List returns all variables.
func (s *VariableStore) List() []Variable {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return s.vars
}

// Get returns a variable by ID.
func (s *VariableStore) Get(id string) (*Variable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, v := range s.vars {
		if v.ID == id {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("variable not found: %s", id)
}

// Create adds a new variable.
func (s *VariableStore) Create(v Variable) (*Variable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	v.ID = fmt.Sprintf("var_%d", time.Now().UnixMilli())
	s.vars = append(s.vars, v)
	return &v, s.save()
}

// Update modifies an existing variable.
func (s *VariableStore) Update(id string, v Variable) (*Variable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for i, existing := range s.vars {
		if existing.ID == id {
			v.ID = id
			s.vars[i] = v
			return &v, s.save()
		}
	}
	return nil, fmt.Errorf("variable not found: %s", id)
}

// Delete removes a variable.
func (s *VariableStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for i, v := range s.vars {
		if v.ID == id {
			s.vars = append(s.vars[:i], s.vars[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("variable not found: %s", id)
}

// --- Tag Store ---

// TagRepository defines the interface for tag storage.
type TagRepository interface {
	List() []Tag
	Get(id string) (*Tag, error)
	Create(t Tag) (*Tag, error)
	Update(id string, t Tag) (*Tag, error)
	Delete(id string) error
}

// TagStore manages workflow tags.
type TagStore struct {
	mu   sync.Mutex
	dir  string
	tags []Tag
}

// NewTagStore creates a tag store backed by a JSON file.
func NewTagStore(dir string) *TagStore {
	return &TagStore{dir: dir}
}

func (s *TagStore) path() string {
	return filepath.Join(s.dir, "tags.json")
}

func (s *TagStore) load() {
	data, err := os.ReadFile(s.path())
	if err != nil {
		s.tags = nil
		return
	}
	json.Unmarshal(data, &s.tags)
}

func (s *TagStore) save() error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s.tags, "", "  ")
	return os.WriteFile(s.path(), data, 0o644)
}

// List returns all tags.
func (s *TagStore) List() []Tag {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return s.tags
}

// Get returns a tag by ID.
func (s *TagStore) Get(id string) (*Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for _, t := range s.tags {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("tag not found: %s", id)
}

// Create adds a new tag.
func (s *TagStore) Create(t Tag) (*Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	now := time.Now()
	t.ID = fmt.Sprintf("tag_%d", now.UnixMilli())
	t.CreatedAt = now
	t.UpdatedAt = now
	s.tags = append(s.tags, t)
	return &t, s.save()
}

// Update modifies an existing tag.
func (s *TagStore) Update(id string, t Tag) (*Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for i, existing := range s.tags {
		if existing.ID == id {
			t.ID = id
			t.CreatedAt = existing.CreatedAt
			t.UpdatedAt = time.Now()
			s.tags[i] = t
			return &t, s.save()
		}
	}
	return nil, fmt.Errorf("tag not found: %s", id)
}

// Delete removes a tag.
func (s *TagStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	for i, t := range s.tags {
		if t.ID == id {
			s.tags = append(s.tags[:i], s.tags[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("tag not found: %s", id)
}

// --- DataTable Store ---

// DataTableRepository defines the interface for data table storage.
type DataTableRepository interface {
	ListTables() []DataTable
	GetTable(id string) (*DataTable, error)
	CreateTable(t DataTable) (*DataTable, error)
	UpdateTable(id string, t DataTable) (*DataTable, error)
	DeleteTable(id string) error
	ListRows(tableID string) []DataTableRow
	InsertRows(tableID string, rows []DataTableRow) ([]DataTableRow, error)
	UpdateRow(tableID string, rowID int, data map[string]any) (*DataTableRow, error)
	DeleteRow(tableID string, rowID int) error
}

// DataTableStore manages data tables and their rows.
type DataTableStore struct {
	dir string
}

// NewDataTableStore creates a data table store.
func NewDataTableStore(dir string) *DataTableStore {
	return &DataTableStore{dir: dir}
}

func (s *DataTableStore) tablesPath() string {
	return filepath.Join(s.dir, "tables.json")
}

func (s *DataTableStore) rowsPath(tableID string) string {
	return filepath.Join(s.dir, "rows_"+tableID+".json")
}

func (s *DataTableStore) loadTables() []DataTable {
	data, err := os.ReadFile(s.tablesPath())
	if err != nil {
		return nil
	}
	var tables []DataTable
	json.Unmarshal(data, &tables)
	return tables
}

func (s *DataTableStore) saveTables(tables []DataTable) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(tables, "", "  ")
	return os.WriteFile(s.tablesPath(), data, 0o644)
}

func (s *DataTableStore) loadRows(tableID string) []DataTableRow {
	data, err := os.ReadFile(s.rowsPath(tableID))
	if err != nil {
		return nil
	}
	var rows []DataTableRow
	json.Unmarshal(data, &rows)
	return rows
}

func (s *DataTableStore) saveRows(tableID string, rows []DataTableRow) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	return os.WriteFile(s.rowsPath(tableID), data, 0o644)
}

// ListTables returns all data tables.
func (s *DataTableStore) ListTables() []DataTable {
	return s.loadTables()
}

// GetTable returns a table by ID.
func (s *DataTableStore) GetTable(id string) (*DataTable, error) {
	for _, t := range s.loadTables() {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("table not found: %s", id)
}

// CreateTable creates a new data table.
func (s *DataTableStore) CreateTable(t DataTable) (*DataTable, error) {
	tables := s.loadTables()
	now := time.Now()
	t.ID = fmt.Sprintf("dt_%d", now.UnixMilli())
	t.CreatedAt = now
	t.UpdatedAt = now
	tables = append(tables, t)
	return &t, s.saveTables(tables)
}

// UpdateTable updates a data table's metadata.
func (s *DataTableStore) UpdateTable(id string, t DataTable) (*DataTable, error) {
	tables := s.loadTables()
	for i, existing := range tables {
		if existing.ID == id {
			t.ID = id
			t.CreatedAt = existing.CreatedAt
			t.UpdatedAt = time.Now()
			tables[i] = t
			return &t, s.saveTables(tables)
		}
	}
	return nil, fmt.Errorf("table not found: %s", id)
}

// DeleteTable removes a table and its rows.
func (s *DataTableStore) DeleteTable(id string) error {
	tables := s.loadTables()
	for i, t := range tables {
		if t.ID == id {
			tables = append(tables[:i], tables[i+1:]...)
			os.Remove(s.rowsPath(id))
			return s.saveTables(tables)
		}
	}
	return fmt.Errorf("table not found: %s", id)
}

// ListRows returns all rows for a table.
func (s *DataTableStore) ListRows(tableID string) []DataTableRow {
	return s.loadRows(tableID)
}

// InsertRows adds rows to a table.
func (s *DataTableStore) InsertRows(tableID string, newRows []DataTableRow) ([]DataTableRow, error) {
	rows := s.loadRows(tableID)
	nextID := 1
	for _, r := range rows {
		if r.ID >= nextID {
			nextID = r.ID + 1
		}
	}
	now := time.Now()
	for i := range newRows {
		newRows[i].ID = nextID
		newRows[i].CreatedAt = now
		newRows[i].UpdatedAt = now
		nextID++
	}
	rows = append(rows, newRows...)
	return newRows, s.saveRows(tableID, rows)
}

// UpdateRow updates a single row by ID.
func (s *DataTableStore) UpdateRow(tableID string, rowID int, data map[string]any) (*DataTableRow, error) {
	rows := s.loadRows(tableID)
	for i, r := range rows {
		if r.ID == rowID {
			rows[i].Data = data
			rows[i].UpdatedAt = time.Now()
			return &rows[i], s.saveRows(tableID, rows)
		}
	}
	return nil, fmt.Errorf("row %d not found in table %s", rowID, tableID)
}

// DeleteRow removes a row by ID.
func (s *DataTableStore) DeleteRow(tableID string, rowID int) error {
	rows := s.loadRows(tableID)
	for i, r := range rows {
		if r.ID == rowID {
			rows = append(rows[:i], rows[i+1:]...)
			return s.saveRows(tableID, rows)
		}
	}
	return fmt.Errorf("row %d not found in table %s", rowID, tableID)
}
