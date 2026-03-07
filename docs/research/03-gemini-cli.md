# Gemini CLI Agent Loop

## Overview
- TypeScript/Node.js monorepo
- `@google/gemini-cli-core` (headless engine) + `@google/gemini-cli` (Ink TUI)
- **Layered async generator architecture** -- core does NOT close the loop, UI layer does

## Architecture Layers
1. **GeminiClient.sendMessageStream()** -- entry point, AsyncGenerator, hooks, MAX_TURNS=100
2. **GeminiClient.processTurn()** -- compression, overflow check, loop detection, model routing
3. **Turn.run()** -- single LLM round-trip, yields typed events
4. **GeminiChat.sendMessageStream()** -- raw API call with retry, stream validation

## Loop Closure (in UI layer)
```
submitQuery(userText)
  -> geminiClient.sendMessageStream(userText)
  -> processGeminiStreamEvents(stream)
     -> ToolCallRequest events -> collect
  -> scheduleToolCalls(toolCallRequests)
     -> CoreToolScheduler: validate -> policy -> confirm -> execute
  -> handleCompletedTools()
     -> build functionResponse Parts from results
     -> submitQuery(responseParts, {isContinuation: true})  // CLOSES THE LOOP
```

The core library yields events but **never** executes tools itself. The caller (UI hook) executes tools and feeds results back by calling `submitQuery` recursively.

## Stop Conditions
| Condition | Mechanism |
|-----------|-----------|
| No tool calls in response | `handleCompletedTools` returns without `submitQuery` |
| MAX_TURNS (100) exceeded | `MaxSessionTurns` event |
| User cancellation (Escape) | AbortController |
| Loop detected | `LoopDetectionService`, offers user choice |
| Context window overflow | `ContextWindowWillOverflow` event |
| All tools cancelled | No `submitQuery` call |
| Hook stops execution | `AgentExecutionStopped` event |
| finishReason STOP + no tools | Next-speaker check -> "user" = end |

## Streaming
- End-to-end AsyncGenerators from API to UI
- Event types: Content, Thought, ToolCallRequest, Finished, Error
- Text streamed incrementally, split at markdown-safe boundaries
- Tool execution supports live output via callback

## Permission/Confirmation
Approval modes:
- `DEFAULT` -- ask for destructive ops
- `AUTO_EDIT` -- auto-approve edits, ask for shell
- `YOLO` -- auto-approve everything
- `PLAN` -- plan only, no execution

Policy engine: rules match tool name, MCP server, args, annotations -> ALLOW/DENY/ASK_USER

User responses: ProceedOnce, ProceedAlways, ProceedAlwaysAndSave, ModifyWithEditor, Cancel

## Hook System
6 hook points: BeforeAgent, AfterAgent, BeforeModel, AfterModel, BeforeTool, AfterTool

## Message Format
- Gemini API format with `functionCall` / `functionResponse`
- Tool call: `{name, args, id}` wrapped as ToolCallRequestInfo
- Tool result: `{llmContent: Part[], returnDisplay, error?, tailToolCallRequest?}`
- Results sent as `functionResponse` Parts in user message

## Key Design
- Generator-based streaming (no buffering)
- Core/UI separation (core is headless, reusable)
- Sequential tool execution via CoreToolScheduler queue
- Recursive loop closure via submitQuery
- 6-point hook system for interception
