# Kuro TODO

## Done

- [x] Chat 会话 + 消息持久化（JSONL）
- [x] Session list sidebar
- [x] Markdown rendering for documents
- [x] n8n-compatible workflow/node/execution format
- [x] n8n-compatible API routes
- [x] Executor with DAG topological sort, 1:N / N:1 connections
- [x] IMAP IDLE (RFC 2177) for near-instant email notification
- [x] Skip empty trigger executions
- [x] Credential ID-based storage (all ops use `id` as unique key, `name` is display only)
- [x] Credential CRUD via skill + API + UI
- [x] Document CRUD via skill (list, get, save, rename, delete, search)
- [x] Pipeline CRUD via skill (create, list, get, update, activate, deactivate, execute, delete)
- [x] File CRUD via skill (read, write, list, delete, rename)
- [x] Consolidated skills (credential, document, file, pipeline -> single skill with action param)
- [x] Settings persistence (providers, active model)
- [x] Runtime provider hot-swap
- [x] IME composition handling (Enter during CJK input)
- [x] Chat message one-click copy
- [x] Run history clear button
- [x] Multi-user session persistence
- [x] ADK agent loop — multi-turn tool calling via Google ADK Go v0.6.0
- [x] SSE streaming — real-time text deltas + tool call events via `POST /api/chat/stream`
- [x] Frontend SSE consumer (`useChatStream` hook) with streaming dots indicator
- [x] Collapsible tool call cards (name + status collapsed, JSON input/output expanded)
- [x] Settings provider CRUD with presets (OpenAI, OpenRouter, Anthropic, Custom)
- [x] No env var fallback — must configure via Settings UI
- [x] Hot-swap provider + re-create ADK runner at runtime
- [x] Collapsible sidebar (icon-only mode with localStorage persistence)
- [x] Session reuse (empty "New Chat" recycled instead of duplicated)
- [x] File-based ADK session adapter（persist across restarts）
- [x] HITL: `RequestConfirmation` for destructive tool calls via SSE
- [x] Pipeline 执行结果迁移至 SQLite（`internal/db/execution_store.go`）
- [x] 统一存储层，抽象 Repository/Store 接口
- [x] 文档存储保持 Markdown 文件（不入数据库）
- [x] Per-user SQLite 数据库隔离（`UserDBCache`，`modernc.org/sqlite` 纯 Go）
- [x] Schema migration 系统（版本追踪，8 张表）
- [x] 全链路审计日志 + 结构化存储 + Trace ID 串联
- [x] Optional SQLite — `DBCache` 为 nil 时回退到 JSON 文件存储
- [x] Chat / Pipeline / API / SQLite 存储层测试覆盖
- [x] 全局事件流 `GET /api/events`（SSE pub/sub + 50 条 ring buffer）
- [x] `NotificationCenter` 接入后端 SSE 实时事件
- [x] Chat 输入框 react-textarea-autosize
- [x] BlockNote 文档编辑器（默认编辑模式，Cmd+S 保存，markdown 双向转换）
- [x] react-arborist 文件树（虚拟化渲染，搜索过滤，右键菜单，文件类型图标）
- [x] Execution logs viewer（过滤、节点结果详情、状态/耗时/错误展示）
- [x] DataTable UI（CRUD 列表 + 详情视图，行内编辑，列类型支持）
- [x] 双视图模式：App 视图 (`/app/*`) + Desktop 视图 (`/desktop/*`)
- [x] Desktop 窗口管理器（react-rnd：Window, Desktop, Taskbar, AppLauncher, NotificationCenter）
- [x] 窗口布局持久化（localStorage + `GET/PUT /api/settings/layout` 服务端同步）
- [x] Web Search skill（Tavily API）
- [x] Credential 调用方隔离（Pipeline 真实值，Chat placeholder）
- [x] Docker build（3-stage: node→go→alpine，non-root，`/data` volume）
- [x] SQLite persistent storage（per-user，WAL mode）

---

## TODO

### 插件化架构（兼容 OpenClaw）

#### Phase 1: Skill 层

- [ ] Skill 声明式注册，从 `RegisterDefaults()` 硬编码解耦
- [ ] OpenClaw 兼容的 skill 目录格式（`SKILL.md` + YAML frontmatter）
- [ ] 三级加载优先级：内置 > 用户全局 `~/.kuro/skills` > 工作区 skills
- [ ] Go handler 通过接口解耦（`CredentialProvider`、`DocumentProvider`）
- [ ] 外部 skill 执行：Shell stdin/stdout、HTTP 代理
- [ ] Skill 运行时门控（env/bins/os 过滤）

#### Phase 2: Plugin 层

- [ ] `PluginApi` 接口（registerTool, registerHook, registerProvider, registerHttpRoute）
- [ ] 生命周期钩子（before/after skill_call, chat_response, pipeline_node, llm_input/output）
- [ ] Plugin 清单格式（`kuro.plugin.json`）
- [ ] Plugin 发现、加载、安全校验
- [ ] 外部 plugin 桥接：gRPC / HTTP / WASM

### UI

- [ ] Workflow visual editor (node graph)

### 事件流扩展

- [ ] IMAP IDLE 邮件到达事件
- [ ] Cron 任务触发事件
