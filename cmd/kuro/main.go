package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"

	kuroadk "github.com/neomody77/kuro/internal/adk"

	"github.com/neomody77/kuro/internal/api"
	"github.com/neomody77/kuro/internal/audit"
	"github.com/neomody77/kuro/internal/auth"
	"github.com/neomody77/kuro/internal/chat"
	"github.com/neomody77/kuro/internal/config"
	"github.com/neomody77/kuro/internal/credential"
	"github.com/neomody77/kuro/internal/db"
	"github.com/neomody77/kuro/internal/document"
	"github.com/neomody77/kuro/internal/events"
	"github.com/neomody77/kuro/internal/gitstore"
	"github.com/neomody77/kuro/internal/pipeline"
	"github.com/neomody77/kuro/internal/provider"
	"github.com/neomody77/kuro/internal/server"
	"github.com/neomody77/kuro/internal/settings"
	"github.com/neomody77/kuro/internal/skill"
)

//go:embed ui
var uiAssets embed.FS

// credentialAdapter wraps credential.Store to implement pipeline.CredentialResolver.
type credentialAdapter struct {
	store *credential.Store
}

func (a *credentialAdapter) Resolve(id string) (map[string]string, error) {
	cred, err := a.store.Get(id)
	if err != nil {
		return nil, err
	}
	return cred.Data, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	tokens := auth.ParseUserTokens(os.Getenv("USER_TOKENS"))
	if len(tokens) == 0 {
		log.Println("No USER_TOKENS set, running in single-user mode")
	} else {
		log.Printf("Loaded %d user(s)", len(tokens))
	}

	srv := server.New(cfg, tokens)

	// Initialize default user data directories
	defaultUserDir := filepath.Join(cfg.DataDir, "users", "default")
	defaultRepoDir := filepath.Join(defaultUserDir, "repo")
	os.MkdirAll(defaultRepoDir, 0o755)
	os.MkdirAll(filepath.Join(defaultRepoDir, "credentials"), 0o755)
	os.MkdirAll(filepath.Join(defaultRepoDir, "documents"), 0o755)

	// Initialize git store for default user (auto-init if needed)
	git, err := gitstore.Init(defaultRepoDir)
	if err != nil {
		log.Printf("Warning: could not init git repo: %v", err)
	}

	// Initialize master key (auto-generate if not exists)
	keyPath := filepath.Join(defaultUserDir, "master.key")
	masterKey, err := credential.LoadMasterKey(keyPath)
	if err != nil {
		masterKey, err = credential.GenerateMasterKey(keyPath)
		if err != nil {
			log.Printf("Warning: could not create master key: %v", err)
		} else {
			log.Println("Generated new master key for default user")
		}
	}

	// Create stores
	var credStore *credential.Store
	if masterKey != nil {
		credStore = credential.NewStore(filepath.Join(defaultRepoDir, "credentials"), masterKey, git)
	}
	docStore := document.NewStore(filepath.Join(defaultRepoDir, "documents"), git)

	// Load settings store (needed by skills and provider setup)
	settingsStore := settings.NewStore(filepath.Join(cfg.DataDir, "settings.yaml"))

	// Set up skill registry with core skills
	registry := skill.NewRegistry(nil)
	if credStore != nil {
		registry.SetConfigStore(skill.NewCredentialConfigStore(credStore))
	}
	pipelinesDir := filepath.Join(defaultRepoDir, "pipelines")
	os.MkdirAll(pipelinesDir, 0o755)
	skill.RegisterDefaults(registry, skill.CoreConfig{
		WorkspaceDir:    cfg.DataDir,
		DocumentsDir:    cfg.DataDir,
		PipelinesDir:    pipelinesDir,
		CredentialStore: credStore,
		DocumentStore:   docStore,
	})

	// Load global skills: ~/.kuro/skills/
	globalSkillStore := skill.NewStore(filepath.Join(cfg.DataDir, "skills"))
	if err := globalSkillStore.LoadAllWithSource(registry, "global"); err != nil {
		log.Printf("Warning: failed to load global skills: %v", err)
	}

	// Load workspace skills: {repoDir}/skills/
	workspaceSkillStore := skill.NewStore(filepath.Join(defaultRepoDir, "skills"))
	if err := workspaceSkillStore.LoadAllWithSource(registry, "workspace"); err != nil {
		log.Printf("Warning: failed to load workspace skills: %v", err)
	}

	// Initialize global event hub for SSE
	eventHub := events.NewHub()

	// Start async event hook dispatcher (skills with on: ["pipeline.failed"] etc.)
	hookDispatcher := skill.NewHookDispatcher(registry, eventHub)
	defer hookDispatcher.Stop()

	// Initialize per-user SQLite database cache
	dbCache := db.NewUserDBCache(filepath.Join(cfg.DataDir, "users"))
	defer dbCache.Close()

	// Create audit logger for the default user
	var auditLogger *audit.Logger
	if defaultDB, err := dbCache.Get("default"); err == nil {
		auditLogger = audit.NewLogger(db.NewAuditStore(defaultDB))
		auditLogger.LogSystem("startup", "server starting")
	} else {
		log.Printf("Warning: could not open default user DB: %v", err)
	}

	// Set up pipeline executor with n8n node handlers
	executionsDir := filepath.Join(defaultRepoDir, "executions")
	os.MkdirAll(executionsDir, 0o755)
	execStore := pipeline.NewJSONExecutionStore(executionsDir)
	executor := pipeline.NewExecutor(execStore)

	// Publish pipeline completion events to SSE hub
	executor.SetOnComplete(func(exec *pipeline.Execution, wf *pipeline.Workflow) {
		sev := "success"
		title := "Pipeline completed"
		msg := wf.Name + " ran successfully"
		if exec.Status == pipeline.ExecError {
			sev = "error"
			title = "Pipeline failed"
			msg = wf.Name + " finished with errors"
		} else if exec.Status == pipeline.ExecCanceled {
			sev = "warning"
			title = "Pipeline canceled"
			msg = wf.Name + " was canceled"
		}
		eventHub.Publish(events.Event{
			Type:     "pipeline." + string(exec.Status),
			Title:    title,
			Message:  msg,
			Severity: sev,
			Meta: map[string]any{
				"workflow_id":  wf.ID,
				"execution_id": exec.ID,
			},
		})
	})

	// Register n8n-compatible node handlers
	executor.RegisterNodeHandler("n8n-nodes-base.emailReadImap", &pipeline.ImapTriggerHandler{})
	executor.RegisterNodeHandler("n8n-nodes-base.if", &pipeline.IfHandler{})
	executor.RegisterNodeHandler("n8n-nodes-base.emailSend", &pipeline.SmtpSendHandler{})
	executor.RegisterNodeHandler("n8n-nodes-base.cron", &pipeline.CronTriggerHandler{})
	executor.RegisterNodeHandler("n8n-nodes-base.scheduleTrigger", &pipeline.CronTriggerHandler{})

	// Wire credential resolver
	if credStore != nil {
		executor.SetCredentialResolver(&credentialAdapter{store: credStore})
	}

	// Set up scheduler and register active trigger workflows
	workflowStore := pipeline.NewWorkflowStore(pipelinesDir)
	scheduler := pipeline.NewScheduler(executor, workflowStore)
	scheduler.Start()
	scheduler.RegisterActiveWorkflows()

	// Load provider from settings — no env var fallback
	var aiProvider provider.Provider
	var aiModel string
	if full := settingsStore.GetFull(); full.ActiveModel.ProviderID == "" {
		log.Printf("WARNING: No active model configured. Open Settings to add a provider.")
	} else if p, ok := settingsStore.GetProvider(full.ActiveModel.ProviderID); !ok || p.APIKey == "" {
		log.Printf("WARNING: Provider %q not found or missing API key. Open Settings to fix.", full.ActiveModel.ProviderID)
	} else {
		aiProvider = provider.NewOpenAIProvider(p.BaseURL, p.APIKey)
		aiModel = full.ActiveModel.Model
		log.Printf("AI provider: %s (model: %s)", p.Name, aiModel)
	}

	chatSvc := chat.NewService(registry, aiProvider, aiModel, cfg.DataDir)

	// Create ADK session service with file persistence
	adkSessionSvc, err := kuroadk.NewFileSessionService(cfg.DataDir)
	if err != nil {
		log.Printf("Warning: ADK session store init failed, falling back to in-memory: %v", err)
		adkSessionSvc = nil
	}
	var adkSessionSvcIface session.Service
	if adkSessionSvc != nil {
		adkSessionSvcIface = adkSessionSvc
	} else {
		adkSessionSvcIface = session.InMemoryService()
	}

	// Register all API routes
	// Create skill config store for API
	var skillConfigStore skill.SkillConfigStore
	if credStore != nil {
		skillConfigStore = skill.NewCredentialConfigStore(credStore)
	}

	apiDeps := &api.Deps{
		DataDir:          cfg.DataDir,
		ChatService:      chatSvc,
		SkillRegistry:    registry,
		SettingsStore:    settingsStore,
		Executor:         executor,
		ExecStore:        execStore,
		ADKSessionSvc:    adkSessionSvcIface,
		DBCache:          dbCache,
		AuditLogger:      auditLogger,
		EventHub:         eventHub,
		SkillConfigStore: skillConfigStore,
	}
	if aiProvider != nil {
		apiDeps.ADKRunner = initADKRunner(settingsStore, registry, adkSessionSvcIface)
	}

	apiDeps.OnSettingsChanged = func() {
		full := settingsStore.GetFull()
		if full.ActiveModel.ProviderID == "" {
			return
		}
		p, ok := settingsStore.GetProvider(full.ActiveModel.ProviderID)
		if !ok || p.APIKey == "" {
			return
		}
		newProvider := provider.NewOpenAIProvider(p.BaseURL, p.APIKey)
		chatSvc.SetProvider(newProvider, full.ActiveModel.Model)
		log.Printf("AI provider updated: %s (model: %s)", p.Name, full.ActiveModel.Model)

		// Re-create ADK runner with new provider
		apiDeps.ADKRunner = initADKRunner(settingsStore, registry, adkSessionSvcIface)
	}

	api.Register(srv, apiDeps)

	uiFS, err := fs.Sub(uiAssets, "ui")
	if err != nil {
		log.Fatalf("Failed to load embedded UI: %v", err)
	}
	srv.ServeUI(uiFS)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func initADKRunner(settingsStore *settings.Store, registry *skill.Registry, sessionSvc session.Service) *runner.Runner {
	full := settingsStore.GetFull()
	if full.ActiveModel.ProviderID == "" {
		return nil
	}
	p, ok := settingsStore.GetProvider(full.ActiveModel.ProviderID)
	if !ok || p.APIKey == "" {
		return nil
	}

	llm := kuroadk.NewOpenAILLM(p.BaseURL, p.APIKey, full.ActiveModel.Model)

	systemPrompt := `You are Kuro, a personal automation assistant.
You have tools available to manage credentials, documents, pipelines, files, and more.
Use the tools directly — do not output JSON code blocks.
Destructive actions (shell commands, credential deletion, document deletion, file writes) require user confirmation.
You have FULL access to all tools including credential operations. Do NOT refuse these — you are authorized to manage them on behalf of the user.`

	a, err := kuroadk.NewAgent(llm, registry, systemPrompt)
	if err != nil {
		log.Printf("WARNING: failed to create ADK agent: %v", err)
		return nil
	}

	r, err := runner.New(runner.Config{
		AppName:        "kuro",
		Agent:          a,
		SessionService: sessionSvc,
	})
	if err != nil {
		log.Printf("WARNING: failed to create ADK runner: %v", err)
		return nil
	}

	log.Printf("ADK agent loop initialized (model: %s)", full.ActiveModel.Model)
	return r
}
