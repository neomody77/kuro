# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make build          # Build engine binary (builds UI first, output: bin/kuro)
make build-cli      # Build CLI binary (bin/kuro-cli)
make build-recovery # Build recovery/version manager binary (bin/kuro-recovery)
make build-all      # Build all three binaries
make dev            # Run Go backend + Vite dev server concurrently
make dev-go         # Run Go backend only
make dev-ui         # Run Vite dev UI only
make check          # Run vet + lint + test (full quality gate)
make test           # go test ./...
make lint           # golangci-lint run ./...
```

Run a single Go test:
```bash
go test ./internal/pipeline/ -run TestExecutor
```

UI development:
```bash
cd ui && npm install && npm run dev
```

## Architecture Overview

Kuro is a personal AI assistant platform with three binaries:

- **`cmd/kuro`** — Main HTTP server with embedded React UI, pipeline executor, chat/agent service, skill registry
- **`cmd/kuro-cli`** — CLI client that talks to the server via REST API
- **`cmd/kuro-recovery`** — Sidecar for health-checking, version management, and reverse-proxy routing

### Request Flow

Config (`~/.kuro/config.yaml`) → Auth middleware (optional `USER_TOKENS` env) → Per-user data isolation → REST API handlers → Service layer

### Storage Strategy

| What | Where | Format |
|------|-------|--------|
| Credentials, documents, workflows | `~/.kuro/users/{user}/repo/` | Git-versioned files |
| Chat history, executions, audit logs, variables, data tables | `~/.kuro/users/{user}/data/kuro.db` | Per-user SQLite (WAL mode, pure Go via modernc.org/sqlite) |
| Settings, config | `~/.kuro/settings.yaml`, `~/.kuro/config.yaml` | YAML |
| ADK sessions | File-based JSON | JSON |

Credentials are AES-256-GCM encrypted with a per-user master key stored outside the git repo.

### Key Internal Packages

- **`api/`** — All REST routes registered in a single `RegisterRoutes()` function. Routes mirror n8n's API structure (`/api/v1/workflows`, `/api/v1/credentials`, etc.)
- **`pipeline/`** — n8n-compatible workflow engine: DAG builder, topological executor, expression evaluator, node handlers (IMAP, SMTP, Cron, If)
- **`chat/`** — Chat service with conversation state, skill invocation, and human-in-the-loop confirmation for destructive actions
- **`adk/`** — Google ADK integration: OpenAI-compatible LLM adapter, skill→tool adapter, session persistence, agent runner factory
- **`db/`** — SQLite wrapper with migration system (8 tables), per-user lazy-open cache
- **`credential/`** — AES-256-GCM encryption, credential CRUD with git versioning
- **`skill/`** — Skill registry (in-memory) + file-based storage (YAML). Skills are invocable from chat, API, CLI, or workflow nodes
- **`action/`** — Low-level action handlers (shell, HTTP, file, email, template, transform) each implementing `pipeline.ActionHandler`
- **`provider/`** — AI provider abstraction with OpenAI-compatible adapter (works with OpenAI, OpenRouter, Anthropic endpoints)
- **`events/`** — Pub/sub event hub with SSE endpoint and 50-event ring buffer for late joiners
- **`auth/`** — Token-based multi-user auth from `USER_TOKENS` env; defaults to single-user mode

### Frontend (ui/)

React 19 + TypeScript + Vite + TailwindCSS 4. Two view modes:
- `/app/*` — Traditional sidebar layout
- `/desktop/*` — Window manager with draggable/resizable windows (react-rnd), taskbar, app launcher

Key libraries: Mantine (UI components), BlockNote (document editor), react-arborist (file tree), react-markdown. Chat uses SSE streaming via `useChatStream` hook.

## Conventions

- Go packages grouped by feature under `internal/`
- Interfaces for pluggability: `Provider`, `ActionHandler`, `NodeHandler`, `Store`
- `sync.RWMutex` for read-heavy registries, channels for event streaming
- HTTP server uses standard library `http.ServeMux` (no framework)
- Linting: 14 linters configured in `.golangci.yml` (errcheck, govet, staticcheck, unused, gosimple, ineffassign, bodyclose, gocritic, gofmt, goimports, misspell, unconvert, unparam, prealloc)
- Docker: 3-stage build (Node → Go → Alpine runtime)
