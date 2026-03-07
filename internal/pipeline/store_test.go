package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkflowStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewWorkflowStore(dir)

	w := &Workflow{
		ID:   "test-pipe",
		Name: "test-pipe",
		Nodes: []Node{
			{Name: "a", Type: "http.request"},
		},
	}

	if err := store.Save(w); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Get("test-pipe")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Name != "test-pipe" {
		t.Errorf("name = %q, want %q", loaded.Name, "test-pipe")
	}
	if len(loaded.Nodes) != 1 {
		t.Errorf("node count = %d, want 1", len(loaded.Nodes))
	}
}

func TestWorkflowStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewWorkflowStore(dir)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		w := &Workflow{
			ID:    name,
			Name:  name,
			Nodes: []Node{{Name: "a", Type: "x"}},
		}
		if err := store.Save(w); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("count = %d, want 3", len(list))
	}
}

func TestWorkflowStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store := NewWorkflowStore(dir)

	w := &Workflow{
		ID:    "to-delete",
		Name:  "to-delete",
		Nodes: []Node{{Name: "a", Type: "x"}},
	}
	if err := store.Save(w); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := store.Delete("to-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := store.Get("to-delete")
	if err == nil {
		t.Error("expected error loading deleted workflow")
	}
}

func TestWorkflowStore_ListEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	store := NewWorkflowStore(dir)

	list, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("count = %d, want 0", len(list))
	}
}

func TestWorkflowStore_DeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store := NewWorkflowStore(dir)

	if err := store.Delete("nope"); err != nil {
		t.Errorf("delete nonexistent should not error, got %v", err)
	}
}

func TestWorkflowStore_SaveCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	store := NewWorkflowStore(dir)

	w := &Workflow{
		ID:    "auto-dir",
		Name:  "auto-dir",
		Nodes: []Node{{Name: "a", Type: "x"}},
	}
	if err := store.Save(w); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

// --- Execution Store tests ---

func TestJSONExecutionStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONExecutionStore(dir)

	exec := &Execution{
		ID:         "run_123",
		WorkflowID: "my-pipe",
		Status:     ExecSuccess,
		StartedAt:  time.Now().Add(-time.Minute),
		Finished:   true,
		Data: &ExecutionData{
			ResultData: ResultData{
				RunData: map[string][]NodeRunData{
					"a": {{Data: map[string]any{"output": "ok"}}},
				},
			},
		},
	}

	if err := store.SaveExecution(exec); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.GetExecution("run_123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.Status != ExecSuccess {
		t.Errorf("status = %v, want %v", loaded.Status, ExecSuccess)
	}
	if loaded.WorkflowID != "my-pipe" {
		t.Errorf("workflowId = %q, want %q", loaded.WorkflowID, "my-pipe")
	}
}

func TestJSONExecutionStore_ListExecutions(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONExecutionStore(dir)

	now := time.Now()
	for i := 0; i < 5; i++ {
		exec := &Execution{
			ID:         fmt.Sprintf("run_%d", i),
			WorkflowID: "pipe-a",
			Status:     ExecSuccess,
			StartedAt:  now.Add(time.Duration(i) * time.Minute),
		}
		if err := store.SaveExecution(exec); err != nil {
			t.Fatalf("save run_%d: %v", i, err)
		}
	}

	// Add an execution for a different workflow.
	if err := store.SaveExecution(&Execution{
		ID: "run_other", WorkflowID: "pipe-b", Status: ExecSuccess, StartedAt: now,
	}); err != nil {
		t.Fatalf("save other: %v", err)
	}

	// List all for pipe-a.
	execs, err := store.ListExecutions("pipe-a", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(execs) != 5 {
		t.Errorf("count = %d, want 5", len(execs))
	}

	// List with limit.
	execs, err = store.ListExecutions("pipe-a", 3)
	if err != nil {
		t.Fatalf("list limited: %v", err)
	}
	if len(execs) != 3 {
		t.Errorf("limited count = %d, want 3", len(execs))
	}

	// Verify sorted descending by start time.
	for i := 1; i < len(execs); i++ {
		if execs[i].StartedAt.After(execs[i-1].StartedAt) {
			t.Error("executions not sorted descending by start time")
		}
	}

	// List all workflows.
	all, err := store.ListExecutions("", 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 6 {
		t.Errorf("all count = %d, want 6", len(all))
	}
}

func TestJSONExecutionStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONExecutionStore(dir)

	_, err := store.GetExecution("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent execution")
	}
}

func TestJSONExecutionStore_ListEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "empty")
	store := NewJSONExecutionStore(dir)

	execs, err := store.ListExecutions("", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(execs) != 0 {
		t.Errorf("count = %d, want 0", len(execs))
	}
}
