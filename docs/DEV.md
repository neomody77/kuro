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
- In-memory ADK sessions (not persisted across restarts)

### Chat UX Improvements
- Session reuse: "+ New Chat" reuses existing empty "New Chat" sessions instead of creating duplicates
- Session title: only updated on first message (when title is "New Chat")
- Streaming indicator: pulsing dots always visible while `msg.streaming` is true

### Credential ID-Based Storage
- All credential operations use `id` as primary key, `name` is display only
- Files named `{id}.yaml`, all CRUD by ID
- `Save` returns `(string, error)` — auto-generates `cred_*` ID if empty
- `CredentialResolver` interface: single `Resolve(id string)` method

### Settings / Provider Configuration
- Provider config persisted in `~/.kuro/settings.yaml`
- No env var fallback — must configure via Settings UI
- `onSettingsChanged` callback hot-swaps provider + re-creates ADK runner at runtime
- API keys masked in GET response, preserved on masked-key updates

### n8n-Compatible Workflow Engine
- Types: `Workflow{id, name, nodes[], connections{}, settings{}}`
- DAG executor with topological sort, 1:N / N:1 connections
- IMAP IDLE trigger (RFC 2177), cron trigger
- Pipeline CRUD via skill + API + UI

### Unified Skill Pattern
- All skills: single name + `action` param (e.g., `credential` with `action: list/get/save/delete`)
- Destructive check: `"skill:action"` format (e.g., `credential:delete`)
- Built-in: credential, document, pipeline, shell, email, http, file, transform, template, ai

### UI
- React pages: Chat, Pipelines, Documents, Vault, Skills, Settings, Logs
- Settings: provider CRUD with presets (OpenAI, OpenRouter, Anthropic, Custom)
- Markdown rendering, one-click copy, run history clear
- IME composition handling for CJK input

## Architecture

### Key Paths
- `cmd/kuro/main.go` — entry point, wiring, ADK runner init
- `internal/adk/` — ADK integration (OpenAI LLM adapter, Skill→Tool adapter, Runner)
- `internal/chat/` — chat service, system prompt, session persistence (JSONL)
- `internal/skill/` — skill registry + built-in handlers
- `internal/pipeline/` — workflow store, parser, executor, scheduler
- `internal/credential/` — AES-256-GCM encrypted vault (git-versioned)
- `internal/document/` — git-versioned markdown store
- `internal/provider/` — OpenAI-compatible LLM provider (used by Settings test)
- `internal/settings/` — YAML settings persistence
- `internal/api/` — HTTP API routes + SSE streaming
- `ui/src/pages/` — React pages
- `ui/src/hooks/` — React hooks (useChatStream, useTheme)

### Data Layout
```
~/.kuro/
  settings.yaml              # Provider config, active model
  users/default/
    master.key
    repo/
      credentials/*.yaml     # Encrypted credential files (by ID)
      documents/*.md
      pipelines/*.json       # n8n-format workflow files
      executions/             # Workflow execution results
    chat/
      sessions.jsonl
      {sessionID}.jsonl
    adk-sessions/
      {sessionID}/
        session.json           # Session metadata + state
        events.jsonl           # Append-only event log
```

### File-Based ADK Session Persistence
- `internal/adk/session_store.go` — `FileSessionService` wraps `InMemoryService` + file persistence
- Sessions stored as JSON: `{dataDir}/users/{userID}/adk-sessions/{sessionID}/session.json`
- Events stored as append-only JSONL: `events.jsonl`
- On startup: scans disk, replays events into in-memory service
- Atomic writes for session.json (write-to-tmp + rename)
- Graceful degradation: falls back to in-memory if file store fails

### HITL (Human-in-the-Loop) Confirmation
- Destructive skills (shell, credential:delete, document:delete, file:write/delete, pipeline:delete) require user approval
- Uses ADK's native `ctx.ToolConfirmation()` / `ctx.RequestConfirmation()` in `skillToolWrapper.Run()`
- Backend detects `adk_request_confirmation` FunctionCall events → emits `confirm_request` SSE event
- `POST /api/chat/stream/confirm` sends user's decision back as `FunctionResponse`
- Frontend: tool call card shows "Needs Approval" badge + Approve/Reject buttons
- Rejection returns "rejected by user" to LLM; approval executes the action

## Known Issues
- Plugin architecture for skills not yet implemented

## Future
- Multi-agent orchestration (ADK supports but not yet used)
