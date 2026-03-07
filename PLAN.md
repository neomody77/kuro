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

---

## In Progress / Next

### Agent Loop (agentic chat)

> 当前问题：AI 调用 skill 后只执行一轮就停了，无法完成需要多步推理的任务。
> 参考 OpenClaw / claw0 的 agent loop 模式。

- [ ] Agent loop: AI -> skill call -> result fed back -> AI continues -> repeat
  - `while` loop in `SendMessage`, check `stop_reason` / no more skill calls
  - Max iterations cap (default 10) to prevent runaway
  - Skill result as `tool` role message fed back to conversation context
  - Destructive skill -> break loop, return confirm request -> `ConfirmAction` resumes loop
- [ ] SSE streaming for agent loop (stream each step to frontend in real-time)
- [ ] Frontend: show intermediate skill calls and results as they happen

### Settings / Provider 配置

> 当前问题：settings API 返回空 providers，实际在用环境变量里的 OpenRouter。

- [ ] Settings 页面配置的 provider 应该正确持久化和加载
- [ ] 移除环境变量 fallback，统一走 settings 配置
- [ ] 首次启动引导用户配置 provider

---

## Priority

### 持久化
- [ ] Pipeline 执行结果迁移至 SQLite
- [ ] 统一存储层，抽象 Repository/Store 接口
- [ ] 文档存储保持 Markdown 文件（不入数据库）

### 审计日志
- [ ] 全链路操作日志（用户操作、AI 行为、Skill 执行、Pipeline 节点）
- [ ] 结构化存储，支持按时间/类型/会话筛选
- [ ] Trace ID 串联完整调用链

### 测试覆盖
- [ ] Chat 模块测试（agent loop 多轮测试）
- [ ] Pipeline 引擎测试
- [ ] API 端点集成测试

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

---

## UI
- [ ] Chat 输入框 react-textarea-autosize
- [ ] Chat UI skill 调用卡片：默认折叠显示 skill 名称和状态，展开显示完整结果；JSON 结果自动 format
- [x] BlockNote 文档编辑器（默认编辑模式，Cmd+S 保存，markdown 双向转换）
- [x] react-arborist 文件树（虚拟化渲染，搜索过滤，右键菜单，文件类型图标）
- [x] 文档 API 目录列表（GET /api/documents/{dir} 返回子项，前端递归获取完整树）
- [ ] Workflow visual editor (node graph)
- [ ] Execution logs viewer
- [ ] DataTable UI

## Security
- [ ] Credential 调用方隔离（Pipeline 返回真实值，Chat 返回 placeholder）

## Infra
- [ ] Docker build
- [ ] SQLite persistent storage
