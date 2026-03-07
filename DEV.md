# DEV.md — Development State Snapshot

> Last updated: 2026-03-08

## Build

```bash
# Frontend (generates cmd/kuro/ui/ for go:embed)
cd ui && npm install && npm run build && cd ..

# Backend
go build -o kuro ./cmd/kuro/
go build -o kuro-cli ./cmd/kuro-cli/

# Lint & test
go vet ./...
go test ./...
```

## Recently Completed

### Credential ID-Based Storage
- All credential operations use `id` as primary key, `name` is display only
- Files named `{id}.yaml`, all CRUD by ID
- `Save` returns `(string, error)` — auto-generates `cred_*` ID if empty
- `CredentialResolver` interface: single `Resolve(id string)` method
- No name-based fallback anywhere — AI uses `list` to find ID by name
- Removed all migration/compat code (`Migrate()`, `GetByName()`, `MigrateCredentialRefs()`)

### Settings / Provider Configuration
- Provider config persisted in `~/.kuro/settings.yaml`
- No env var fallback — must configure via Settings UI
- Server logs warning and starts without AI if no provider configured
- `onSettingsChanged` callback hot-swaps provider at runtime
- Settings API: GET/PUT active model, POST/DELETE/test providers
- API keys masked in GET response, preserved on masked-key updates

### Chat Input
- `react-textarea-autosize` replaces manual resize hack
- `minRows=1`, `maxRows=6`, natural container growth

### Chat Session Persistence (JSONL)
- Session index: `{dataDir}/users/{userID}/chat/sessions.jsonl`
- Message log: `{dataDir}/users/{userID}/chat/{sessionID}.jsonl` (append-only)
- Auto-loads from disk, user isolation, memory-only fallback

### n8n-Compatible Workflow Engine
- Types: `Workflow{id, name, nodes[], connections{}, settings{}}`
- DAG executor with topological sort, 1:N / N:1 connections
- IMAP IDLE trigger (RFC 2177), cron trigger
- Skip empty trigger executions
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
- `cmd/kuro/main.go` — entry point, wiring
- `internal/chat/` — chat service, system prompt
- `internal/skill/` — skill registry + built-in handlers
- `internal/pipeline/` — workflow store, parser, executor, scheduler
- `internal/credential/` — AES-256-GCM encrypted vault (git-versioned)
- `internal/document/` — git-versioned markdown store
- `internal/provider/` — OpenAI-compatible LLM provider
- `internal/settings/` — YAML settings persistence
- `internal/api/` — HTTP API routes
- `ui/src/pages/` — React pages

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
```

## In Progress

### Agent Loop (agentic chat)
- Current: single-round skill execution
- Goal: multi-round while loop (AI -> skill call -> result -> AI continues)
- Researching: NextClaw, OpenClaw, Gemini CLI, Codex, Claude Code, Google ADK, Vercel AI SDK, OpenAI Agents SDK, LangChain/LangGraph, Microsoft SK/AutoGen

## Known Issues
- No agent loop — AI only executes one skill call per message
- Plugin architecture for skills not yet implemented
