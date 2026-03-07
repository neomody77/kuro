# OpenClaw Agent Loop

## Overview
- Built on `@mariozechner/pi-agent-core` (external npm package)
- Three layers: pi-agent-core (generic loop) -> OpenClaw runner (retry/failover) -> tools & policies
- Double-nested while loop

## Core Loop (pi-agent-core)
```javascript
async function runLoop(currentContext, newMessages, config, signal, stream) {
    let pendingMessages = await config.getSteeringMessages?.() || [];
    while (true) {                                    // OUTER: follow-up loop
        let hasMoreToolCalls = true;
        while (hasMoreToolCalls || pendingMessages.length > 0) {  // INNER: tool loop
            // Inject steering messages
            // Call LLM
            const message = await streamAssistantResponse(currentContext, config, signal, stream);
            if (message.stopReason === "error" || message.stopReason === "aborted") return;

            const toolCalls = message.content.filter(c => c.type === "toolCall");
            hasMoreToolCalls = toolCalls.length > 0;

            if (hasMoreToolCalls) {
                const results = await executeToolCalls(tools, message, signal, stream);
                // Push results into context
            }
            pendingMessages = await config.getSteeringMessages?.() || [];
        }
        // Check follow-up messages
        const followUp = await config.getFollowUpMessages?.() || [];
        if (followUp.length > 0) { pendingMessages = followUp; continue; }
        break;
    }
}
```

## Tool Execution
- **Sequential** (one at a time, not parallel)
- Between each tool: poll for steering messages -> if user interrupts, skip remaining tools
- Errors caught and converted to `isError: true` tool results
- Partial results streamed via callback

## Stop Conditions
1. No tool calls + no steering messages + no follow-up messages
2. stopReason === "error" or "aborted"
3. OpenClaw retry layer: 32-160 iterations max (scales with auth profiles)
4. Context overflow -> compact (3 attempts) -> truncate tool results

## Streaming Events
| Event | When |
|---|---|
| agent_start/end | Agent lifecycle |
| turn_start/end | Each LLM + tool cycle |
| message_start/update/end | Message streaming (text_delta, thinking_delta, toolcall_delta) |
| tool_execution_start/update/end | Tool lifecycle |

LLM output streamed token-by-token, partial message updated in-place on context.

## Permission System (4 layers)
1. **Tool Profiles**: minimal/coding/messaging/full + allow/deny lists
2. **Exec Security**: deny/allowlist/full + ask: off/on-miss/always + safeBins
3. **Before-Tool-Call Hook**: loop detection + plugin hooks (can block/modify)
4. **Owner-only**: sensitive tools filtered for non-owner users

## Message Format
```typescript
// Tool call (in assistant content):
{ type: "toolCall", id, name, arguments: Record<string,any> }

// Tool result:
{ role: "toolResult", toolCallId, toolName, content: (Text|Image)[], isError, details }
```
- `details` field is app-specific metadata, stripped before sending to LLM
- StopReason: "stop" | "length" | "toolUse" | "error" | "aborted"

## Key Design
- Double-nested loop (inner: tools, outer: follow-ups)
- Steering messages allow user interruption between tool calls
- External pi-agent-core package provides transport-agnostic loop
- Retry/failover layer with auth rotation and context compaction
