// Package api implements the REST API handlers for Kuro.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"

	"github.com/neomody77/kuro/internal/audit"
	"github.com/neomody77/kuro/internal/auth"
	"github.com/neomody77/kuro/internal/chat"
	"github.com/neomody77/kuro/internal/credential"
	"github.com/neomody77/kuro/internal/db"
	"github.com/neomody77/kuro/internal/document"
	"github.com/neomody77/kuro/internal/events"
	"github.com/neomody77/kuro/internal/gitstore"
	"github.com/neomody77/kuro/internal/pipeline"
	"github.com/neomody77/kuro/internal/server"
	"github.com/neomody77/kuro/internal/settings"
	"github.com/neomody77/kuro/internal/skill"
)

// Deps holds the shared dependencies for all API handlers.
type Deps struct {
	DataDir           string
	Executor          *pipeline.Executor
	ExecStore         *pipeline.JSONExecutionStore
	ChatService       *chat.Service
	SkillRegistry     *skill.Registry
	SettingsStore     *settings.Store
	OnSettingsChanged func()

	// ADK agent loop
	ADKRunner     *runner.Runner
	ADKSessionSvc session.Service

	// SQLite stores (optional — if nil, falls back to JSON file stores)
	DBCache     *db.UserDBCache
	AuditLogger *audit.Logger

	// Real-time event hub for SSE
	EventHub *events.Hub

	// Skill config store (backed by credential store)
	SkillConfigStore skill.SkillConfigStore
}

// Register registers all API routes on the server.
func Register(srv *server.Server, deps *Deps) {
	// Health (no auth)
	srv.HandleAPI("GET /api/health", handleHealth)

	// Workflows (n8n-compatible)
	srv.HandleAPI("GET /api/v1/workflows", withDeps(deps, handleListWorkflows))
	srv.HandleAPI("POST /api/v1/workflows", withDeps(deps, handleCreateWorkflow))
	srv.HandleAPI("GET /api/v1/workflows/{id}", withDeps(deps, handleGetWorkflow))
	srv.HandleAPI("PUT /api/v1/workflows/{id}", withDeps(deps, handleUpdateWorkflow))
	srv.HandleAPI("DELETE /api/v1/workflows/{id}", withDeps(deps, handleDeleteWorkflow))
	srv.HandleAPI("POST /api/v1/workflows/{id}/activate", withDeps(deps, handleActivateWorkflow))
	srv.HandleAPI("POST /api/v1/workflows/{id}/deactivate", withDeps(deps, handleDeactivateWorkflow))

	// Legacy pipeline routes (redirect to workflow)
	srv.HandleAPI("GET /api/pipelines", withDeps(deps, handleListWorkflows))
	srv.HandleAPI("POST /api/pipelines", withDeps(deps, handleCreateWorkflow))
	srv.HandleAPI("GET /api/pipelines/{id}", withDeps(deps, handleGetWorkflow))
	srv.HandleAPI("PUT /api/pipelines/{id}", withDeps(deps, handleUpdateWorkflow))
	srv.HandleAPI("DELETE /api/pipelines/{id}", withDeps(deps, handleDeleteWorkflow))
	srv.HandleAPI("POST /api/pipelines/{id}/run", withDeps(deps, handleRunWorkflow))
	srv.HandleAPI("GET /api/pipelines/{id}/runs", withDeps(deps, handleListWorkflowExecutions))

	// Executions (n8n-compatible)
	srv.HandleAPI("GET /api/v1/executions", withDeps(deps, handleListExecutions))
	srv.HandleAPI("GET /api/v1/executions/{id}", withDeps(deps, handleGetExecution))
	srv.HandleAPI("DELETE /api/v1/executions/{id}", withDeps(deps, handleDeleteExecution))
	srv.HandleAPI("POST /api/v1/executions/clear", withDeps(deps, handleClearExecutions))

	// Credentials (n8n-compatible)
	srv.HandleAPI("GET /api/v1/credentials", withDeps(deps, handleListCredentials))
	srv.HandleAPI("POST /api/v1/credentials", withDeps(deps, handleCreateCredential))
	srv.HandleAPI("GET /api/v1/credentials/{id}", withDeps(deps, handleGetCredential))
	srv.HandleAPI("PATCH /api/v1/credentials/{id}", withDeps(deps, handleUpdateCredential))
	srv.HandleAPI("DELETE /api/v1/credentials/{id}", withDeps(deps, handleDeleteCredential))
	// Legacy routes
	srv.HandleAPI("GET /api/credentials", withDeps(deps, handleListCredentials))
	srv.HandleAPI("POST /api/credentials", withDeps(deps, handleCreateCredential))
	srv.HandleAPI("GET /api/credentials/{id}", withDeps(deps, handleGetCredential))
	srv.HandleAPI("PUT /api/credentials/{id}", withDeps(deps, handleUpdateCredential))
	srv.HandleAPI("DELETE /api/credentials/{id}", withDeps(deps, handleDeleteCredential))

	// Variables (n8n-compatible)
	srv.HandleAPI("GET /api/v1/variables", withDeps(deps, handleListVariables))
	srv.HandleAPI("POST /api/v1/variables", withDeps(deps, handleCreateVariable))
	srv.HandleAPI("PUT /api/v1/variables/{id}", withDeps(deps, handleUpdateVariable))
	srv.HandleAPI("DELETE /api/v1/variables/{id}", withDeps(deps, handleDeleteVariable))

	// Tags (n8n-compatible)
	srv.HandleAPI("GET /api/v1/tags", withDeps(deps, handleListTags))
	srv.HandleAPI("POST /api/v1/tags", withDeps(deps, handleCreateTag))
	srv.HandleAPI("GET /api/v1/tags/{id}", withDeps(deps, handleGetTag))
	srv.HandleAPI("PUT /api/v1/tags/{id}", withDeps(deps, handleUpdateTag))
	srv.HandleAPI("DELETE /api/v1/tags/{id}", withDeps(deps, handleDeleteTag))

	// Data Tables (n8n-compatible)
	srv.HandleAPI("GET /api/v1/data-tables", withDeps(deps, handleListDataTables))
	srv.HandleAPI("POST /api/v1/data-tables", withDeps(deps, handleCreateDataTable))
	srv.HandleAPI("GET /api/v1/data-tables/{id}", withDeps(deps, handleGetDataTable))
	srv.HandleAPI("PATCH /api/v1/data-tables/{id}", withDeps(deps, handleUpdateDataTable))
	srv.HandleAPI("DELETE /api/v1/data-tables/{id}", withDeps(deps, handleDeleteDataTable))
	srv.HandleAPI("GET /api/v1/data-tables/{id}/rows", withDeps(deps, handleListRows))
	srv.HandleAPI("POST /api/v1/data-tables/{id}/rows", withDeps(deps, handleInsertRows))
	srv.HandleAPI("PATCH /api/v1/data-tables/{id}/rows/{rowId}", withDeps(deps, handleUpdateRow))
	srv.HandleAPI("DELETE /api/v1/data-tables/{id}/rows/{rowId}", withDeps(deps, handleDeleteRow))

	// Documents
	srv.HandleAPI("GET /api/documents", withDeps(deps, handleListDocuments))
	srv.HandleAPI("GET /api/documents/{path...}", withDeps(deps, handleGetDocument))
	srv.HandleAPI("PUT /api/documents/{path...}", withDeps(deps, handlePutDocument))
	srv.HandleAPI("DELETE /api/documents/{path...}", withDeps(deps, handleDeleteDocument))

	// Chat
	srv.HandleAPI("GET /api/chat/sessions", withDeps(deps, handleListSessions))
	srv.HandleAPI("POST /api/chat/sessions", withDeps(deps, handleCreateSession))
	srv.HandleAPI("DELETE /api/chat/sessions/{id}", withDeps(deps, handleDeleteSession))
	srv.HandleAPI("POST /api/chat", withDeps(deps, handleChat))
	srv.HandleAPI("POST /api/chat/stream", withDeps(deps, handleChatStream))
	srv.HandleAPI("POST /api/chat/stream/confirm", withDeps(deps, handleChatStreamConfirm))
	srv.HandleAPI("GET /api/chat/history", withDeps(deps, handleChatHistory))
	srv.HandleAPI("POST /api/chat/confirm", withDeps(deps, handleChatConfirm))

	// Logs (legacy, maps to executions)
	srv.HandleAPI("GET /api/logs", withDeps(deps, handleListExecutions))
	srv.HandleAPI("GET /api/logs/{run_id}", withDeps(deps, handleGetExecutionLegacy))

	// Settings - Providers
	srv.HandleAPI("GET /api/settings", withDeps(deps, handleGetSettings))
	srv.HandleAPI("PUT /api/settings/active-model", withDeps(deps, handleSetActiveModel))
	srv.HandleAPI("GET /api/settings/providers", withDeps(deps, handleListProviders))
	srv.HandleAPI("POST /api/settings/providers", withDeps(deps, handleAddProvider))
	srv.HandleAPI("DELETE /api/settings/providers/{id}", withDeps(deps, handleDeleteProvider))
	srv.HandleAPI("POST /api/settings/providers/test", withDeps(deps, handleTestProvider))

	// Settings - Integrations
	srv.HandleAPI("PUT /api/settings/tavily-key", withDeps(deps, handleSetTavilyKey))

	// Layout persistence (per-user window layout)
	srv.HandleAPI("GET /api/settings/layout", withDeps(deps, handleGetLayout))
	srv.HandleAPI("PUT /api/settings/layout", withDeps(deps, handlePutLayout))

	// Skills
	srv.HandleAPI("GET /api/skills", withDeps(deps, handleListSkills))
	srv.HandleAPI("GET /api/skills/{id}", withDeps(deps, handleGetSkill))
	srv.HandleAPI("GET /api/skills/{id}/config", withDeps(deps, handleGetSkillConfig))
	srv.HandleAPI("PUT /api/skills/{id}/config", withDeps(deps, handleSaveSkillConfig))

	// Audit Logs
	srv.HandleAPI("GET /api/v1/audit-logs", withDeps(deps, handleListAuditLogs))

	// SSE event stream
	srv.HandleAPI("GET /api/events", withDeps(deps, handleEventStream))
}

type depsHandler func(deps *Deps, w http.ResponseWriter, r *http.Request)

func withDeps(deps *Deps, fn depsHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(deps, w, r)
	}
}

func publishEvent(deps *Deps, typ, title, message, severity string, meta map[string]any) {
	if deps.EventHub != nil {
		deps.EventHub.Publish(events.Event{
			Type:     typ,
			Title:    title,
			Message:  message,
			Severity: severity,
			Meta:     meta,
		})
	}
}

// --- helper to get per-user stores ---

func userRepoDir(dataDir string, r *http.Request) string {
	username := auth.GetUser(r.Context())
	return filepath.Join(dataDir, "users", username, "repo")
}

func userDataDir(dataDir string, r *http.Request) string {
	username := auth.GetUser(r.Context())
	return filepath.Join(dataDir, "users", username, "data")
}

func workflowStore(dataDir string, r *http.Request) *pipeline.WorkflowStore {
	return pipeline.NewWorkflowStore(filepath.Join(userRepoDir(dataDir, r), "pipelines"))
}

func getExecStore(deps *Deps, r *http.Request) pipeline.ExecutionStore {
	if deps.DBCache != nil {
		username := auth.GetUser(r.Context())
		if userDB, err := deps.DBCache.Get(username); err == nil {
			return db.NewExecutionStore(userDB)
		}
	}
	if deps.ExecStore != nil {
		return deps.ExecStore
	}
	return pipeline.NewJSONExecutionStore(filepath.Join(userRepoDir(deps.DataDir, r), "executions"))
}

func variableStore(deps *Deps, r *http.Request) pipeline.VariableRepository {
	if deps.DBCache != nil {
		username := auth.GetUser(r.Context())
		if userDB, err := deps.DBCache.Get(username); err == nil {
			return db.NewVariableStore(userDB)
		}
	}
	return pipeline.NewVariableStore(filepath.Join(userDataDir(deps.DataDir, r)))
}

func tagStore(deps *Deps, r *http.Request) pipeline.TagRepository {
	if deps.DBCache != nil {
		username := auth.GetUser(r.Context())
		if userDB, err := deps.DBCache.Get(username); err == nil {
			return db.NewTagStore(userDB)
		}
	}
	return pipeline.NewTagStore(filepath.Join(userDataDir(deps.DataDir, r)))
}

func dataTableStore(deps *Deps, r *http.Request) pipeline.DataTableRepository {
	if deps.DBCache != nil {
		username := auth.GetUser(r.Context())
		if userDB, err := deps.DBCache.Get(username); err == nil {
			return db.NewDataTableStore(userDB)
		}
	}
	return pipeline.NewDataTableStore(filepath.Join(userDataDir(deps.DataDir, r), "tables"))
}

func credentialStore(dataDir string, r *http.Request) (*credential.Store, error) {
	username := auth.GetUser(r.Context())
	repoDir := filepath.Join(dataDir, "users", username, "repo")
	keyPath := filepath.Join(dataDir, "users", username, "master.key")
	key, err := credential.LoadMasterKey(keyPath)
	if err != nil {
		return nil, err
	}
	git, err := gitstore.Open(repoDir)
	if err != nil {
		return nil, err
	}
	return credential.NewStore(filepath.Join(repoDir, "credentials"), key, git), nil
}

func docStore(dataDir string, r *http.Request) *document.Store {
	repoDir := userRepoDir(dataDir, r)
	git, _ := gitstore.Open(repoDir)
	return document.NewStore(filepath.Join(repoDir, "documents"), git)
}

// --- Health ---

func handleHealth(w http.ResponseWriter, r *http.Request) {
	server.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Workflows ---

func handleListWorkflows(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := workflowStore(deps.DataDir, r)
	workflows, err := store.List()
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if workflows == nil {
		workflows = []*pipeline.Workflow{}
	}
	server.WriteJSON(w, http.StatusOK, workflows)
}

func handleCreateWorkflow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	wf, err := pipeline.ParseWorkflow(body)
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errs := pipeline.ValidateWorkflow(wf); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		server.WriteError(w, http.StatusBadRequest, strings.Join(msgs, "; "))
		return
	}
	store := workflowStore(deps.DataDir, r)
	if err := store.Save(wf); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusCreated, wf)
}

func handleGetWorkflow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := workflowStore(deps.DataDir, r)
	wf, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, wf)
}

func handleUpdateWorkflow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	wf, err := pipeline.ParseWorkflow(body)
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	wf.ID = id
	if errs := pipeline.ValidateWorkflow(wf); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		server.WriteError(w, http.StatusBadRequest, strings.Join(msgs, "; "))
		return
	}
	store := workflowStore(deps.DataDir, r)
	if err := store.Save(wf); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, wf)
}

func handleDeleteWorkflow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := workflowStore(deps.DataDir, r)
	if err := store.Delete(id); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func handleActivateWorkflow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := workflowStore(deps.DataDir, r)
	wf, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	wf.Active = true
	if err := store.Save(wf); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, wf)
}

func handleDeactivateWorkflow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := workflowStore(deps.DataDir, r)
	wf, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	wf.Active = false
	if err := store.Save(wf); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, wf)
}

func handleRunWorkflow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := workflowStore(deps.DataDir, r)
	wf, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	es := getExecStore(deps, r)
	executor := pipeline.NewExecutor(es)
	if deps.Executor != nil {
		executor = deps.Executor
	}

	result, err := executor.Execute(r.Context(), wf)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, result)
}

func handleListWorkflowExecutions(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	es := getExecStore(deps, r)
	execs, err := es.ListExecutions(id, 50)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if execs == nil {
		execs = []*pipeline.Execution{}
	}
	server.WriteJSON(w, http.StatusOK, execs)
}

// --- Executions ---

func handleListExecutions(deps *Deps, w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflowId")
	es := getExecStore(deps, r)
	execs, err := es.ListExecutions(workflowID, 100)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if execs == nil {
		execs = []*pipeline.Execution{}
	}
	server.WriteJSON(w, http.StatusOK, execs)
}

func handleGetExecution(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	es := getExecStore(deps, r)
	exec, err := es.GetExecution(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, exec)
}

func handleGetExecutionLegacy(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("run_id")
	es := getExecStore(deps, r)
	exec, err := es.GetExecution(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, exec)
}

func handleDeleteExecution(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	es := getExecStore(deps, r)
	if err := es.DeleteExecution(id); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func handleClearExecutions(deps *Deps, w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflowId")
	es := getExecStore(deps, r)
	count, err := es.ClearExecutions(workflowID)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]any{"cleared": count})
}

// --- Credentials ---

func handleListCredentials(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store, err := credentialStore(deps.DataDir, r)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	creds, err := store.List()
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if creds == nil {
		creds = []credential.Credential{}
	}
	server.WriteJSON(w, http.StatusOK, creds)
}

func handleCreateCredential(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store, err := credentialStore(deps.DataDir, r)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var cred credential.Credential
	if err := json.NewDecoder(r.Body).Decode(&cred); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if cred.Name == "" {
		server.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	id, err := store.Save(cred)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusCreated, map[string]string{"id": id, "name": cred.Name})
	publishEvent(deps, "credential.created", "Credential created", cred.Name+" was added", "info", nil)
}

func handleGetCredential(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store, err := credentialStore(deps.DataDir, r)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cred, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, "credential not found")
		return
	}
	server.WriteJSON(w, http.StatusOK, cred)
}

func handleUpdateCredential(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store, err := credentialStore(deps.DataDir, r)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	existing, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, "credential not found")
		return
	}
	var update credential.Credential
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	// Apply partial update
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Type != "" {
		existing.Type = update.Type
	}
	if update.Data != nil {
		for k, v := range update.Data {
			existing.Data[k] = v
		}
	}
	if _, err := store.Save(*existing); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"id": id, "name": existing.Name})
	publishEvent(deps, "credential.updated", "Credential updated", existing.Name+" was modified", "info", nil)
}

func handleDeleteCredential(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store, err := credentialStore(deps.DataDir, r)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := store.Delete(id); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
	publishEvent(deps, "credential.deleted", "Credential deleted", id+" was removed", "warning", nil)
}

// --- Variables ---

func handleListVariables(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := variableStore(deps, r)
	vars := store.List()
	if vars == nil {
		vars = []pipeline.Variable{}
	}
	server.WriteJSON(w, http.StatusOK, vars)
}

func handleCreateVariable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := variableStore(deps, r)
	var v pipeline.Variable
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := store.Create(v)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusCreated, created)
}

func handleUpdateVariable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := variableStore(deps, r)
	var v pipeline.Variable
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := store.Update(id, v)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, updated)
}

func handleDeleteVariable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := variableStore(deps, r)
	if err := store.Delete(id); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// --- Tags ---

func handleListTags(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := tagStore(deps, r)
	tags := store.List()
	if tags == nil {
		tags = []pipeline.Tag{}
	}
	server.WriteJSON(w, http.StatusOK, tags)
}

func handleCreateTag(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := tagStore(deps, r)
	var t pipeline.Tag
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := store.Create(t)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusCreated, created)
}

func handleGetTag(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := tagStore(deps, r)
	t, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, t)
}

func handleUpdateTag(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := tagStore(deps, r)
	var t pipeline.Tag
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := store.Update(id, t)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, updated)
}

func handleDeleteTag(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := tagStore(deps, r)
	if err := store.Delete(id); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// --- Data Tables ---

func handleListDataTables(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := dataTableStore(deps, r)
	tables := store.ListTables()
	if tables == nil {
		tables = []pipeline.DataTable{}
	}
	server.WriteJSON(w, http.StatusOK, tables)
}

func handleCreateDataTable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := dataTableStore(deps, r)
	var t pipeline.DataTable
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	created, err := store.CreateTable(t)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusCreated, created)
}

func handleGetDataTable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps, r)
	t, err := store.GetTable(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, t)
}

func handleUpdateDataTable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps, r)
	var t pipeline.DataTable
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := store.UpdateTable(id, t)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, updated)
}

func handleDeleteDataTable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps, r)
	if err := store.DeleteTable(id); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func handleListRows(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps, r)
	rows := store.ListRows(id)
	if rows == nil {
		rows = []pipeline.DataTableRow{}
	}
	server.WriteJSON(w, http.StatusOK, rows)
}

func handleInsertRows(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps, r)
	var rows []pipeline.DataTableRow
	if err := json.NewDecoder(r.Body).Decode(&rows); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	inserted, err := store.InsertRows(id, rows)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusCreated, inserted)
}

func handleUpdateRow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("id")
	rowIDStr := r.PathValue("rowId")
	rowID, err := strconv.Atoi(rowIDStr)
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid row ID")
		return
	}
	store := dataTableStore(deps, r)
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	updated, err := store.UpdateRow(tableID, rowID, data)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, updated)
}

func handleDeleteRow(deps *Deps, w http.ResponseWriter, r *http.Request) {
	tableID := r.PathValue("id")
	rowIDStr := r.PathValue("rowId")
	rowID, err := strconv.Atoi(rowIDStr)
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid row ID")
		return
	}
	store := dataTableStore(deps, r)
	if err := store.DeleteRow(tableID, rowID); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": rowIDStr})
}

// --- Documents ---

func handleListDocuments(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := docStore(deps.DataDir, r)
	docs, err := store.List("")
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if docs == nil {
		docs = []document.Doc{}
	}
	server.WriteJSON(w, http.StatusOK, docs)
}

func handleGetDocument(deps *Deps, w http.ResponseWriter, r *http.Request) {
	docPath := r.PathValue("path")
	store := docStore(deps.DataDir, r)

	// If the path is a directory, return its listing instead.
	if store.IsDir(docPath) {
		docs, err := store.List(docPath)
		if err != nil {
			server.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if docs == nil {
			docs = []document.Doc{}
		}
		server.WriteJSON(w, http.StatusOK, docs)
		return
	}

	doc, err := store.Get(docPath)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, doc)
}

func handlePutDocument(deps *Deps, w http.ResponseWriter, r *http.Request) {
	docPath := r.PathValue("path")
	store := docStore(deps.DataDir, r)
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := store.Put(docPath, payload.Content); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"path": docPath})
}

func handleDeleteDocument(deps *Deps, w http.ResponseWriter, r *http.Request) {
	docPath := r.PathValue("path")
	store := docStore(deps.DataDir, r)
	if err := store.Delete(docPath); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": docPath})
}

// --- Chat ---

func handleListSessions(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ChatService == nil {
		server.WriteJSON(w, http.StatusOK, []any{})
		return
	}
	userID := auth.GetUser(r.Context())
	sessions := deps.ChatService.ListSessions(userID)
	if sessions == nil {
		sessions = []chat.SessionInfo{}
	}
	server.WriteJSON(w, http.StatusOK, sessions)
}

func handleCreateSession(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ChatService == nil {
		server.WriteError(w, http.StatusServiceUnavailable, "chat service not configured")
		return
	}
	userID := auth.GetUser(r.Context())
	info := deps.ChatService.CreateSession(userID)
	server.WriteJSON(w, http.StatusCreated, info)
}

func handleDeleteSession(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ChatService == nil {
		server.WriteError(w, http.StatusServiceUnavailable, "chat service not configured")
		return
	}
	userID := auth.GetUser(r.Context())
	sessionID := r.PathValue("id")
	deps.ChatService.DeleteSession(userID, sessionID)
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": sessionID})
}

func handleChat(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ChatService == nil {
		server.WriteError(w, http.StatusServiceUnavailable, "chat service not configured")
		return
	}
	var payload struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if payload.Message == "" {
		server.WriteError(w, http.StatusBadRequest, "message is required")
		return
	}
	userID := auth.GetUser(r.Context())
	resp, err := deps.ChatService.SendMessage(r.Context(), userID, payload.SessionID, payload.Message)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, resp)
}

func handleChatHistory(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ChatService == nil {
		server.WriteJSON(w, http.StatusOK, []any{})
		return
	}
	userID := auth.GetUser(r.Context())
	sessionID := r.URL.Query().Get("session_id")
	history := deps.ChatService.GetHistory(userID, sessionID)
	server.WriteJSON(w, http.StatusOK, history)
}

func handleChatStream(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ADKRunner == nil {
		server.WriteError(w, http.StatusServiceUnavailable, "agent not configured")
		return
	}
	var payload struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if payload.Message == "" {
		server.WriteError(w, http.StatusBadRequest, "message is required")
		return
	}

	userID := auth.GetUser(r.Context())

	// Ensure ADK session exists
	adkSessionID := payload.SessionID
	if adkSessionID == "" {
		adkSessionID = "default"
	}
	_, err := deps.ADKSessionSvc.Get(r.Context(), &session.GetRequest{
		AppName:   "kuro",
		UserID:    userID,
		SessionID: adkSessionID,
	})
	if err != nil {
		// Create session if it doesn't exist
		_, err = deps.ADKSessionSvc.Create(r.Context(), &session.CreateRequest{
			AppName:   "kuro",
			UserID:    userID,
			SessionID: adkSessionID,
		})
		if err != nil {
			server.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create session: %v", err))
			return
		}
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		server.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	msg := genai.NewContentFromText(payload.Message, genai.RoleUser)

	events := deps.ADKRunner.Run(r.Context(), userID, adkSessionID, msg, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	})

	for ev, err := range events {
		if err != nil {
			writeSSE(w, flusher, map[string]any{"type": "error", "error": err.Error()})
			break
		}
		if ev == nil {
			continue
		}

		sseEvent := eventToSSE(ev)
		if sseEvent != nil {
			writeSSE(w, flusher, sseEvent)
		}
	}

	writeSSE(w, flusher, map[string]any{"type": "done"})
}

func handleChatStreamConfirm(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ADKRunner == nil {
		server.WriteError(w, http.StatusServiceUnavailable, "agent not configured")
		return
	}
	var payload struct {
		SessionID string `json:"session_id"`
		CallID    string `json:"call_id"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if payload.CallID == "" {
		server.WriteError(w, http.StatusBadRequest, "call_id is required")
		return
	}

	userID := auth.GetUser(r.Context())
	adkSessionID := payload.SessionID
	if adkSessionID == "" {
		adkSessionID = "default"
	}

	// Build FunctionResponse for adk_request_confirmation
	content := &genai.Content{
		Role: string(genai.RoleUser),
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				Name: toolconfirmation.FunctionCallName,
				ID:   payload.CallID,
				Response: map[string]any{
					"confirmed": payload.Confirmed,
				},
			},
		}},
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		server.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	events := deps.ADKRunner.Run(r.Context(), userID, adkSessionID, content, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	})

	for ev, err := range events {
		if err != nil {
			writeSSE(w, flusher, map[string]any{"type": "error", "error": err.Error()})
			break
		}
		if ev == nil {
			continue
		}
		sseEvent := eventToSSE(ev)
		if sseEvent != nil {
			writeSSE(w, flusher, sseEvent)
		}
	}

	writeSSE(w, flusher, map[string]any{"type": "done"})
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}

func eventToSSE(ev *session.Event) map[string]any {
	if ev.Content == nil {
		return nil
	}

	for _, part := range ev.Content.Parts {
		if part == nil {
			continue
		}

		if part.Text != "" {
			if ev.Partial {
				return map[string]any{"type": "text_delta", "text": part.Text}
			}
			// Non-partial text is the TurnComplete summary — skip it since
			// the frontend already assembled the text from text_delta events.
			continue
		}

		if part.FunctionCall != nil {
			fc := part.FunctionCall
			if fc.Name == toolconfirmation.FunctionCallName {
				// HITL confirmation request
				originalCall, err := toolconfirmation.OriginalCallFrom(fc)
				if err != nil {
					continue
				}
				hint := ""
				if args, ok := fc.Args["toolConfirmation"]; ok {
					if m, ok := args.(map[string]any); ok {
						hint, _ = m["hint"].(string)
					}
				}
				return map[string]any{
					"type":      "confirm_request",
					"call_id":   fc.ID,
					"tool_name": originalCall.Name,
					"tool_input": originalCall.Args,
					"hint":      hint,
				}
			}
			return map[string]any{
				"type":       "tool_call",
				"tool_name":  fc.Name,
				"tool_input": fc.Args,
				"call_id":    fc.ID,
			}
		}

		if part.FunctionResponse != nil {
			fr := part.FunctionResponse
			return map[string]any{
				"type":        "tool_result",
				"tool_name":   fr.Name,
				"tool_output": fr.Response,
				"call_id":     fr.ID,
			}
		}
	}

	return nil
}

func handleChatConfirm(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.ChatService == nil {
		server.WriteError(w, http.StatusServiceUnavailable, "chat service not configured")
		return
	}
	var payload struct {
		Approve   bool   `json:"approve"`
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	userID := auth.GetUser(r.Context())
	resp, err := deps.ChatService.ConfirmAction(r.Context(), userID, payload.SessionID, payload.Approve)
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, resp)
}

// --- Settings ---

func handleGetSettings(deps *Deps, w http.ResponseWriter, r *http.Request) {
	server.WriteJSON(w, http.StatusOK, deps.SettingsStore.Get())
}

func handleSetActiveModel(deps *Deps, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := deps.SettingsStore.SetActiveModel(payload.ProviderID, payload.Model); err != nil {
		server.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if deps.OnSettingsChanged != nil {
		deps.OnSettingsChanged()
	}
	server.WriteJSON(w, http.StatusOK, deps.SettingsStore.Get())
}

func handleListProviders(deps *Deps, w http.ResponseWriter, r *http.Request) {
	s := deps.SettingsStore.Get()
	if s.Providers == nil {
		s.Providers = []settings.ProviderConfig{}
	}
	server.WriteJSON(w, http.StatusOK, s.Providers)
}

func handleSetTavilyKey(deps *Deps, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := deps.SettingsStore.SetTavilyAPIKey(payload.APIKey); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleAddProvider(deps *Deps, w http.ResponseWriter, r *http.Request) {
	var p settings.ProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if p.ID == "" {
		server.WriteError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := deps.SettingsStore.AddProvider(p); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deps.OnSettingsChanged != nil {
		deps.OnSettingsChanged()
	}
	server.WriteJSON(w, http.StatusCreated, deps.SettingsStore.Get())
}

func handleDeleteProvider(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := deps.SettingsStore.DeleteProvider(id); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if deps.OnSettingsChanged != nil {
		deps.OnSettingsChanged()
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func handleTestProvider(deps *Deps, w http.ResponseWriter, r *http.Request) {
	var p settings.ProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := deps.SettingsStore.TestProvider(r.Context(), p); err != nil {
		server.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Layout Persistence ---

func layoutPath(dataDir string, r *http.Request) string {
	username := auth.GetUser(r.Context())
	return filepath.Join(dataDir, "users", username, "layout.json")
}

func handleGetLayout(deps *Deps, w http.ResponseWriter, r *http.Request) {
	path := layoutPath(deps.DataDir, r)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No saved layout — return empty array
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func handlePutLayout(deps *Deps, w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB max
	if err != nil {
		server.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	// Validate that the body is valid JSON
	if !json.Valid(data) {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	path := layoutPath(deps.DataDir, r)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Skills ---

func handleListSkills(deps *Deps, w http.ResponseWriter, _ *http.Request) {
	if deps.SkillRegistry == nil {
		server.WriteJSON(w, http.StatusOK, []any{})
		return
	}
	type skillInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source,omitempty"`
		Destructive bool   `json:"destructive,omitempty"`
		HasConfig   bool   `json:"has_config,omitempty"`
	}
	skills := deps.SkillRegistry.List()
	result := make([]skillInfo, 0, len(skills))
	for _, s := range skills {
		result = append(result, skillInfo{
			Name:        s.Name,
			Description: s.Description,
			Source:      s.Source,
			Destructive: s.Destructive,
			HasConfig:   len(s.Config) > 0,
		})
	}
	server.WriteJSON(w, http.StatusOK, result)
}

func handleGetSkill(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if deps.SkillRegistry == nil {
		server.WriteError(w, http.StatusNotFound, "skill not found")
		return
	}
	s, ok := deps.SkillRegistry.Get(id)
	if !ok {
		server.WriteError(w, http.StatusNotFound, "skill not found")
		return
	}
	server.WriteJSON(w, http.StatusOK, s)
}

func handleGetSkillConfig(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if deps.SkillConfigStore == nil {
		server.WriteJSON(w, http.StatusOK, map[string]any{})
		return
	}
	cfg, err := deps.SkillConfigStore.GetConfig(id)
	if err != nil {
		// No config yet — return empty
		server.WriteJSON(w, http.StatusOK, map[string]any{})
		return
	}
	// Mask sensitive values (show last 4 chars)
	masked := make(map[string]string, len(cfg))
	for k, v := range cfg {
		if len(v) > 4 {
			masked[k] = "***" + v[len(v)-4:]
		} else if v != "" {
			masked[k] = "***"
		}
	}
	server.WriteJSON(w, http.StatusOK, masked)
}

func handleSaveSkillConfig(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if deps.SkillConfigStore == nil {
		server.WriteError(w, http.StatusInternalServerError, "skill config store not available")
		return
	}
	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		server.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Skip masked values — don't overwrite real keys with placeholder
	existing, _ := deps.SkillConfigStore.GetConfig(id)
	for k, v := range data {
		if strings.HasPrefix(v, "***") && existing != nil {
			if orig, ok := existing[k]; ok {
				data[k] = orig
			}
		}
	}
	if err := deps.SkillConfigStore.SaveConfig(id, data); err != nil {
		server.WriteError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Audit Logs ---

func handleListAuditLogs(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.DBCache == nil {
		server.WriteJSON(w, http.StatusOK, []any{})
		return
	}
	username := auth.GetUser(r.Context())
	userDB, err := deps.DBCache.Get(username)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := r.URL.Query()
	filter := audit.QueryFilter{
		Type:      audit.EventType(q.Get("type")),
		TraceID:   q.Get("trace_id"),
		SessionID: q.Get("session_id"),
		UserID:    q.Get("user_id"),
		Limit:     100,
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	store := db.NewAuditStore(userDB)
	entries, total, err := store.Query(filter)
	if err != nil {
		server.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	server.WriteJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   total,
	})
}

// --- SSE Event Stream ---

func handleEventStream(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.EventHub == nil {
		http.Error(w, "event stream not available", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := deps.EventHub.Subscribe()
	defer cancel()

	// Send initial heartbeat
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
