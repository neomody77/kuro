# Google ADK Go Agent Loop

## Overview
- Go framework (`google.golang.org/adk`), v0.2.0+, Go 1.24.4+
- Optimized for Gemini, model-agnostic via `model.LLM` interface
- Iterator-based streaming (`iter.Seq2`), event-sourced sessions

## Core Loop (`internal/llminternal/base_flow.go`)
```go
func (f *Flow) Run(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
    return func(yield func(*session.Event, error) bool) {
        for {
            for ev, err := range f.runOneStep(ctx) {
                if !yield(ev, nil) { return }
                lastEvent = ev
            }
            if lastEvent == nil || lastEvent.IsFinalResponse() {
                return  // STOP
            }
        }
    }
}
```

Each step: Preprocess (13 request processors) -> Call LLM -> Postprocess -> Yield response -> Handle function calls -> Handle agent transfer -> Loop if not final.

`IsFinalResponse()`: no FunctionCall parts, no FunctionResponse parts, not Partial.

## Tool System
```go
// Generic function tool:
stockTool, _ := functiontool.New(functiontool.Config{
    Name: "get_stock_price", Description: "...",
}, getStockPrice)  // typed Go function with args/results structs

// Agent as tool:
agenttool.New(searchAgent, nil)

// MCP integration:
mcptoolset.New(mcptoolset.Config{Transport: &mcp.CommandTransport{...}})
```
- JSON schema auto-inferred from Go struct tags
- Tool Context provides: FunctionCallID, Actions (state, transfer, escalate), State, SearchMemory, RequestConfirmation (HITL)

## Runner / Session / Agent
- **Agent interface**: `Name()`, `Description()`, `Run(InvocationContext) iter.Seq2`, `SubAgents()`
- **Runner**: top-level executor, owns session/artifact/memory services
- **Session**: ID, State (app:/user:/temp: scoped), Events (append-only)
- Runner.Run(): load session -> find agent -> append user msg -> agent.Run() -> persist events -> yield

## Streaming
- `RunConfig.StreamingMode`: None or SSE
- LLM returns `iter.Seq2[*LLMResponse, error]` with `Partial: true` chunks
- Partial events yielded to consumer but NOT persisted (only final event persisted)

## Multi-Agent Orchestration
| Pattern | Mechanism |
|---|---|
| SequentialAgent | Sub-agents run one after another |
| ParallelAgent | Sub-agents run concurrently (errgroup), isolated branches |
| LoopAgent | Repeat until MaxIterations or Escalate |
| Agent Transfer | LLM calls auto-injected `transfer_to_agent` tool |
| AgentTool | Sub-agent runs as tool call, parent keeps control |

## Stop Conditions
| Mechanism | Effect |
|---|---|
| Final response (no function calls) | Loop exits |
| EndInvocation() | Immediately stops |
| Escalate | LoopAgent exits |
| TransferToAgent | Hands off to another agent |
| MaxIterations | LoopAgent counter |
| BeforeAgentCallback returns content | Agent skipped |
| BeforeModelCallback returns response | LLM call skipped |

## Callback System
- Agent: BeforeAgent, AfterAgent
- Model: BeforeModel, AfterModel, OnModelError
- Tool: BeforeTool, AfterTool, OnToolError
- If callback returns non-nil, remaining callbacks and operation skipped

## Key Design
- Iterator-based (`iter.Seq2`) — pull-based, unifies streaming/non-streaming/multi-agent
- Event-sourced sessions — append-only, auditable, resumable
- Request processor pipeline (13 processors) for building LLM requests
- Composition: workflow agents compose sub-agents, LLM agents wrap Flow loop
- Model-agnostic: `model.LLM` interface is just `Name()` + `GenerateContent()`
