# OpenAI Agents SDK

## Overview
- Python, async-first, pip install `openai-agents`
- Default model: gpt-4.1
- Core: `Runner` class with `run()`, `run_sync()`, `run_streamed()`

## Core Loop (Runner.run -> AgentRunner.run)
```python
while True:
    if current_turn > max_turns:
        raise MaxTurnsExceeded  # or call error handler

    # Input guardrails (first turn only)
    # run_single_turn() -> model call + tool execution
    turn_result = await run_single_turn()

    match turn_result.next_step:
        case NextStepFinalOutput:   # done, run output guardrails, return
        case NextStepHandoff:       # switch agent, continue loop
        case NextStepRunAgain:      # tool calls executed, loop again
        case NextStepInterruption:  # needs approval, pause and return
```

State serializable via `RunState.to_json()`/`from_json()` for durable pauses.

## Tool Types
1. **Function tools**: `@function_tool` decorator, auto JSON schema from type hints
2. **Hosted tools**: WebSearchTool, FileSearchTool, CodeInterpreterTool, ImageGenerationTool
3. **Agents-as-tools**: `agent.as_tool()`, supports `needs_approval`
4. **Computer/Shell/ApplyPatch**: ComputerTool, ShellTool, ApplyPatchTool
5. **MCP tools**: HostedMCPTool

## Tool Use Behavior
- `"run_llm_again"` (default) — results go back to LLM
- `"stop_on_first_tool"` — first tool output = final result
- `StopAtTools([...])` — halt on specific tools
- `reset_tool_choice` (default True) prevents infinite loops

## Stop Conditions
1. Final output produced (no tool calls)
2. `max_turns` exceeded
3. Guardrail tripwire triggered
4. Interruption (tool needs approval)
5. `tool_use_behavior` early termination

## Streaming
`Runner.run_streamed()` returns `RunResultStreaming.stream_events()`:
- `RawResponsesStreamEvent` — token-by-token deltas
- `RunItemStreamEvent` — semantic: tool_called, tool_output, handoff, etc.
- `AgentUpdatedStreamEvent` — agent changed

Optional WebSocket transport for connection reuse.

## Handoffs
- Presented as `transfer_to_<agent_name>` tools
- `handoff()` function: tool_name_override, on_handoff callback, input_type, input_filter
- On handoff: current_agent = new_agent, reset hooks, continue loop
- `nest_handoff_history` collapses prior transcripts

## Guardrails
- **Input**: `@input_guardrail`, parallel (with model) or blocking, first agent only
- **Output**: `@output_guardrail`, after completion, last agent only
- **Tool**: `@tool_input_guardrail` / `@tool_output_guardrail`, every invocation
- Tripwire raises exception immediately

## Tracing
- Built-in, enabled by default
- Auto-instrumented: agent_span, generation_span, function_span, guardrail_span, handoff_span
- Custom: `with trace("name"):` context manager
- Sensitive data control via config
- 20+ external processors (W&B, Langfuse, LangSmith, etc.)

## Key Design
- Plain Python async, not DSL/graph
- Agent is a dataclass, tools are decorated functions
- Human-in-the-loop deeply integrated (RunState serialization)
- Sessions: SQLite, Redis, SQLAlchemy, server-managed
- ProcessedResponse categorizes model output into handoffs/functions/computer/shell/mcp
