# Kuro Acceptance Criteria & Test Plan

Each module must pass all criteria before considered complete.

## Module 1: Core Skills System

Skills are the universal building block. Everything is a skill — email, file ops, shell exec, credential access, document ops, AI calls. Chat invokes skills.

### Core Skills (built-in)

| Skill | Action | Inputs | Outputs | Test |
|-------|--------|--------|---------|------|
| `credential.list` | List saved credentials | - | []Credential (name+type) | Unit: returns list without secrets |
| `credential.get` | Get credential by name | name | Credential (decrypted) | Unit: decrypt matches original |
| `credential.save` | Save/update credential | name, type, data | ok + git commit | Unit: encrypted in file, commit exists |
| `credential.delete` | Delete credential | name | ok + git commit | Unit: file removed, commit exists |
| `document.list` | List documents | path? | []DocEntry (tree) | Unit: returns folder tree |
| `document.get` | Read document content | path | string (markdown) | Unit: reads file content |
| `document.save` | Write document | path, content | ok + git commit | Unit: file written, commit exists |
| `document.delete` | Delete document | path | ok + git commit | Unit: file removed, commit exists |
| `document.search` | Search docs by keyword | query | []DocEntry | Unit: finds matching docs |
| `shell.exec` | Run shell command | cmd, timeout? | stdout, stderr, exit_code | Unit: runs echo, captures output |
| `email.fetch` | Fetch emails via IMAP | cred, filter | []Email | Integration: mock IMAP |
| `email.send` | Send email via SMTP | cred, to, subject, body | ok | Integration: mock SMTP |
| `http.request` | HTTP request | method, url, headers?, body? | status, headers, body | Unit: mock server |
| `file.read` | Read file | path | content | Unit: reads test file |
| `file.write` | Write file | path, content | ok | Unit: creates file |
| `file.list` | List directory | path | []Entry | Unit: lists test dir |
| `transform.jq` | JQ-like transform | input, expr | output | Unit: filter/map/select |
| `template.render` | Render Go template | template, vars | string | Unit: template output |
| `ai.complete` | Call LLM | provider, prompt, input | response | Unit: mock provider |

### Acceptance

- [ ] All core skills implement `ActionHandler` interface
- [ ] Each skill has unit tests with >80% coverage
- [ ] Skills are registered in default registry at startup
- [ ] Chat can discover and invoke any skill by name
- [ ] Skill execution result is returned to chat

## Module 2: Pipeline Engine

### Features

| Feature | Description | Test |
|---------|-------------|------|
| Parse pipeline YAML/JSON | Load definition from file | Unit: parse valid + invalid |
| Validate DAG | Detect cycles, missing refs | Unit: cyclic graph fails, valid passes |
| Execute linear pipeline | Run A → B → C | Unit: 3-node chain, all succeed |
| Execute parallel branches | A → [B, C] → D | Unit: B and C run concurrently |
| Condition node | Branch on expression | Unit: true/false paths |
| Loop node | Iterate array | Unit: processes each item |
| Switch node | Multi-way branch | Unit: routes by value |
| Template expressions | Resolve {{ nodes.X.output }} | Unit: variable substitution |
| Error handling | Node failure + fallback | Unit: skip/error/raw behavior |
| Cron scheduling | Parse cron expr, trigger on time | Unit: next fire time calculation |
| Manual trigger | Run pipeline on demand | Unit: API trigger → execution |
| Run history | Save run results | Unit: store + retrieve run |

### Acceptance

- [ ] All node types work (action, condition, loop, switch, merge)
- [ ] Parallel branches execute concurrently
- [ ] Template expressions resolve correctly
- [ ] Pipeline CRUD via API (create, read, update, delete)
- [ ] Pipeline detail view shows nodes + run history
- [ ] Manual run from UI works end-to-end
- [ ] Cron trigger fires at correct time

## Module 3: Credential Store

### Features

| Feature | Description | Test |
|---------|-------------|------|
| Master key generation | Generate 32-byte random key | Unit: key is 32 bytes |
| Master key load | Read from file | Unit: load matches saved |
| Encrypt field | AES-256-GCM encrypt | Unit: ciphertext != plaintext |
| Decrypt field | AES-256-GCM decrypt | Unit: roundtrip matches |
| Save credential | Encrypt + write + git commit | Unit: file has ENC[], commit exists |
| Load credential | Read + decrypt | Unit: roundtrip matches |
| List credentials | Names + types, no secrets | Unit: no plaintext in output |
| Delete credential | Remove file + git commit | Unit: file gone, commit exists |
| Credential types | Schema validation per type | Unit: email requires imap_host etc |

### Acceptance

- [ ] Master key never appears in git
- [ ] All credential fields encrypted at rest
- [ ] CRUD works end-to-end via API
- [ ] UI shows credential list, add form, edit, delete
- [ ] Credentials injectable into pipeline nodes

## Module 4: Document Store

### Features

| Feature | Description | Test |
|---------|-------------|------|
| List tree | Return folder/file structure | Unit: nested folders |
| Read file | Get markdown content | Unit: returns content |
| Write file | Save + git commit | Unit: file written, commit exists |
| Delete file | Remove + git commit | Unit: file gone, commit exists |
| Create folder | Create nested directory | Unit: mkdir -p behavior |
| Search | Find docs by keyword | Unit: matches in content |
| Path safety | Prevent traversal (../) | Unit: ../etc/passwd rejected |

### Acceptance

- [ ] CRUD works end-to-end via API
- [ ] UI file browser shows tree, click to view content
- [ ] Markdown editor + preview in UI
- [ ] Git history per document
- [ ] Documents referenceable from pipeline templates

## Module 5: Git Store

### Features

| Feature | Description | Test |
|---------|-------------|------|
| Init repo | Create .git if not exists | Unit: .git dir created |
| Add + commit | Stage and commit files | Unit: commit in log |
| Log | Last N commits | Unit: returns hash, msg, time |
| Revert | Revert a specific commit | Unit: file restored to prior state |
| Diff | Show changes in a commit | Unit: returns patch |
| Status | Working tree status | Unit: detects modifications |

### Acceptance

- [ ] Every config change (credential, pipeline, document) produces a git commit
- [ ] Revert actually undoes a change
- [ ] Log shows human-readable history
- [ ] UI logs page shows git history

## Module 6: Chat + AI Integration

### Features

| Feature | Description | Test |
|---------|-------------|------|
| Send message | POST /api/chat returns response | Unit: echo works |
| Chat history | GET /api/chat/history | Unit: returns past messages |
| Skill discovery | AI can list available skills | Unit: returns skill names |
| Intent → skill | AI maps "check email" to email.fetch | Integration: prompt test |
| Confirm before execute | Destructive skills require confirmation | Unit: shell.exec needs confirm |
| Pipeline creation via chat | "set up daily report" → YAML | Integration: generates valid YAML |
| Session context | Multi-turn conversation | Unit: context preserved |

### Acceptance

- [ ] Chat UI sends/receives messages
- [ ] AI can call any core skill
- [ ] Pipeline creation from chat works end-to-end
- [ ] Confirmation flow for destructive actions
- [ ] Chat history persists across page refresh

## Module 7: Recovery + Multi-Version

### Features

| Feature | Description | Test |
|---------|-------------|------|
| Version scan | Find versions in directory | Unit: lists installed versions |
| Start engine | Launch binary on port | Unit: process starts, health check passes |
| Stop engine | Graceful shutdown | Unit: process exits |
| Set default | Update current symlink | Unit: symlink points to correct version |
| Proxy routing | ?v= routes to version | Unit: correct backend hit |
| No ?v= routing | Routes to default | Unit: default backend hit |
| Health check | Periodic /api/health ping | Unit: detect failure |
| Auto rollback | Switch to last-good on crash | Unit: default changes after N failures |

### Acceptance

- [ ] Upload new version via admin UI
- [ ] Multiple versions run simultaneously
- [ ] ?v= parameter switches version
- [ ] Crash triggers auto-rollback
- [ ] Admin UI shows all versions with status

## Module 8: Web UI

### Pages — each must be functional, not just a shell

| Page | Must do | Test |
|------|---------|------|
| Chat | Send message, show response, history | Manual: send msg, see response |
| Pipelines | List, click → detail (nodes + runs), create, run, pause | Manual: full CRUD |
| Skills | List, click → detail (inputs/outputs/nodes), search | Manual: view skill detail |
| Documents | File tree, click → view content, edit, save, new | Manual: full CRUD |
| Vault | List, add credential (form with type fields), edit, delete | Manual: full CRUD |
| Logs | List runs, click → run detail (per-node status) | Manual: view run detail |
| Settings | Provider config, save, theme toggle | Manual: save and reload |

### Acceptance

- [ ] All pages functional, not placeholder
- [ ] Theme toggle works (light default, dark mode)
- [ ] Mobile responsive
- [ ] PWA installable

## Test Execution Order

1. **Git Store** — foundation, everything depends on it
2. **Credential Store** — needs git store
3. **Document Store** — needs git store
4. **Core Skills** — needs credential + document stores
5. **Pipeline Engine** — needs skills
6. **Chat + AI** — needs skills + pipeline
7. **Web UI** — needs all APIs working
8. **Recovery** — independent, can test in parallel

## Running Tests

```bash
make test                  # All Go unit tests
go test ./internal/gitstore/ -v    # Single module
go test ./internal/credential/ -v
go test ./internal/document/ -v
go test ./internal/pipeline/ -v
go test ./internal/action/... -v
go test ./internal/skill/ -v
cd ui && npm test          # Frontend tests
```
