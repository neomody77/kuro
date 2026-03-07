// Package api implements the REST API handlers for Kuro.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neomody77/kuro/internal/auth"
	"github.com/neomody77/kuro/internal/chat"
	"github.com/neomody77/kuro/internal/credential"
	"github.com/neomody77/kuro/internal/document"
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

	// Skills
	srv.HandleAPI("GET /api/skills", withDeps(deps, handleListSkills))
	srv.HandleAPI("GET /api/skills/{id}", withDeps(deps, handleGetSkill))
}

type depsHandler func(deps *Deps, w http.ResponseWriter, r *http.Request)

func withDeps(deps *Deps, fn depsHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(deps, w, r)
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

func getExecStore(deps *Deps, r *http.Request) *pipeline.JSONExecutionStore {
	if deps.ExecStore != nil {
		return deps.ExecStore
	}
	return pipeline.NewJSONExecutionStore(filepath.Join(userRepoDir(deps.DataDir, r), "executions"))
}

func variableStore(dataDir string, r *http.Request) *pipeline.VariableStore {
	return pipeline.NewVariableStore(filepath.Join(userDataDir(dataDir, r)))
}

func tagStore(dataDir string, r *http.Request) *pipeline.TagStore {
	return pipeline.NewTagStore(filepath.Join(userDataDir(dataDir, r)))
}

func dataTableStore(dataDir string, r *http.Request) *pipeline.DataTableStore {
	return pipeline.NewDataTableStore(filepath.Join(userDataDir(dataDir, r), "tables"))
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
}

// --- Variables ---

func handleListVariables(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := variableStore(deps.DataDir, r)
	vars := store.List()
	if vars == nil {
		vars = []pipeline.Variable{}
	}
	server.WriteJSON(w, http.StatusOK, vars)
}

func handleCreateVariable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := variableStore(deps.DataDir, r)
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
	store := variableStore(deps.DataDir, r)
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
	store := variableStore(deps.DataDir, r)
	if err := store.Delete(id); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// --- Tags ---

func handleListTags(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := tagStore(deps.DataDir, r)
	tags := store.List()
	if tags == nil {
		tags = []pipeline.Tag{}
	}
	server.WriteJSON(w, http.StatusOK, tags)
}

func handleCreateTag(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := tagStore(deps.DataDir, r)
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
	store := tagStore(deps.DataDir, r)
	t, err := store.Get(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, t)
}

func handleUpdateTag(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := tagStore(deps.DataDir, r)
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
	store := tagStore(deps.DataDir, r)
	if err := store.Delete(id); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// --- Data Tables ---

func handleListDataTables(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := dataTableStore(deps.DataDir, r)
	tables := store.ListTables()
	if tables == nil {
		tables = []pipeline.DataTable{}
	}
	server.WriteJSON(w, http.StatusOK, tables)
}

func handleCreateDataTable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	store := dataTableStore(deps.DataDir, r)
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
	store := dataTableStore(deps.DataDir, r)
	t, err := store.GetTable(id)
	if err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, t)
}

func handleUpdateDataTable(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps.DataDir, r)
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
	store := dataTableStore(deps.DataDir, r)
	if err := store.DeleteTable(id); err != nil {
		server.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	server.WriteJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func handleListRows(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps.DataDir, r)
	rows := store.ListRows(id)
	if rows == nil {
		rows = []pipeline.DataTableRow{}
	}
	server.WriteJSON(w, http.StatusOK, rows)
}

func handleInsertRows(deps *Deps, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	store := dataTableStore(deps.DataDir, r)
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
	store := dataTableStore(deps.DataDir, r)
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
	store := dataTableStore(deps.DataDir, r)
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

// --- Skills ---

func handleListSkills(deps *Deps, w http.ResponseWriter, r *http.Request) {
	if deps.SkillRegistry == nil {
		server.WriteJSON(w, http.StatusOK, []any{})
		return
	}
	type skillInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	skills := deps.SkillRegistry.List()
	result := make([]skillInfo, 0, len(skills))
	for _, s := range skills {
		result = append(result, skillInfo{Name: s.Name, Description: s.Description})
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
