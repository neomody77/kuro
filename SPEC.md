# Kuro - Personal AI Assistant

## Overview

Kuro is a self-hosted personal AI assistant with a chat-driven interface. Users interact through natural language; Kuro translates intent into deterministic code/config, commits to Git, and executes via a DAG-based pipeline engine. AI generates, code executes.

## Architecture

```
kuro (Go single binary)              kuro-recovery (Go single binary)
├── HTTP Server                       ├── Health check
│   ├── Web UI (embedded React SPA)   ├── Version manager (multi-version)
│   └── REST API                      ├── Reverse proxy (?v= routing)
├── Chat (AI intent → YAML → commit)  └── Management UI (:8081)
├── Pipeline Engine (DAG executor)
├── Skill Registry
├── Credential Store (AES-256-GCM)
├── Document Store
├── Git Store (all config versioned)
├── SQLite (runtime state, logs)
└── Channel Bridge (on-demand Node.js)
```

## Tech Stack

| Layer | Choice | Reason |
|-------|--------|--------|
| Engine | Go | Single binary, low memory (~30-50MB), great concurrency |
| Recovery | Go | Independent binary, minimal deps |
| UI | React + Vite + TailwindCSS | NextClaw-style UI reuse |
| Pipeline def | YAML | Human-readable, git-diff friendly |
| Storage | Git + SQLite | Git for config versioning, SQLite for runtime |
| Encryption | AES-256-GCM | Credential encryption, master key outside git |

## Directory Structure

```
~/.kuro/
├── bin/
│   ├── kuro                  # Main binary
│   └── kuro-recovery         # Recovery binary
├── versions/                 # Multi-version support
│   ├── current -> 0.2.0      # Symlink to default
│   ├── 0.2.0/
│   │   ├── kuro
│   │   └── ui/
│   └── 0.1.0/
│       ├── kuro
│       └── ui/
├── users/
│   └── {username}/
│       ├── repo/             # Git repository
│       │   ├── pipelines/    # Pipeline definitions (YAML)
│       │   ├── skills/       # Custom skill definitions
│       │   ├── credentials/  # Encrypted credential files
│       │   ├── documents/    # Notes, templates, knowledge base
│       │   ├── channels/     # Channel configs (optional)
│       │   └── .kuro/
│       │       └── versions.json
│       ├── data/
│       │   └── kuro.db       # SQLite (run state, logs)
│       └── master.key        # Credential master key (NOT in git)
├── bridges/                  # On-demand Node.js bridges
│   └── whatsapp/             # Installed when needed
└── config.yaml               # Global config
```

## Authentication

Env var at startup:
```
USER_TOKENS=alice:tok_abc123;bob:tok_def456;admin:tok_xyz789
```

- Parsed on startup, no user database needed
- Token via `Authorization: Bearer <token>` header or `?token=` param
- Each user gets isolated folder under `users/{username}/`
- No token = single-user mode (no auth required)

## Multi-Version & Recovery

### Recovery Process (:8081)

Recovery is always-on, manages engine processes:

```
Recovery (:8080 proxy + :8081 admin)
  ├── Engine 0.2.0 (:9001)  ← current
  ├── Engine 0.1.0 (:9002)  ← running for observation
  └── Engine 0.0.9 (:9003)  ← stopped
```

### Version Routing

```
GET /?v=0.1.0        → proxied to engine 0.1.0
GET /api/tasks?v=0.2.0 → proxied to engine 0.2.0
GET /                → proxied to current version
```

### Upgrade Flow

1. Upload new version binary + UI assets to recovery
2. Recovery starts new engine process on next available port
3. Old and new run simultaneously
4. User tests via `?v=` parameter
5. "Set as default" in recovery UI
6. Observe → stop old version when confident

### Recovery Admin UI (:8081)

Simple management page:
- List all versions with status (running/stopped)
- Set default version
- Start/stop/delete versions
- Health status of each engine
- Auto-rollback toggle (restart last-known-good on crash)

## Pipeline Engine

### Node-Based DAG (inspired by n8n)

```yaml
name: daily-report
description: Collect daily reports and send summary to boss
trigger:
  type: cron
  expr: "0 18 * * 1-5"

nodes:
  fetch:
    action: email.fetch
    credentials: work-email
    params:
      subject_match: "daily report"
      since: today
    next: [filter, archive]

  filter:
    action: transform.jq
    input: "{{ nodes.fetch.messages }}"
    expr: '[.[] | select(.from | contains("team"))]'
    next: summarize

  archive:
    action: file.save
    params:
      path: "archive/{{ now | date('YYYY-MM-DD') }}.json"
      data: "{{ nodes.fetch.messages }}"

  summarize:
    action: ai.complete
    provider: openai/gpt-4o
    fallback: raw                    # on AI failure: skip | error | raw
    params:
      prompt: "Summarize in 3 bullet points:"
      input: "{{ nodes.filter.output }}"
    next: send

  send:
    action: email.send
    credentials: work-email
    params:
      to: boss@example.com
      subject: "{{ now | date('YYYY-MM-DD') }} Team Daily Summary"
      body: "{{ nodes.summarize.output }}"
```

### Node Types

| Type | Description |
|------|-------------|
| Trigger | cron / webhook / event / manual |
| Action | Concrete operation (email, http, file, shell) |
| Transform | Data transformation (jq, template, regex) |
| AI | LLM call (explicit, with fallback policy) |
| Condition | Boolean branch (on_true / on_false) |
| Loop | Iterate over array |
| Switch | Multi-way branch |
| Merge | Join parallel branches |
| Skill | Reference a skill (sub-pipeline) |
| Error | Error handler node |

### Flow Control

```yaml
# Condition
check:
  action: condition
  expr: "{{ nodes.fetch.count > 0 }}"
  on_true: process
  on_false: notify_empty

# Loop
loop_items:
  action: loop
  items: "{{ nodes.fetch.messages }}"
  body: process_one
  next: merge_results

# Switch
switch_type:
  action: switch
  value: "{{ nodes.parse.type }}"
  cases:
    urgent: handle_urgent
    normal: handle_normal
    _default: handle_other
```

### Design Principle: AI Generates, Code Executes

| Phase | Method | Why |
|-------|--------|-----|
| Understand user intent | AI | Only AI can parse natural language |
| Generate pipeline YAML | AI → human confirm | AI drafts, human approves |
| Trigger (cron/webhook) | Code | Deterministic |
| Data fetch (email/http) | Code | Deterministic |
| Data transform | Code (jq/template) | Testable, predictable |
| AI content generation | Explicit AI node | Clearly marked, has fallback |
| Send/write output | Code | Deterministic |
| Error retry | Code (policy) | Deterministic |
| Rollback | Code (git revert) | Deterministic |

## Skills

Reusable pipeline fragments, callable from pipelines or chat.

### Skill Types

| Type | Description | Example |
|------|-------------|---------|
| Built-in | Ships with binary | email, http, file, template, cron, shell |
| AI | LLM capabilities | summarize, translate, extract |
| System | Host operations | shell.exec, notify |
| Custom | User-defined YAML | Business-specific logic |

### Skill Definition

```yaml
# skills/email-digest.yaml
name: email-digest
description: Fetch emails and generate digest
inputs:
  - name: mailbox
    type: credential/email
    required: true
  - name: filter
    type: string
    default: "*"
  - name: period
    type: enum[today, week, month]
    default: today
outputs:
  - name: digest
    type: string
  - name: count
    type: number
nodes:
  fetch:
    action: email.fetch
    credentials: "{{ inputs.mailbox }}"
    params:
      since: "{{ inputs.period }}"
    next: summarize
  summarize:
    action: ai.complete
    fallback: raw
    params:
      input: "{{ nodes.fetch.messages }}"
    next: output
  output:
    action: transform.template
    template: |
      Found {{ nodes.fetch.messages | length }} emails.
      {{ nodes.summarize.output }}
```

## Credential Management (n8n-style)

### Storage

```yaml
# credentials/work-email.yaml (encrypted fields in git)
name: work-email
type: email
data:
  imap_host: ENC[AES256:base64...]
  imap_user: ENC[AES256:base64...]
  imap_pass: ENC[AES256:base64...]
  smtp_host: ENC[AES256:base64...]
  smtp_user: ENC[AES256:base64...]
  smtp_pass: ENC[AES256:base64...]
```

- Master key stored at `users/{name}/master.key` (NOT in git)
- AES-256-GCM encryption per field
- Pipelines reference by name: `credentials: work-email`
- UI for create/edit/delete, never shows decrypted values after save

### Credential Types

Pre-defined schemas for common services:
- email (IMAP/SMTP)
- http-basic / http-bearer / http-oauth2
- openai / anthropic / custom-llm
- telegram-bot / discord-bot / slack-bot
- generic (key-value pairs)

## Document Management

```
users/{name}/repo/documents/
├── notes/            # Quick notes
├── templates/        # Email/report templates
├── runbooks/         # Procedures
└── knowledge/        # Reference material
```

- All Markdown, versioned in Git
- AI can search/reference documents as context
- Pipelines can use templates: `{{ documents['templates/report.md'] }}`
- Web UI for browse/edit/preview

## Chat Interaction

### Flow

```
User: "Send me a daily summary of team emails at 6pm"
Kuro: I'll set that up. A few questions:
      1. Which email account? (select from saved credentials or add new)
      2. How to identify "team" emails?
User: Use work-email, filter by subject containing "report"
Kuro: Here's the pipeline I'll create:

      Pipeline: daily-team-summary
      Trigger: Every day at 18:00
      Steps:
        1. Fetch emails from work-email (subject: *report*)
        2. AI: Summarize content (fallback: send raw)
        3. Send summary to you

      [View YAML]  [Confirm]  [Edit]
```

User clicks Confirm → Git commit → Engine loads pipeline.

### Chat can also:

- "Pause the daily report pipeline"
- "Show me what ran today"
- "Add a step to also save to notes"
- "Run email-digest now"

## Web UI

### Style

Reuse NextClaw UI style: React + Vite + TailwindCSS, dark theme, sidebar layout.

### Pages

| Page | Content |
|------|---------|
| Chat | Main interface, AI conversation, quick actions |
| Pipelines | List/detail view with DAG visualization, run history |
| Skills | Installed skills, install/create new |
| Documents | File browser, markdown editor/preview |
| Vault | Credential management (create/edit/delete) |
| Logs | Execution logs, git history, error details |
| Settings | Provider config, channel config, preferences |

### Mobile (PWA)

- Responsive layout, installable as PWA
- Mobile: bottom tab navigation (Chat, Pipelines, Vault, More)
- Desktop: sidebar navigation

### Pipeline Detail View

```
Pipeline: daily-team-summary          [Edit YAML] [Run Now] [Pause]

  Trigger: cron 18:00 weekdays
       |
  [fetch] email.fetch
       |------------|
  [filter] jq      [archive] file.save
       |
  [summarize] ai.complete (gpt-4o)
       |
  [send] email.send

  Recent Runs:
  [ok] 2026-03-07 18:00 (1.2s)
  [ok] 2026-03-06 18:00 (0.9s)
  [err] 2026-03-05 18:00 - summarize timeout
```

## API

```
GET    /api/pipelines              # List pipelines
POST   /api/pipelines              # Create pipeline
GET    /api/pipelines/:id          # Get pipeline detail
PUT    /api/pipelines/:id          # Update pipeline
DELETE /api/pipelines/:id          # Delete pipeline
POST   /api/pipelines/:id/run      # Manual trigger
GET    /api/pipelines/:id/runs     # Run history

GET    /api/skills                 # List skills
POST   /api/skills                 # Create/install skill
GET    /api/skills/:id             # Skill detail

GET    /api/credentials            # List credentials (no secrets)
POST   /api/credentials            # Create credential
PUT    /api/credentials/:id        # Update credential
DELETE /api/credentials/:id        # Delete credential

GET    /api/documents              # List documents
GET    /api/documents/*path        # Get document content
PUT    /api/documents/*path        # Create/update document
DELETE /api/documents/*path        # Delete document

POST   /api/chat                   # Send chat message
GET    /api/chat/history           # Chat history

GET    /api/logs                   # Execution logs
GET    /api/logs/:run_id           # Specific run log

GET    /api/settings               # Get settings
PUT    /api/settings               # Update settings
```

## Go Project Structure

```
kuro/
├── cmd/
│   ├── kuro/                 # Main binary
│   │   └── main.go
│   └── kuro-recovery/        # Recovery binary
│       └── main.go
├── internal/
│   ├── server/               # HTTP server, router, middleware
│   ├── auth/                 # Token auth, user isolation
│   ├── chat/                 # Chat handler, AI intent parsing
│   ├── pipeline/             # DAG engine, node executor, scheduler
│   ├── skill/                # Skill registry, built-in skills
│   ├── credential/           # Encryption, credential CRUD
│   ├── document/             # Document store
│   ├── gitstore/             # Git operations
│   ├── db/                   # SQLite, migrations
│   ├── provider/             # AI provider adapters (OpenAI, Anthropic, etc)
│   ├── bridge/               # On-demand Node.js bridge manager
│   ├── action/               # Built-in action implementations
│   │   ├── email/
│   │   ├── http/
│   │   ├── file/
│   │   ├── shell/
│   │   ├── transform/
│   │   └── template/
│   └── config/               # Config loading
├── ui/                       # React frontend (Vite + TailwindCSS)
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   ├── hooks/
│   │   ├── api/
│   │   └── store/
│   ├── package.json
│   └── vite.config.ts
├── recovery/                 # Recovery-specific code
│   ├── proxy/                # Reverse proxy with ?v= routing
│   ├── version/              # Version manager
│   ├── health/               # Health checker
│   └── ui/                   # Recovery admin UI (minimal)
├── go.mod
├── go.sum
├── Makefile
└── SPEC.md
```

## Build & Deploy

```bash
# Build
make build          # → bin/kuro + bin/kuro-recovery
make ui             # → Build React UI, embed in Go binary

# Run
USER_TOKENS=admin:secret ./kuro              # Engine on :8080
./kuro-recovery --engine=./kuro              # Recovery on :8080 (proxy), :8081 (admin)

# Development
make dev            # Hot reload Go + Vite dev server
make test           # Go tests + frontend tests
make lint           # golangci-lint + eslint
```

## Code Quality

- Go: `golangci-lint` strict config + pre-commit hooks
- Frontend: ESLint + Prettier + TypeScript strict
- YAML: Schema validation for pipeline/skill/credential definitions
- CI: lint + test on every commit
- Git: conventional commits
