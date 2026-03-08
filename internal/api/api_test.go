package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/neomody77/kuro/internal/auth"
	"github.com/neomody77/kuro/internal/chat"
	"github.com/neomody77/kuro/internal/pipeline"
	"github.com/neomody77/kuro/internal/provider"
	"github.com/neomody77/kuro/internal/settings"
	"github.com/neomody77/kuro/internal/skill"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	response string
}

func (m *mockProvider) Complete(_ context.Context, req *provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{Content: m.response, Model: "mock"}, nil
}

// setupDeps creates a Deps instance with a temp directory and all dependencies wired up.
func setupDeps(t *testing.T) *Deps {
	t.Helper()
	dataDir := t.TempDir()

	// Create user directory structure for user "default".
	if _, err := auth.EnsureUserDir(dataDir, "default"); err != nil {
		t.Fatalf("EnsureUserDir: %v", err)
	}

	// Reference the repo dir (pipelines dir is created on demand by WorkflowStore.Save).
	repoDir := filepath.Join(dataDir, "users", "default", "repo")

	// Settings store
	settingsStore := settings.NewStore(filepath.Join(dataDir, "settings.yaml"))

	// Skill registry (no executor needed for listing)
	registry := skill.NewRegistry(nil)
	registry.Register(&skill.Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Inputs: []skill.SkillParam{
			{Name: "input1", Required: true},
		},
	})

	// Chat service with a mock provider
	mock := &mockProvider{response: "Hello from mock"}
	chatSvc := chat.NewService(registry, mock, "mock-model", dataDir)

	// Execution store
	execStore := pipeline.NewJSONExecutionStore(filepath.Join(repoDir, "executions"))

	return &Deps{
		DataDir:       dataDir,
		ExecStore:     execStore,
		ChatService:   chatSvc,
		SkillRegistry: registry,
		SettingsStore: settingsStore,
		OnSettingsChanged: func() {
			// no-op for tests
		},
		// ADKRunner and ADKSessionSvc left nil — those endpoints are tested separately.
		// Executor left nil — not needed for these handler tests.
	}
}

// callHandler wraps a plain http.HandlerFunc through auth middleware (empty
// tokens = single-user "default") and calls it with the given method/path/body.
func callHandler(handler http.HandlerFunc, method, path string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Wrap through auth middleware so auth.GetUser returns "default".
	authMW := auth.Middleware(map[string]string{}) // empty map → single-user mode
	wrapped := authMW(handler)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)
	return rr
}

// callDepsHandler is a convenience for handlers that take deps.
func callDepsHandler(deps *Deps, fn depsHandler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	return callHandler(withDeps(deps, fn), method, path, body)
}

// jsonBody marshals v to a *bytes.Reader for use as request body.
func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("jsonBody marshal: %v", err)
	}
	return bytes.NewReader(data)
}

// decodeJSON decodes the recorder body into dst.
func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(dst); err != nil {
		t.Fatalf("decodeJSON: %v (body: %s)", err, rr.Body.String())
	}
}

// --- Tests ---

func TestHealthEndpoint(t *testing.T) {
	rr := callHandler(http.HandlerFunc(handleHealth), "GET", "/api/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result map[string]string
	decodeJSON(t, rr, &result)
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", result["status"])
	}
}

func TestWorkflowCRUD(t *testing.T) {
	deps := setupDeps(t)

	// Create
	wfJSON := map[string]any{
		"id":   "wf_test1",
		"name": "Test Workflow",
		"nodes": []any{
			map[string]any{
				"name":        "Start",
				"type":        "n8n-nodes-base.manualTrigger",
				"typeVersion": 1,
				"position":    []any{0, 0},
			},
		},
	}
	rr := callDepsHandler(deps, handleCreateWorkflow, "POST", "/api/v1/workflows", jsonBody(t, wfJSON))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var created pipeline.Workflow
	decodeJSON(t, rr, &created)
	if created.ID != "wf_test1" {
		t.Fatalf("create: expected id=wf_test1, got %q", created.ID)
	}
	if created.Name != "Test Workflow" {
		t.Fatalf("create: expected name='Test Workflow', got %q", created.Name)
	}

	// Get — need to set path value
	rr = callDepsHandlerWithPathValue(deps, handleGetWorkflow, "GET", "/api/v1/workflows/wf_test1", nil, "id", "wf_test1")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got pipeline.Workflow
	decodeJSON(t, rr, &got)
	if got.Name != "Test Workflow" {
		t.Fatalf("get: expected name='Test Workflow', got %q", got.Name)
	}

	// List
	rr = callDepsHandler(deps, handleListWorkflows, "GET", "/api/v1/workflows", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var list []pipeline.Workflow
	decodeJSON(t, rr, &list)
	if len(list) != 1 {
		t.Fatalf("list: expected 1 workflow, got %d", len(list))
	}

	// Delete
	rr = callDepsHandlerWithPathValue(deps, handleDeleteWorkflow, "DELETE", "/api/v1/workflows/wf_test1", nil, "id", "wf_test1")
	if rr.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify deleted
	rr = callDepsHandlerWithPathValue(deps, handleGetWorkflow, "GET", "/api/v1/workflows/wf_test1", nil, "id", "wf_test1")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", rr.Code)
	}
}

func TestDocumentCRUD(t *testing.T) {
	deps := setupDeps(t)

	// Create (PUT) a document
	rr := callDepsHandlerWithPathValue(deps, handlePutDocument, "PUT", "/api/documents/notes/hello.md", jsonBody(t, map[string]string{
		"content": "# Hello World",
	}), "path", "notes/hello.md")
	if rr.Code != http.StatusOK {
		t.Fatalf("put: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Get
	rr = callDepsHandlerWithPathValue(deps, handleGetDocument, "GET", "/api/documents/notes/hello.md", nil, "path", "notes/hello.md")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var doc map[string]any
	decodeJSON(t, rr, &doc)
	if doc["content"] != "# Hello World" {
		t.Fatalf("get: expected content='# Hello World', got %q", doc["content"])
	}

	// List
	rr = callDepsHandler(deps, handleListDocuments, "GET", "/api/documents", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var docs []map[string]any
	decodeJSON(t, rr, &docs)
	if len(docs) != 1 {
		t.Fatalf("list: expected 1 entry, got %d", len(docs))
	}

	// Delete
	rr = callDepsHandlerWithPathValue(deps, handleDeleteDocument, "DELETE", "/api/documents/notes/hello.md", nil, "path", "notes/hello.md")
	if rr.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify deleted
	rr = callDepsHandlerWithPathValue(deps, handleGetDocument, "GET", "/api/documents/notes/hello.md", nil, "path", "notes/hello.md")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestChatEndpoints(t *testing.T) {
	deps := setupDeps(t)

	// List sessions — should be empty initially
	rr := callDepsHandler(deps, handleListSessions, "GET", "/api/chat/sessions", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list sessions: expected 200, got %d", rr.Code)
	}
	var sessions []chat.SessionInfo
	decodeJSON(t, rr, &sessions)
	if len(sessions) != 0 {
		t.Fatalf("list sessions: expected 0, got %d", len(sessions))
	}

	// Create session
	rr = callDepsHandler(deps, handleCreateSession, "POST", "/api/chat/sessions", nil)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create session: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var sessionInfo chat.SessionInfo
	decodeJSON(t, rr, &sessionInfo)
	if sessionInfo.ID == "" {
		t.Fatalf("create session: got empty ID")
	}
	sessionID := sessionInfo.ID

	// Send a chat message
	rr = callDepsHandler(deps, handleChat, "POST", "/api/chat", jsonBody(t, map[string]string{
		"message":    "Hello",
		"session_id": sessionID,
	}))
	if rr.Code != http.StatusOK {
		t.Fatalf("chat: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var chatResp chat.Response
	decodeJSON(t, rr, &chatResp)
	if chatResp.Message.Role != "assistant" {
		t.Fatalf("chat: expected role=assistant, got %q", chatResp.Message.Role)
	}
	if chatResp.Message.Content == "" {
		t.Fatalf("chat: got empty content")
	}

	// Get chat history
	rr = callDepsHandler(deps, handleChatHistory, "GET", "/api/chat/history?session_id="+sessionID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("history: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var history []chat.Message
	decodeJSON(t, rr, &history)
	if len(history) < 2 {
		t.Fatalf("history: expected at least 2 messages (user+assistant), got %d", len(history))
	}
	if history[0].Role != "user" {
		t.Fatalf("history: first message should be user, got %q", history[0].Role)
	}
	if history[1].Role != "assistant" {
		t.Fatalf("history: second message should be assistant, got %q", history[1].Role)
	}
}

func TestSkillEndpoints(t *testing.T) {
	deps := setupDeps(t)

	// List skills
	rr := callDepsHandler(deps, handleListSkills, "GET", "/api/skills", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var skills []map[string]string
	decodeJSON(t, rr, &skills)
	if len(skills) != 1 {
		t.Fatalf("list: expected 1 skill, got %d", len(skills))
	}
	if skills[0]["name"] != "test-skill" {
		t.Fatalf("list: expected name=test-skill, got %q", skills[0]["name"])
	}

	// Get skill
	rr = callDepsHandlerWithPathValue(deps, handleGetSkill, "GET", "/api/skills/test-skill", nil, "id", "test-skill")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var sk skill.Skill
	decodeJSON(t, rr, &sk)
	if sk.Name != "test-skill" {
		t.Fatalf("get: expected name=test-skill, got %q", sk.Name)
	}
	if sk.Description != "A test skill" {
		t.Fatalf("get: expected description='A test skill', got %q", sk.Description)
	}

	// Get non-existent skill
	rr = callDepsHandlerWithPathValue(deps, handleGetSkill, "GET", "/api/skills/nonexistent", nil, "id", "nonexistent")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get nonexistent: expected 404, got %d", rr.Code)
	}
}

func TestVariableCRUD(t *testing.T) {
	deps := setupDeps(t)

	// List — empty initially
	rr := callDepsHandler(deps, handleListVariables, "GET", "/api/v1/variables", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rr.Code)
	}
	var vars []pipeline.Variable
	decodeJSON(t, rr, &vars)
	if len(vars) != 0 {
		t.Fatalf("list: expected 0, got %d", len(vars))
	}

	// Create
	rr = callDepsHandler(deps, handleCreateVariable, "POST", "/api/v1/variables", jsonBody(t, pipeline.Variable{
		Key:   "MY_VAR",
		Value: "hello",
		Type:  "string",
	}))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var created pipeline.Variable
	decodeJSON(t, rr, &created)
	if created.Key != "MY_VAR" {
		t.Fatalf("create: expected key=MY_VAR, got %q", created.Key)
	}
	if created.ID == "" {
		t.Fatalf("create: got empty ID")
	}
	varID := created.ID

	// List — should have one
	rr = callDepsHandler(deps, handleListVariables, "GET", "/api/v1/variables", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rr.Code)
	}
	decodeJSON(t, rr, &vars)
	if len(vars) != 1 {
		t.Fatalf("list: expected 1, got %d", len(vars))
	}

	// Update
	rr = callDepsHandlerWithPathValue(deps, handleUpdateVariable, "PUT", "/api/v1/variables/"+varID, jsonBody(t, pipeline.Variable{
		Key:   "MY_VAR",
		Value: "updated",
		Type:  "string",
	}), "id", varID)
	if rr.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var updated pipeline.Variable
	decodeJSON(t, rr, &updated)
	if updated.Value != "updated" {
		t.Fatalf("update: expected value=updated, got %q", updated.Value)
	}

	// Delete
	rr = callDepsHandlerWithPathValue(deps, handleDeleteVariable, "DELETE", "/api/v1/variables/"+varID, nil, "id", varID)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify deleted
	rr = callDepsHandler(deps, handleListVariables, "GET", "/api/v1/variables", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list after delete: expected 200, got %d", rr.Code)
	}
	decodeJSON(t, rr, &vars)
	if len(vars) != 0 {
		t.Fatalf("list after delete: expected 0, got %d", len(vars))
	}
}

// callDepsHandlerWithPathValue is like callDepsHandler but also sets path values
// on the request (for handlers that use r.PathValue).
func callDepsHandlerWithPathValue(deps *Deps, fn depsHandler, method, urlPath string, body io.Reader, pathParams ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, urlPath, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Set path values in pairs: key1, val1, key2, val2, ...
	for i := 0; i+1 < len(pathParams); i += 2 {
		req.SetPathValue(pathParams[i], pathParams[i+1])
	}
	// Wrap through auth middleware
	authMW := auth.Middleware(map[string]string{})
	handler := withDeps(deps, fn)
	wrapped := authMW(handler)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)
	return rr
}
