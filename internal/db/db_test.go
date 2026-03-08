package db

import (
	"testing"
	"time"

	"github.com/neomody77/kuro/internal/audit"
	"github.com/neomody77/kuro/internal/pipeline"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// --- Execution Store ---

func TestExecutionStore_SaveAndGet(t *testing.T) {
	db := openTestDB(t)
	store := NewExecutionStore(db)

	now := time.Now().Truncate(time.Second)
	stopped := now.Add(5 * time.Second)
	exec := &pipeline.Execution{
		ID:         "exec_1",
		WorkflowID: "wf_1",
		Status:     pipeline.ExecSuccess,
		Mode:       pipeline.ModeManual,
		StartedAt:  now,
		StoppedAt:  &stopped,
		Finished:   true,
		Data: &pipeline.ExecutionData{
			ResultData: pipeline.ResultData{
				LastNodeExecuted: "node_a",
			},
		},
	}

	if err := store.SaveExecution(exec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.GetExecution("exec_1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.WorkflowID != "wf_1" {
		t.Errorf("workflow_id = %q, want wf_1", got.WorkflowID)
	}
	if got.Status != pipeline.ExecSuccess {
		t.Errorf("status = %v, want success", got.Status)
	}
	if got.Data == nil || got.Data.ResultData.LastNodeExecuted != "node_a" {
		t.Error("data not preserved")
	}
}

func TestExecutionStore_ListAndDelete(t *testing.T) {
	db := openTestDB(t)
	store := NewExecutionStore(db)

	for i := 0; i < 5; i++ {
		store.SaveExecution(&pipeline.Execution{
			ID:         "exec_" + string(rune('a'+i)),
			WorkflowID: "wf_1",
			Status:     pipeline.ExecSuccess,
			Mode:       pipeline.ModeManual,
			StartedAt:  time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	list, err := store.ListExecutions("wf_1", 3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("list len = %d, want 3", len(list))
	}

	// Delete one
	if err := store.DeleteExecution("exec_a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list2, _ := store.ListExecutions("wf_1", 100)
	if len(list2) != 4 {
		t.Errorf("after delete len = %d, want 4", len(list2))
	}
}

func TestExecutionStore_ClearExecutions(t *testing.T) {
	db := openTestDB(t)
	store := NewExecutionStore(db)

	for i := 0; i < 3; i++ {
		store.SaveExecution(&pipeline.Execution{
			ID:         "exec_" + string(rune('a'+i)),
			WorkflowID: "wf_1",
			Status:     pipeline.ExecSuccess,
			StartedAt:  time.Now(),
		})
	}

	n, err := store.ClearExecutions("wf_1")
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if n != 3 {
		t.Errorf("cleared = %d, want 3", n)
	}

	list, _ := store.ListExecutions("", 100)
	if len(list) != 0 {
		t.Errorf("after clear len = %d, want 0", len(list))
	}
}

func TestExecutionStore_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewExecutionStore(db)

	_, err := store.GetExecution("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent execution")
	}
}

// --- Variable Store ---

func TestVariableStore_CRUD(t *testing.T) {
	db := openTestDB(t)
	store := NewVariableStore(db)

	// Create
	v, err := store.Create(pipeline.Variable{Key: "API_KEY", Value: "secret", Type: "string"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if v.ID == "" {
		t.Error("expected auto-generated ID")
	}

	// List
	list := store.List()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].Key != "API_KEY" {
		t.Errorf("key = %q, want API_KEY", list[0].Key)
	}

	// Get
	got, err := store.Get(v.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Value != "secret" {
		t.Errorf("value = %q, want secret", got.Value)
	}

	// Update
	updated, err := store.Update(v.ID, pipeline.Variable{Key: "API_KEY", Value: "new_secret", Type: "string"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Value != "new_secret" {
		t.Errorf("updated value = %q", updated.Value)
	}

	// Delete
	if err := store.Delete(v.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list = store.List()
	if len(list) != 0 {
		t.Errorf("after delete len = %d", len(list))
	}
}

// --- Tag Store ---

func TestTagStore_CRUD(t *testing.T) {
	db := openTestDB(t)
	store := NewTagStore(db)

	// Create
	tag, err := store.Create(pipeline.Tag{Name: "production"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tag.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if tag.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// List
	list := store.List()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	// Get
	got, err := store.Get(tag.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "production" {
		t.Errorf("name = %q", got.Name)
	}

	// Update
	updated, err := store.Update(tag.ID, pipeline.Tag{Name: "staging"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "staging" {
		t.Errorf("updated name = %q", updated.Name)
	}

	// Delete
	if err := store.Delete(tag.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(store.List()) != 0 {
		t.Error("expected empty after delete")
	}
}

// --- DataTable Store ---

func TestDataTableStore_TableCRUD(t *testing.T) {
	db := openTestDB(t)
	store := NewDataTableStore(db)

	// Create table
	tbl, err := store.CreateTable(pipeline.DataTable{
		Name: "users",
		Columns: []pipeline.DataColumn{
			{ID: "c1", Name: "name", Type: "string"},
			{ID: "c2", Name: "age", Type: "number"},
		},
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if tbl.ID == "" {
		t.Error("expected auto-generated table ID")
	}

	// List tables
	tables := store.ListTables()
	if len(tables) != 1 {
		t.Fatalf("list tables = %d, want 1", len(tables))
	}

	// Get table
	got, err := store.GetTable(tbl.ID)
	if err != nil {
		t.Fatalf("get table: %v", err)
	}
	if got.Name != "users" {
		t.Errorf("table name = %q", got.Name)
	}
	if len(got.Columns) != 2 {
		t.Errorf("columns = %d, want 2", len(got.Columns))
	}

	// Update table
	_, err = store.UpdateTable(tbl.ID, pipeline.DataTable{Name: "people"})
	if err != nil {
		t.Fatalf("update table: %v", err)
	}

	// Delete table
	if err := store.DeleteTable(tbl.ID); err != nil {
		t.Fatalf("delete table: %v", err)
	}
	if len(store.ListTables()) != 0 {
		t.Error("expected empty after delete")
	}
}

func TestDataTableStore_RowCRUD(t *testing.T) {
	db := openTestDB(t)
	store := NewDataTableStore(db)

	tbl, _ := store.CreateTable(pipeline.DataTable{Name: "items"})

	// Insert rows
	rows, err := store.InsertRows(tbl.ID, []pipeline.DataTableRow{
		{Data: map[string]any{"name": "Alice"}},
		{Data: map[string]any{"name": "Bob"}},
	})
	if err != nil {
		t.Fatalf("insert rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("inserted = %d, want 2", len(rows))
	}

	// List rows
	listed := store.ListRows(tbl.ID)
	if len(listed) != 2 {
		t.Fatalf("list rows = %d, want 2", len(listed))
	}

	// Update row
	updated, err := store.UpdateRow(tbl.ID, rows[0].ID, map[string]any{"name": "Alice Updated"})
	if err != nil {
		t.Fatalf("update row: %v", err)
	}
	if updated.Data["name"] != "Alice Updated" {
		t.Errorf("updated name = %v", updated.Data["name"])
	}

	// Delete row
	if err := store.DeleteRow(tbl.ID, rows[1].ID); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	if len(store.ListRows(tbl.ID)) != 1 {
		t.Error("expected 1 row after delete")
	}
}

// --- Chat Store ---

func TestChatStore_SessionAndMessages(t *testing.T) {
	db := openTestDB(t)
	store := NewChatStore(db)

	// Create session
	err := store.CreateSession(ChatSession{
		ID:        "s1",
		UserID:    "alice",
		Title:     "Test Chat",
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// List sessions
	sessions, err := store.ListSessions("alice")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(sessions))
	}
	if sessions[0].Title != "Test Chat" {
		t.Errorf("title = %q", sessions[0].Title)
	}

	// Append messages
	store.AppendMessage(ChatMessage{ID: "m1", SessionID: "s1", Role: "user", Content: "Hello", Timestamp: time.Now()})
	store.AppendMessage(ChatMessage{ID: "m2", SessionID: "s1", Role: "assistant", Content: "Hi!", Timestamp: time.Now()})

	// Load messages
	msgs, err := store.LoadMessages("s1")
	if err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("roles: %q, %q", msgs[0].Role, msgs[1].Role)
	}

	// Update title
	store.UpdateSessionTitle("s1", "Updated Title")
	sess, _ := store.GetSession("s1")
	if sess.Title != "Updated Title" {
		t.Errorf("title after update = %q", sess.Title)
	}

	// Delete session (should cascade delete messages)
	store.DeleteSession("s1")
	sessions, _ = store.ListSessions("alice")
	if len(sessions) != 0 {
		t.Error("expected empty after delete")
	}
	msgs, _ = store.LoadMessages("s1")
	if len(msgs) != 0 {
		t.Error("expected messages deleted with session")
	}
}

// --- Audit Store ---

func TestAuditStore_LogAndQuery(t *testing.T) {
	db := openTestDB(t)
	store := NewAuditStore(db)

	// Log entries
	for i := 0; i < 5; i++ {
		store.Log(audit.Entry{
			TraceID:   "trace_1",
			UserID:    "alice",
			Type:      audit.TypeSkillExec,
			SessionID: "s1",
			Action:    "document:list",
			Detail:    "ok",
		})
	}
	store.Log(audit.Entry{
		TraceID: "trace_2",
		UserID:  "alice",
		Type:    audit.TypeUserAction,
		Action:  "chat:send",
		Detail:  "Hello",
	})

	// Query all
	entries, total, err := store.Query(audit.QueryFilter{Limit: 100})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if total != 6 {
		t.Errorf("total = %d, want 6", total)
	}
	if len(entries) != 6 {
		t.Errorf("entries = %d, want 6", len(entries))
	}

	// Query by type
	entries, total, _ = store.Query(audit.QueryFilter{Type: audit.TypeSkillExec})
	if total != 5 {
		t.Errorf("skill_exec total = %d, want 5", total)
	}

	// Query by trace
	entries, total, _ = store.Query(audit.QueryFilter{TraceID: "trace_2"})
	if total != 1 {
		t.Errorf("trace_2 total = %d, want 1", total)
	}
	if entries[0].Action != "chat:send" {
		t.Errorf("action = %q", entries[0].Action)
	}

	// Query with limit and offset
	entries, _, _ = store.Query(audit.QueryFilter{Limit: 2, Offset: 0})
	if len(entries) != 2 {
		t.Errorf("limited entries = %d, want 2", len(entries))
	}
}

// --- Audit Logger ---

func TestAuditLogger(t *testing.T) {
	db := openTestDB(t)
	store := NewAuditStore(db)
	logger := audit.NewLogger(store)

	logger.LogUserAction("alice", "s1", "trace_1", "chat:send", "Hello")
	logger.LogSkillExec("alice", "s1", "trace_1", "document:list", 42, nil)
	logger.LogSystem("startup", "server started")

	entries, total, _ := store.Query(audit.QueryFilter{})
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	_ = entries
}

// --- DB Open/Close ---

func TestOpenFileBased(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/test.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Verify migration applied
	var version int
	db.SQL().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
}

func TestUserDBCache(t *testing.T) {
	dir := t.TempDir()
	cache := NewUserDBCache(dir)
	defer cache.Close()

	db1, err := cache.Get("alice")
	if err != nil {
		t.Fatalf("get alice: %v", err)
	}

	db2, err := cache.Get("alice")
	if err != nil {
		t.Fatalf("get alice again: %v", err)
	}

	if db1 != db2 {
		t.Error("expected same DB instance for same user")
	}

	db3, err := cache.Get("bob")
	if err != nil {
		t.Fatalf("get bob: %v", err)
	}
	if db1 == db3 {
		t.Error("expected different DB for different user")
	}
}
