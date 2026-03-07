# Agent Loop 对比研究

11 个框架的 agent loop 实现对比。详细资料见各子文档 `01-nextclaw.md` ~ `11-anthropic-claude-sdk.md`。

---

## 一、核心循环模式

| 框架 | 语言 | 循环形式 | 循环位置 |
|------|------|----------|----------|
| NextClaw | TypeScript | `while (!done)` + plan/execute phases | AgentLoop 类 |
| Claude Code | TypeScript | `while (tool_call)` | 内部主循环 |
| Gemini CLI | TypeScript | `do...while (functionCalls)` | AgentLoop, turnBased |
| OpenAI Codex | Rust | SQ/EQ event loop | Core session |
| OpenClaw | JavaScript | 双层 `while`: inner(tools) + outer(follow-ups) | pi-agent-core |
| Microsoft SK | Python/C# | `for i in range(max_auto_invoke)` | Chat completion client |
| Microsoft AutoGen | Python | `for iteration in range(max_tool_iterations)` | Agent 内部 |
| Google ADK Go | Go | `for { runOneStep() }`, `iter.Seq2` iterator | Flow.Run() |
| OpenAI Agents SDK | Python | `while True` + match turn_result.next_step | Runner.run() |
| Vercel AI SDK | TypeScript | `do...while` (generateText) / recursive streamStep | generateText/streamText |
| LangChain/LangGraph | Python | Graph: agent → tools_condition → tools → agent | StateGraph 节点+边 |
| Anthropic Claude API | 多语言 | `while stop_reason == "tool_use"` | 用户自行实现 / Agent SDK |

**三种主要范式：**

1. **命令式 while 循环**：NextClaw, Claude Code, OpenClaw, SK, AutoGen, OpenAI Agents SDK, Anthropic API
2. **事件驱动 / 迭代器**：Codex (SQ/EQ), ADK Go (`iter.Seq2`)
3. **图/声明式**：LangGraph (StateGraph 节点+边)

---

## 二、工具系统

| 框架 | 工具定义方式 | Schema 来源 | 并行执行 |
|------|-------------|-------------|----------|
| NextClaw | Tool 类 + JSON Schema | 手动 JSON Schema | 是 |
| Claude Code | 内置 tools (Read/Write/Bash 等) | 硬编码 | 是 |
| Gemini CLI | FunctionDeclaration | 手动 JSON Schema | 是 |
| Codex | Tool trait (Rust) | 手动 | 否 (sandbox 串行) |
| OpenClaw | Tool 对象 | 手动 | 否 (串行，可中断) |
| SK | `@kernel_function` 装饰器 | 自动从类型推断 | 是 (asyncio.gather) |
| AutoGen | 普通 Python 函数传入 | 自动从类型推断 | 是 (asyncio.gather) |
| ADK Go | `functiontool.New` + Go struct | 自动从 struct tag | 是 |
| OpenAI Agents SDK | `@function_tool` 装饰器 | 自动从 type hints | 是 |
| Vercel AI SDK | `tool()` + Zod schema | Zod/Valibot/JSON Schema | 是 |
| LangGraph | `@tool` 装饰器 | 自动从 type hints + docstring | 是 (ToolNode) |
| Anthropic API | JSON 对象 (name/description/input_schema) | 手动 JSON Schema | 是 (多 tool_use blocks) |

---

## 三、终止条件

| 框架 | 主要终止条件 | 默认最大步数 |
|------|-------------|-------------|
| NextClaw | 无 tool calls / maxSteps / user abort | 可配置 |
| Claude Code | 无 tool calls / maxTurns | 无硬限制 |
| Gemini CLI | 无 function calls / maxSteps / context overflow | 可配置 |
| Codex | 无 tool calls / sandbox 超时 / user cancel | — |
| OpenClaw | 无 tool calls + 无 steering/follow-up | 32-160 |
| SK | 无 tool calls / max_auto_invoke / filter terminate | 5 |
| AutoGen | 无 tool calls / max_tool_iterations / handoff | 10 |
| ADK Go | IsFinalResponse() / EndInvocation / MaxIterations | — |
| OpenAI Agents SDK | FinalOutput / max_turns / guardrail tripwire / interruption | — |
| Vercel AI SDK | 无 tool calls / stepCountIs(n) / hasToolCall / custom | 1 (generateText), 20 (Agent) |
| LangGraph | 无 tool_calls → `__end__` / recursion_limit / remaining_steps | recursion_limit |
| Anthropic API | stop_reason: end_turn / max_tokens / stop_sequence | 用户自定 |

---

## 四、流式输出

| 框架 | 流式方式 | Token 级 | 工具执行中流式 |
|------|----------|----------|---------------|
| NextClaw | SSE stream | 是 | 否 |
| Claude Code | StreamEvent chunks | 是 | 是 (部分工具) |
| Gemini CLI | generateContentStream | 是 | 否 |
| Codex | EQ events | 是 | 是 (exec output) |
| OpenClaw | agent/turn/message/tool events | 是 | 是 (partial results) |
| SK | 底层 streaming | 是 | 否 |
| AutoGen | run_stream() events | 是 | 否 |
| ADK Go | iter.Seq2 + Partial events | 是 | 否 |
| OpenAI Agents SDK | RunResultStreaming.stream_events() | 是 | 否 |
| Vercel AI SDK | streamText + TransformStream | 是 | 是 (inline tool results) |
| LangGraph | stream(stream_mode="messages/values/updates") | 是 | 否 |
| Anthropic API | SSE (message_start/content_block_delta/input_json_delta) | 是 | 否 |

---

## 五、多智能体

| 框架 | 模式 | 机制 |
|------|------|------|
| NextClaw | 无内置 | — |
| Claude Code | Task tool 子代理 | 主代理 spawn 子代理 |
| Gemini CLI | 无内置 | — |
| Codex | 无内置 | — |
| OpenClaw | 无内置 | — |
| SK | 5 种编排模式 | Sequential, Concurrent, Handoff, GroupChat, Magentic |
| AutoGen | 4 种团队类型 | RoundRobin, Selector, Swarm, MagenticOne |
| ADK Go | 4 种组合 | Sequential, Parallel, Loop, AgentTransfer |
| OpenAI Agents SDK | Handoff | `transfer_to_<agent>` tool, state serializable |
| Vercel AI SDK | 无内置 | — |
| LangGraph | 子图 + 编排 | Supervisor, Handoff, Hierarchical, Subgraph |
| Anthropic Agent SDK | Task tool 子代理 | 内置 + 自定义 AgentDefinition |

---

## 六、Human-in-the-Loop

| 框架 | 支持方式 |
|------|----------|
| NextClaw | 无 |
| Claude Code | 权限提示 (allowedTools, permissions) |
| Gemini CLI | 沙箱模式切换 |
| Codex | 沙箱隔离 + 审批 |
| OpenClaw | Steering messages 中断 + 权限分层 |
| SK | auto_invoke=False 手动模式 |
| AutoGen | ExternalTermination + Handoff |
| ADK Go | RequestConfirmation in ToolContext |
| OpenAI Agents SDK | RunState 序列化 + needs_approval |
| Vercel AI SDK | Client-side tools (无 execute = 暂停) |
| LangGraph | interrupt_before/after + checkpointer |
| Anthropic Agent SDK | can_use_tool callback + PreToolUse hook |

---

## 七、关键设计决策对比

### 循环控制权
- **框架控制**: SK, AutoGen, ADK Go, Agents SDK, Vercel AI SDK — 循环内置在框架中
- **用户控制**: Anthropic API — 用户自己写 while 循环
- **混合**: LangGraph — 图声明式但用户定义节点逻辑

### 工具 Schema 自动化
- **自动推断**: SK, AutoGen, Agents SDK, LangGraph, ADK Go — 从类型注解/struct 自动生成
- **手动定义**: Anthropic API, Gemini CLI, NextClaw — 需要写 JSON Schema
- **DSL**: Vercel AI SDK — Zod schema

### 状态管理
- **消息列表追加**: Claude Code, Anthropic API, Vercel AI SDK, LangGraph — messages[] 不断追加
- **事件溯源**: ADK Go — append-only events, 可审计可恢复
- **状态序列化**: OpenAI Agents SDK — RunState.to_json()/from_json()
- **图检查点**: LangGraph — Checkpointer 持久化完整状态

### 适合 Kuro 参考的设计
1. **核心循环**: `while stop_reason == tool_use` (Anthropic API 模式) — 最简单直接
2. **工具定义**: Go struct + 自动 schema (ADK Go 模式) — 与 Kuro Go 后端匹配
3. **流式**: SSE event stream (Anthropic/Vercel 模式)
4. **终止**: 无 tool calls + max_turns + context overflow
5. **多智能体**: 后期可参考 LangGraph subgraph / OpenAI handoff 模式
