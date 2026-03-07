# Agent Loop 实现规划（ADK 方案）

> 基于 11 个框架调研（`docs/research/00-comparison.md`），引入 Google ADK Go 作为 agent 运行时。

---

## 现状问题

1. **单轮执行**：`SendMessage` 调用 LLM → 解析 `` ```skill `` 代码块 → 执行一次 → 结束。无法完成多步任务。
2. **文本解析 skill 调用**：从 markdown 代码块提取 JSON，不可靠。
3. **无 tool_call 原生支持**：`provider.Complete()` 只返回 `Content string`，没有 tool_calls。
4. **无流式输出**：LLM 响应完全等待后才返回前端。
5. **Go 版本旧**：当前 Go 1.23，ADK 需要 Go 1.24.4+（`iter.Seq2`）。

---

## 方案：引入 ADK Go

ADK 提供完整的 agent 运行时：循环、工具、事件、session、流式。Kuro 只需写两个 adapter：

1. **OpenAI LLM adapter** — 实现 `model.LLM` 接口，对接 OpenRouter / OpenAI-compatible
2. **Skill → ADK Tool adapter** — 将现有 `skill.Skill` 包装为 ADK `tool.Tool`

ADK 原生支持 Gemini，所以 Gemini adapter 免费获得。

### 获得的能力

| 能力 | ADK 提供 | Kuro 需要写 |
|------|----------|-------------|
| Agent 循环 (while tool_call) | `Flow.Run()` | 无 |
| 流式 (`iter.Seq2`) | 内置 | 无 |
| Tool 系统 + 自动 schema | `functiontool.New()` | Skill adapter |
| Gemini 原生支持 | 内置 | 无 |
| OpenAI-compatible 支持 | 无 | OpenAI LLM adapter |
| 事件溯源 Session | `session.Service` 接口 | 可选：gitstore adapter |
| Callback (Before/After Agent/Model/Tool) | 内置 | 无 |
| 多智能体 (Sequential/Parallel/Loop) | 内置 | 后期使用 |
| HITL (RequestConfirmation) | `ToolContext.RequestConfirmation` | 对接前端 |

---

## 架构

```
internal/
├── adk/                          # 新包：ADK 集成层
│   ├── openai_llm.go             # OpenAI-compatible LLM adapter (实现 model.LLM)
│   ├── skill_tool.go             # Skill → ADK Tool adapter
│   └── runner.go                 # Runner 配置 + 便捷方法
│
├── chat/                         # 重构：使用 ADK Runner
│   └── chat.go                   # SendMessageStream → ADK agent.Run() → SSE
│
├── provider/                     # 保留：给 Settings/测试用
│   ├── provider.go               # Provider 接口 (保留兼容)
│   └── openai.go                 # 现有 Complete() 保留
│
├── skill/                        # 不动：现有 skill 体系
│   └── ...
│
└── api/                          # 新增 SSE 端点
    └── api.go
```

**关键变化：**
- `internal/provider/` 保留给非 agent 场景（Settings 测试连接等）
- `internal/adk/` 是新的 agent 层，调用 ADK 的 Runner/Agent/Flow
- `internal/chat/` 从直接调用 `provider.Complete()` 改为使用 ADK Runner

---

## Phase 0：环境准备

### 0.1 升级 Go 版本

```bash
# go.mod: go 1.23.12 → go 1.24.4+
# ADK 使用 iter.Seq2，这是 Go 1.23 引入但 1.24 稳定的特性
```

### 0.2 引入 ADK 依赖

```bash
go get google.golang.org/adk@latest
```

只引核心包，不引 Vertex AI / Cloud 等可选依赖。

---

## Phase 1：OpenAI LLM Adapter

ADK 的 `model.LLM` 接口只有两个方法：

```go
type LLM interface {
    Name() string
    GenerateContent(ctx context.Context, req *LLMRequest, opts ...LLMRequestOption) iter.Seq2[*LLMResponse, error]
}
```

### 1.1 实现

```go
// internal/adk/openai_llm.go

package adk

import (
    "context"
    "iter"
    // ADK types
    "google.golang.org/adk/api/llm"
)

// OpenAILLM 实现 model.LLM 接口，对接 OpenAI-compatible API
type OpenAILLM struct {
    baseURL string
    apiKey  string
    model   string
}

func NewOpenAILLM(baseURL, apiKey, model string) *OpenAILLM {
    return &OpenAILLM{baseURL: baseURL, apiKey: apiKey, model: model}
}

func (o *OpenAILLM) Name() string { return o.model }

func (o *OpenAILLM) GenerateContent(ctx context.Context, req *llm.LLMRequest, opts ...llm.LLMRequestOption) iter.Seq2[*llm.LLMResponse, error] {
    return func(yield func(*llm.LLMResponse, error) bool) {
        // 1. 转换 ADK LLMRequest → OpenAI chat completion 请求:
        //    - req.Contents        → messages[]
        //    - req.Tools           → tools[] (function calling)
        //    - req.SystemInstruction → system message
        //    - stream: true
        //
        // 2. POST {baseURL}/chat/completions (SSE)
        //
        // 3. 解析 SSE 事件:
        //    - delta.content         → LLMResponse{Content: [Text part], Partial: true}
        //    - delta.tool_calls      → 累积参数
        //    - finish_reason: "stop" → LLMResponse{Content: [Text part], Partial: false}
        //    - finish_reason: "tool_calls" → LLMResponse{Content: [FunctionCall parts], Partial: false}
        //    - [DONE]               → return
        //
        // 4. yield 每个 LLMResponse
    }
}
```

### 1.2 协议转换细节

```
ADK (Gemini 内部格式)                   OpenAI 格式
──────────────────                      ──────────
LLMRequest.SystemInstruction         →  messages[0]{role:"system", content:...}
LLMRequest.Contents[role:"user"]     →  messages[]{role:"user", content:...}
LLMRequest.Contents[role:"model"]    →  messages[]{role:"assistant", ...}
Content.Parts[Text]                  →  message.content (string)
Content.Parts[FunctionCall]          →  message.tool_calls[]
Content.Parts[FunctionResponse]      →  messages[]{role:"tool", tool_call_id:..., content:...}
LLMRequest.Tools[].FunctionDeclarations → tools[]{type:"function", function:{name,description,parameters}}

OpenAI 响应                              ADK
──────────                              ──────────────────
delta.content                        →  LLMResponse{Parts:[Text], Partial:true}
finish_reason:"stop"                 →  LLMResponse{Parts:[Text], Partial:false}
delta.tool_calls + finish:"tool_calls" → LLMResponse{Parts:[FunctionCall...], Partial:false}
```

---

## Phase 2：Skill → ADK Tool Adapter

### 2.1 将现有 Skill 包装为 ADK Tool

```go
// internal/adk/skill_tool.go

package adk

import (
    "context"
    "encoding/json"

    "google.golang.org/adk/tool"
    "google.golang.org/adk/tool/functiontool"
    "github.com/neomody77/kuro/internal/skill"
)

// SkillToADKTools 将 Kuro skill registry 中的所有 skill 转换为 ADK tools
func SkillToADKTools(registry *skill.Registry) []tool.Tool {
    var tools []tool.Tool
    for _, sk := range registry.List() {
        t := newSkillTool(sk, registry)
        tools = append(tools, t)
    }
    return tools
}

// newSkillTool 用 functiontool.New 创建 ADK tool
// 输入 schema 从 skill.Inputs 自动生成
func newSkillTool(sk skill.SkillInfo, registry *skill.Registry) tool.Tool {
    // 方案 A: 用 functiontool.New + 动态 handler
    //   需要为每个 skill 构造一个 typed Go function
    //
    // 方案 B: 实现 tool.Tool 接口 (更灵活)
    return &skillToolWrapper{
        name:        sk.Name,
        description: sk.Description,
        inputs:      sk.Inputs,
        registry:    registry,
    }
}

type skillToolWrapper struct {
    name        string
    description string
    inputs      []skill.SkillParam
    registry    *skill.Registry
}

// 实现 ADK tool.Tool 接口
func (t *skillToolWrapper) Name() string        { return t.name }
func (t *skillToolWrapper) Description() string { return t.description }

func (t *skillToolWrapper) InputSchema() map[string]any {
    props := map[string]any{}
    required := []string{}
    for _, p := range t.inputs {
        props[p.Name] = map[string]any{"type": "string", "description": p.Name}
        if p.Required {
            required = append(required, p.Name)
        }
    }
    return map[string]any{
        "type":       "object",
        "properties": props,
        "required":   required,
    }
}

func (t *skillToolWrapper) Execute(ctx context.Context, args map[string]any) (any, error) {
    return t.registry.Execute(ctx, t.name, args)
}
```

### 2.2 Destructive Tool 处理

ADK 提供 `ToolContext.RequestConfirmation()` 用于 HITL：

```go
func (t *skillToolWrapper) Execute(ctx *tool.Context, args map[string]any) (any, error) {
    // 检查是否需要确认
    if isDestructive(t.name, args) {
        confirmed, err := ctx.RequestConfirmation(
            fmt.Sprintf("Execute %s with %v?", t.name, args),
        )
        if err != nil || !confirmed {
            return map[string]any{"status": "cancelled"}, nil
        }
    }
    return t.registry.Execute(ctx, t.name, args)
}
```

---

## Phase 3：Runner 配置 + Chat 集成

### 3.1 ADK Runner 初始化

```go
// internal/adk/runner.go

package adk

import (
    "google.golang.org/adk/agent"
    "google.golang.org/adk/runner"
    "google.golang.org/adk/session"
    "github.com/neomody77/kuro/internal/skill"
)

// NewKuroAgent 创建 Kuro 的 ADK agent
func NewKuroAgent(llm model.LLM, registry *skill.Registry, systemPrompt string) agent.Agent {
    tools := SkillToADKTools(registry)

    return &agent.LLMAgent{
        AgentName:   "kuro",
        Description: "Kuro personal AI assistant",
        Model:       llm,
        Instruction: systemPrompt,
        Tools:       tools,
        // Callbacks (可选):
        // BeforeModelCallback: logModelCall,
        // AfterToolCallback:   logToolResult,
    }
}

// NewRunner 创建 ADK runner
func NewRunner(agent agent.Agent) *runner.Runner {
    return runner.New(runner.Config{
        Agent:          agent,
        SessionService: session.NewInMemorySessionService(), // 先用内存，后期换 gitstore
        // ArtifactService: ...,
    })
}
```

### 3.2 Chat SendMessageStream

```go
// internal/chat/chat.go — 新增方法

func (s *Service) SendMessageStream(ctx context.Context, userID, sessionID, content string) iter.Seq2[*session.Event, error] {
    // 1. 获取或创建 ADK session
    // 2. 构建 user message content
    // 3. 调用 runner.Run(ctx, sessionID, content)
    // 4. 返回 iter.Seq2 → 调用方消费事件推 SSE
    //
    // ADK Runner.Run() 内部:
    //   - 加载 session
    //   - 追加 user message
    //   - agent.Run() → Flow 循环:
    //       LLM call → tool calls → tool execute → LLM call → ...
    //   - 每个 event yield 给消费方
    //   - 持久化 events 到 session
}
```

### 3.3 API 层 SSE

```go
// internal/api/api.go — 新增 SSE 端点

// POST /api/chat/{sessionID}/stream
// Body: {"content": "..."}
func (h *Handler) handleChatStream(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, _ := w.(http.Flusher)

    events := chatService.SendMessageStream(ctx, userID, sessionID, content)
    for ev, err := range events {
        if err != nil {
            fmt.Fprintf(w, "data: {\"type\":\"error\",\"error\":%q}\n\n", err.Error())
            flusher.Flush()
            return
        }

        // 转换 ADK session.Event → SSE JSON
        sseData := eventToSSE(ev)
        fmt.Fprintf(w, "data: %s\n\n", sseData)
        flusher.Flush()
    }

    fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
    flusher.Flush()
}

// eventToSSE 将 ADK event 转换为前端需要的 JSON 格式
func eventToSSE(ev *session.Event) string {
    // ev.Content.Parts 可能包含:
    //   - Text part      → {"type":"text_delta", "text":"..."}
    //   - FunctionCall    → {"type":"tool_call", "tool_name":"...", "tool_input":{...}}
    //   - FunctionResponse → {"type":"tool_result", "tool_name":"...", "output":{...}}
    // ev.Partial == true  → 流式增量
    // ev.Partial == false → 最终结果
}
```

---

## Phase 4：前端适配

### 4.1 SSE Hook

```typescript
// ui/src/hooks/useChatStream.ts

function useChatStream(sessionId: string) {
    const sendMessage = async (content: string) => {
        const response = await fetch(`/api/chat/${sessionId}/stream`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ content }),
        });

        const reader = response.body!.getReader();
        const decoder = new TextDecoder();

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const lines = decoder.decode(value).split('\n');
            for (const line of lines) {
                if (!line.startsWith('data: ')) continue;
                const event = JSON.parse(line.slice(6));

                switch (event.type) {
                    case 'text_delta':  appendText(event.text); break;
                    case 'tool_call':   addToolCallCard(event); break;
                    case 'tool_result': updateToolCallCard(event); break;
                    case 'done':        finalizeMessage(); break;
                    case 'error':       showError(event.error); break;
                }
            }
        }
    };
}
```

### 4.2 Tool Call 卡片

- 默认折叠：`[📎 credential:list] ✓`
- 展开：显示输入参数 + JSON 输出
- Destructive：`[⚠️ shell] 确认 / 取消`

---

## Phase 5：Session 持久化（可选）

### 5.1 GitStore Session Adapter

```go
// internal/adk/gitstore_session.go

// 实现 ADK session.Service 接口，底层对接 Kuro gitstore
type GitStoreSessionService struct {
    store *gitstore.Store
}

func (s *GitStoreSessionService) CreateSession(ctx context.Context, opts ...session.Option) (*session.Session, error) { ... }
func (s *GitStoreSessionService) GetSession(ctx context.Context, id string) (*session.Session, error) { ... }
func (s *GitStoreSessionService) ListSessions(ctx context.Context) ([]*session.Session, error) { ... }
func (s *GitStoreSessionService) DeleteSession(ctx context.Context, id string) error { ... }
```

先用 `session.NewInMemorySessionService()`，后期再实现 gitstore adapter。现有 JSONL 持久化可以并行保留。

---

## 实施顺序

| Phase | 内容 | 改动范围 |
|-------|------|----------|
| **0** | Go 1.24+ 升级 + `go get adk` | go.mod |
| **1** | OpenAI LLM adapter | `internal/adk/openai_llm.go` (~200 行) |
| **2** | Skill → ADK Tool adapter | `internal/adk/skill_tool.go` (~100 行) |
| **3** | Runner + Chat 集成 + SSE API | `internal/adk/runner.go` + `chat/` 重构 + `api/` 新端点 |
| **4** | 前端 SSE + tool call UI | `ui/src/hooks/` + `ui/src/pages/Chat.tsx` |
| **5** | GitStore session adapter（可选） | `internal/adk/gitstore_session.go` |

**Phase 0-3 是核心**，完成后 Kuro 拥有：
- 多轮 agent loop（LLM ↔ tool 自动循环）
- 原生 function calling（不再解析 markdown）
- SSE 流式输出
- OpenAI-compatible + Gemini 双 provider
- HITL destructive tool 确认
- 未来可直接使用 ADK 多智能体编排

---

## 不做的事

- 不自己写 agent 循环（用 ADK Flow）
- 不自己写 tool schema 生成（用 ADK functiontool）
- 不自己写 Gemini adapter（ADK 内置）
- 不改 skill 注册机制（只加 adapter 层）
- 不做多智能体编排（ADK 支持但暂不使用）
- 不做 request processor pipeline（ADK 内部处理）

---

## 迁移策略

1. **向后兼容**：保留现有 `provider.Complete()` + `chat.SendMessage()`，新增 stream 路径
2. **渐进替换**：前端切换到 SSE 后再移除旧 `SendMessage` + `parseAIResponse`
3. **Skill 不动**：现有 skill 代码零修改，通过 `skillToolWrapper` 桥接
4. **Session 并行**：ADK InMemory session + 现有 JSONL 持久化并存，后期统一

---

## 风险和应对

| 风险 | 应对 |
|------|------|
| ADK 要求 Go 1.24.4+ | 升级 Go，CI/CD 同步更新 |
| ADK v0.2 API 不稳定 | Kuro 自己也是 v0，adapter 层隔离变化 |
| OpenAI adapter 转换复杂 | ADK 的 LLMRequest 格式接近 Gemini，需要仔细映射。社区可能已有参考实现 |
| ADK 依赖体积大 | 只引核心包，`go mod tidy` 清理不用的 |
| RequestConfirmation 集成 | 需要理解 ADK 的 HITL 机制如何与 SSE 配合 |
