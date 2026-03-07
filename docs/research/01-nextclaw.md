# NextClaw Agent Loop

## Overview
- TypeScript, monorepo (`packages/nextclaw-core/`)
- Core loop in `packages/nextclaw-core/src/agent/loop.ts`, `AgentLoop` class
- Classic **while-loop-with-tool-calls** pattern

## Core Loop
```typescript
let iteration = 0;
const maxIterations = this.options.maxIterations ?? 20; // config default: 1000

while (iteration < maxIterations) {
  throwIfAborted(options?.abortSignal);
  iteration += 1;
  this.pruneMessagesForInputBudget(messages);

  const response = await this.chatWithOptionalStreaming({
    messages, tools: this.tools.getDefinitions(), model: runtimeModel, signal
  }, options?.onAssistantDelta);

  if (containsSilentReplyMarker(response.content)) return null;

  if (response.toolCalls.length) {
    // Append assistant message + tool calls
    // Execute each tool call SEQUENTIALLY
    for (const call of response.toolCalls) {
      const result = await this.tools.execute(call.name, call.arguments, call.id);
      this.context.addToolResult(messages, call.id, call.name, result);
    }
    // Loop continues -> next LLM call
  } else {
    finalContent = response.content;
    break;  // EXIT: no tool calls
  }
}
```

## Entry Points
- `run()` -- long-running loop from MessageBus (channels)
- `handleInbound()` -- single-shot inbound
- `processDirect()` -- programmatic (CLI, UI chat)
- All funnel into `processMessage()` -> while loop

## Tool System
- Registry: `Map<string, Tool>`, each tool extends abstract `Tool` base class
- Built-in: read_file, write_file, edit_file, list_dir, exec (shell), web_search, web_fetch, message, spawn, sessions, memory, cron
- Extension tools via `ExtensionToolAdapter` (plugins)
- Tool calls executed **sequentially** (not parallel)

## Stop Conditions
1. **No tool calls** -- normal exit, LLM returns text only
2. **Max iterations** -- default 1000 (config), fallback 20 (constructor)
3. **Silent reply** -- `<noreply/>` marker in response
4. **Abort signal** -- user stop button, checked at multiple points
5. **Empty final reply** -- dropped silently

## Streaming
- Provider level: `chatStream()` yields `{type:"delta", delta}` and `{type:"done", response}`
- Text deltas streamed to UI; tool call args buffered silently
- Server: SSE with events: `ready`, `delta`, `session_event`, `final`, `error`, `done`
- UI: `useChatStreamController.ts` React hook

## Safety / Confirmation
- **No interactive confirmation** in the loop
- Shell deny patterns (rm -rf, dd, shutdown, fork bomb)
- Filesystem sandboxing (restrict to workspace)
- System prompt advisory guidelines
- Abort/stop via AbortController

## Message Format
- OpenAI chat completions format throughout
- Assistant: `{role:"assistant", content, tool_calls: [{id, type:"function", function:{name, arguments}}]}`
- Tool result: `{role:"tool", tool_call_id, name, content}`
- Also supports OpenAI Responses API with format conversion

## Key Design
- Sequential tool execution (not concurrent)
- Input budget pruner trims history to fit context window
- Sub-agent support via `spawn` tool (independent AgentLoop instances)
- Config-driven iteration limit (1000 default)
