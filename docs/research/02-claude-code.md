# Claude Code Agent Loop

## Overview
- Anthropic's official CLI agent
- Loop: `while (tool_call) -> execute -> feed results -> repeat`
- Uses Claude API's `tool_use` / `tool_result` content blocks

## Core Loop
```
while true:
  response = claude.complete(messages, tools)
  if response.stop_reason == "tool_use":
    execute tools
    append tool_result to messages
    continue
  elif response.stop_reason == "end_turn":
    break  # done
```

## stop_reason Values
| stop_reason | Agent behavior |
|-------------|---------------|
| `tool_use` | Execute tool, continue loop |
| `end_turn` | Loop terminates, final result |
| `max_tokens` | Truncated, loop terminates |
| `pause_turn` | Server sampling limit hit, continue |
| `refusal` | Safety refusal, terminates |

## Turn Limits
- `max_turns` / `maxTurns`: cap tool-use round trips
- `max_budget_usd` / `maxBudgetUsd`: cost cap
- Without limits, runs until Claude finishes

## Tool Execution
- Read-only tools (Read, Glob, Grep): **concurrent**
- State-modifying tools (Edit, Write, Bash): **sequential**
- Custom tools default sequential; mark `readOnly` for parallel

## Streaming
Message types yielded during loop:
1. **SystemMessage** -- init, compact_boundary
2. **AssistantMessage** -- after each Claude response
3. **UserMessage** -- after tool execution (tool_result blocks)
4. **StreamEvent** -- raw text deltas, tool input chunks
5. **ResultMessage** -- final result with usage/cost/session

## Permission System
| Mode | Behavior |
|------|----------|
| `default` | Uncovered tools trigger approval |
| `acceptEdits` | Auto-approve edits, ask for others |
| `plan` | No execution, plan only |
| `dontAsk` | Never prompt, pre-approved only |
| `bypassPermissions` | All allowed (isolated envs) |

- `allowed_tools` / `disallowed_tools` lists
- Deny > Ask > Allow priority
- Hooks: `PreToolUse`, `PostToolUse`, `PermissionRequest`

## Hook System
- Fires at: PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest
- Returns: permissionDecision (allow/deny/ask), updatedInput, systemMessage
- Matchers: regex on tool names (e.g. `"Write|Edit"`, `"^mcp__"`)
- Operate outside context window (no token cost)

## Message Format
```json
// Assistant with tool call:
{"stop_reason": "tool_use", "content": [
  {"type": "text", "text": "..."},
  {"type": "tool_use", "id": "toolu_xxx", "name": "get_weather", "input": {...}}
]}

// User with tool result:
{"role": "user", "content": [
  {"type": "tool_result", "tool_use_id": "toolu_xxx", "content": "72F"}
]}
```
- tool_result blocks FIRST in content array
- Multiple parallel results in single message
- `is_error: true` for error results

## Sub-agents
- Spawn via `Task` tool, independent agent loop
- Fresh context (no parent history)
- Only final text returns to parent as tool result
- Hooks: SubagentStart, SubagentStop

## Context Management
- Automatic compaction when approaching limit
- SystemMessage `compact_boundary` emitted
- Configurable preservation instructions in CLAUDE.md
