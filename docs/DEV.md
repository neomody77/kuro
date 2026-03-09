# DEV.md — Development State Snapshot

> Last updated: 2026-03-08

## Build

```bash
# Frontend (generates cmd/kuro/ui/ for go:embed)
cd ui && npm install && npm run build && cd ..

# Copy UI assets for embedding
rm -rf cmd/kuro/ui && cp -r ui/dist cmd/kuro/ui

# Backend
go build -o kuro ./cmd/kuro/
go build -o kuro-cli ./cmd/kuro-cli/

# Lint & test
go vet ./...
go test ./...
```

## Recently Completed

### SQLite Storage Layer + Audit Logging

Per-user SQLite databases for runtime state, replacing JSON file stores where appropriate.

**Architecture:**
- `internal/db/db.go` — `DB` wrapper, `UserDBCache` (per-user lazy open), WAL mode
- `internal/db/migrations.go` — Schema migrations with version tracking (8 tables)
- `internal/db/execution_store.go` — `pipeline.ExecutionStore` implementation
- `internal/db/variable_store.go` — `pipeline.VariableRepository` implementation
- `internal/db/tag_store.go` — `pipeline.TagRepository` implementation
- `internal/db/data_table_store.go` — `pipeline.DataTableRepository` implementation
- `internal/db/chat_store.go` — Chat session + message persistence
- `internal/db/audit_store.go` — `audit.Store` implementation
- `internal/audit/audit.go` — Structured audit logging with trace ID context propagation

**Key design decisions:**
- `modernc.org/sqlite` — pure Go, no CGO, avoids cross-compile issues
- Per-user isolation: `users/{username}/data/kuro.db`
- Optional: all API handlers check `DBCache != nil`, fall back to JSON file stores
- Stores that stay file-based: documents (markdown+git), credentials (encrypted YAML+git), settings (YAML), workflow definitions (JSON+git), ADK sessions
- Repository interfaces in `internal/pipeline/store.go`: `VariableRepository`, `TagRepository`, `DataTableRepository`, `ExecutionStore` (with `ClearExecutions`)

**Audit logging:**
- 5 event types: `user_action`, `ai_response`, `skill_exec`, `pipeline_node`, `system`
- `audit.Logger` with convenience methods for each type
- Trace ID propagation via `context.WithValue` / `audit.WithTraceID(ctx, id)`
- `GET /api/v1/audit-logs` with filtering by type, trace_id, session_id, user_id, limit/offset

### SSE Event Stream + Notification Center

Real-time server-sent events for pipeline completions, credential changes, etc.

**Architecture:**
- `internal/events/hub.go` — Pub/sub `Hub` with subscriber channels, 50-event ring buffer
- `GET /api/events` — SSE endpoint, streams JSON events to all connected clients
- `ui/src/hooks/useEventStream.ts` — React hook wrapping `EventSource`
- `ui/src/components/desktop/NotificationCenter.tsx` — Live notification UI

**Events emitted:**
- Pipeline completion/failure/cancellation (`Executor.SetOnComplete` callback)
- Credential CRUD (created/updated/deleted)
- Late joiners receive recent events from ring buffer

### Credential Caller Isolation

Credentials are redacted in chat context, full values only available to pipeline executor.

**Architecture:**
- `internal/adk/skill_tool.go` — `redactCredentialData()` masks secret values before they reach the LLM (ADK streaming path)
- `internal/chat/chat.go` — `redactCredentialResult()` masks secrets in legacy chat path
- Pipeline path unaffected — `CredentialResolver.Resolve()` returns full decrypted values directly
- Redaction format: `sk****ey` (first 2 + `****` + last 2 chars, or `****` if ≤4 chars)

### Desktop Window Manager

Dual-view UI: traditional sidebar layout (`/app/*`) + desktop-style window manager (`/desktop/*`).

**Components:**
- `ui/src/components/desktop/Window.tsx` — Draggable/resizable window (react-rnd)
- `ui/src/components/desktop/Desktop.tsx` — Container with wallpaper, icon grid, lazy-load pages
- `ui/src/components/desktop/Taskbar.tsx` — Bottom bar with open windows, system tray, clock
- `ui/src/components/desktop/AppLauncher.tsx` — 3x3 app grid + view switch
- `ui/src/components/desktop/NotificationCenter.tsx` — Live SSE notifications
- `ui/src/lib/windowStore.ts` — Layout persistence (localStorage)
- `ui/src/lib/navConfig.ts` — Shared page definitions for both views

### ADK Agent Loop (Phase 0-4)

Multi-turn agent loop via Google ADK Go v0.6.0. Replaces single-round skill execution.

**Architecture:**
- `internal/adk/openai_llm.go` — OpenAI-compatible LLM adapter (`model.LLM` interface)
- `internal/adk/skill_tool.go` — Skill → ADK `tool.Tool` adapter
- `internal/adk/runner.go` — Agent + Runner factory functions
- `POST /api/chat/stream` — SSE endpoint using ADK Runner
- `ui/src/hooks/useChatStream.ts` — React SSE consumer hook

**Capabilities gained:**
- Multi-turn tool calling (LLM ↔ tool automatic loop)
- Native function calling (no more markdown parsing)
- SSE streaming output with real-time text deltas
- OpenAI-compatible + Gemini dual provider support
- Collapsible tool call cards (name + status collapsed, JSON input/output expanded)
- Streaming dots indicator while waiting for response

**Key implementation details:**
- `processSSE()` accumulates streamed text and emits a `TurnComplete` response with full text, so ADK session stores the complete assistant reply for multi-turn context
- `eventToSSE()` skips non-partial text events to avoid duplicating content already sent via deltas
- `WriteTimeout: 0` on HTTP server to prevent SSE streams from being cut off

### HITL (Human-in-the-Loop) Confirmation
- Destructive skills (shell, credential:delete, document:delete, file:write/delete, pipeline:delete) require user approval
- Uses ADK's native `ctx.ToolConfirmation()` / `ctx.RequestConfirmation()` in `skillToolWrapper.Run()`
- Backend detects `adk_request_confirmation` FunctionCall events → emits `confirm_request` SSE event
- `POST /api/chat/stream/confirm` sends user's decision back as `FunctionResponse`
- Frontend: tool call card shows "Needs Approval" badge + Approve/Reject buttons
- Rejection returns "rejected by user" to LLM; approval executes the action

### Other Completed Features

- **File-Based ADK Session Persistence** — `FileSessionService` wraps `InMemoryService` + file persistence, atomic writes, append-only event log
- **Credential ID-Based Storage** — All CRUD by `id`, files named `{id}.yaml`, auto-generated `cred_*` IDs
- **Settings / Provider Configuration** — YAML persistence, no env var fallback, hot-swap + re-create ADK runner
- **n8n-Compatible Workflow Engine** — DAG executor with topological sort, 1:N / N:1, IMAP IDLE, cron triggers
- **Unified Skill Pattern** — Single name + `action` param, destructive check via `"skill:action"` format
- **UI** — Chat, Pipelines, Documents, Vault, Skills, Settings, Logs pages; IME handling; collapsible sidebar

## Architecture

### Key Paths
- `cmd/kuro/main.go` — Entry point, wiring, ADK runner init, SQLite + event hub setup
- `internal/adk/` — ADK integration (OpenAI LLM adapter, Skill→Tool adapter, Runner, credential redaction)
- `internal/api/` — HTTP API routes, SSE streaming, audit log + event stream endpoints
- `internal/audit/` — Structured audit logging, trace ID context propagation
- `internal/chat/` — Chat service, system prompt, session persistence (JSONL), credential redaction
- `internal/credential/` — AES-256-GCM encrypted vault (git-versioned)
- `internal/db/` — Per-user SQLite stores (executions, variables, tags, data tables, chat, audit)
- `internal/document/` — Git-versioned markdown store
- `internal/events/` — Pub/sub event hub for SSE streaming
- `internal/pipeline/` — Workflow store, parser, executor (with OnComplete callback), scheduler
- `internal/provider/` — OpenAI-compatible LLM provider
- `internal/settings/` — YAML settings persistence
- `internal/skill/` — Skill registry + built-in handlers
- `ui/src/components/desktop/` — Desktop window manager (Window, Desktop, Taskbar, AppLauncher, NotificationCenter)
- `ui/src/hooks/` — React hooks (useChatStream, useEventStream, useTheme)
- `ui/src/lib/` — Shared utils (navConfig, windowStore)
- `ui/src/pages/` — React pages

### Data Layout
```
~/.kuro/
  settings.yaml              # Provider config, active model
  users/default/
    master.key
    data/
      kuro.db                # SQLite (executions, variables, tags, data tables, chat, audit)
    repo/
      credentials/*.yaml     # Encrypted credential files (by ID)
      documents/*.md
      pipelines/*.json       # n8n-format workflow files
      executions/            # Legacy JSON execution results (fallback)
    chat/
      sessions.jsonl
      {sessionID}.jsonl
    adk-sessions/
      {sessionID}/
        session.json         # Session metadata + state
        events.jsonl         # Append-only event log
```

### API Routes

```
# Health
GET    /api/health

# Workflows (n8n-compatible)
GET    /api/v1/workflows
POST   /api/v1/workflows
GET    /api/v1/workflows/{id}
PUT    /api/v1/workflows/{id}
DELETE /api/v1/workflows/{id}
POST   /api/v1/workflows/{id}/activate
POST   /api/v1/workflows/{id}/deactivate

# Executions
GET    /api/v1/executions
GET    /api/v1/executions/{id}
DELETE /api/v1/executions/{id}
POST   /api/v1/executions/clear

# Credentials
GET    /api/v1/credentials
POST   /api/v1/credentials
GET    /api/v1/credentials/{id}
PATCH  /api/v1/credentials/{id}
DELETE /api/v1/credentials/{id}

# Variables
GET    /api/v1/variables
POST   /api/v1/variables
PUT    /api/v1/variables/{id}
DELETE /api/v1/variables/{id}

# Tags
GET    /api/v1/tags
POST   /api/v1/tags
GET    /api/v1/tags/{id}
PUT    /api/v1/tags/{id}
DELETE /api/v1/tags/{id}

# Data Tables
GET    /api/v1/data-tables
POST   /api/v1/data-tables
GET    /api/v1/data-tables/{id}
PATCH  /api/v1/data-tables/{id}
DELETE /api/v1/data-tables/{id}
GET    /api/v1/data-tables/{id}/rows
POST   /api/v1/data-tables/{id}/rows
PATCH  /api/v1/data-tables/{id}/rows/{rowId}
DELETE /api/v1/data-tables/{id}/rows/{rowId}

# Documents
GET    /api/documents
GET    /api/documents/{path...}
PUT    /api/documents/{path...}
DELETE /api/documents/{path...}

# Chat
GET    /api/chat/sessions
POST   /api/chat/sessions
DELETE /api/chat/sessions/{id}
POST   /api/chat                    # Legacy (markdown skill parsing)
POST   /api/chat/stream             # ADK SSE streaming
POST   /api/chat/stream/confirm     # HITL confirmation
GET    /api/chat/history
POST   /api/chat/confirm            # Legacy confirmation

# Settings
GET    /api/settings
PUT    /api/settings/active-model
GET    /api/settings/providers
POST   /api/settings/providers
DELETE /api/settings/providers/{id}
POST   /api/settings/providers/test

# Skills
GET    /api/skills
GET    /api/skills/{id}

# Audit Logs
GET    /api/v1/audit-logs

# SSE Event Stream
GET    /api/events

# Legacy routes (redirect to v1)
GET    /api/pipelines → /api/v1/workflows
...
GET    /api/logs → /api/v1/executions
```

## Known Issues
- Plugin architecture for skills not yet implemented
- 3 skill tests fail in sandbox (sh not in PATH) — not a real issue

## Future
- Multi-agent orchestration (ADK supports but not yet used)
- Web Search skill (Brave Search / SerpAPI / Tavily)
- Docker build
