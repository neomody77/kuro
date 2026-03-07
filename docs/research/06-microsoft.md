# Microsoft Agent Frameworks

## 1. Semantic Kernel

### Core Loop (Auto Function Calling)
Located in chat completion client base class. Bounded for-loop:

```python
for request_index in range(settings.function_choice_behavior.maximum_auto_invoke_attempts):
    completions = await self._inner_get_chat_message_contents(chat_history, settings)
    function_calls = [item for item in completions[0].items if isinstance(item, FunctionCallContent)]
    if len(function_calls) == 0:
        return completions                    # no tool calls = done
    chat_history.add_message(completions[0])  # append assistant msg
    results = await asyncio.gather(...)       # invoke all functions in PARALLEL
    if any(result.terminate for result in results):
        return merge_function_results(...)    # filter-based early termination
    # append FunctionResultContent to chat_history, loop back
# after max attempts: call LLM with function calling disabled
```

### Tool Registration
- Plugins with `@kernel_function` decorator (Python) / `[KernelFunction]` attribute (C#)
- Serialized to OpenAI-compatible JSON schema
- `FunctionChoiceBehavior`: Auto (5 attempts), Required (1), NoneInvoke (0)

### Stop Conditions
1. No tool calls returned
2. `maximum_auto_invoke_attempts` exhausted (default 5)
3. Filter termination (`result.terminate = True`)
4. Required behavior auto-disables after first call

### Manual Mode
`auto_invoke=False` returns tool calls to caller, who implements their own loop.

### Multi-Agent (5 patterns)
Sequential, Concurrent, Handoff, Group Chat, Magentic
All use `InProcessRuntime` for communication.

---

## 2. AutoGen

### Core Loop (AssistantAgent)
Loop encapsulated inside the agent:

```python
# Inside on_messages_stream():
for iteration in range(max_tool_iterations):  # default: 10
    response = await call_llm()
    if response has text only:
        return TextMessage
    if response has function calls:
        results = await asyncio.gather(*tool_calls)  # PARALLEL
        if any HandoffMessage: return HandoffMessage
        append results to context
        if final iteration and reflect_on_tool_use:
            call LLM with tool_choice="none"
```

### Tool Registration
Plain Python functions passed to constructor:
```python
agent = AssistantAgent(tools=[web_search], ...)
```
Also: AgentTool (agent-as-tool), McpWorkbench (MCP integration)

### Stop Conditions
Single-agent: no tool calls, max_tool_iterations (10), handoff

Team-level: 11 built-in conditions, composable with & / |:
- MaxMessageTermination, TextMentionTermination, TokenUsageTermination
- TimeoutTermination, HandoffTermination, ExternalTermination, etc.

### Multi-Agent (4 patterns)
- RoundRobinGroupChat: fixed sequential order
- SelectorGroupChat: LLM picks next speaker
- Swarm: explicit handoff via HandoffMessage
- MagenticOneGroupChat: research-inspired generalist

### Streaming
- Agent: `run_stream()` yields events incrementally
- Model: `model_client_stream=True` for token-level streaming
- Team: `team.run_stream()` yields from all agents

---

## Comparison

| Aspect | Semantic Kernel | AutoGen |
|---|---|---|
| Loop location | Chat completion client | Inside the agent |
| Default max iterations | 5 | 10 |
| Tool execution | Parallel (asyncio.gather) | Parallel (asyncio.gather) |
| Manual mode | auto_invoke=False | Not primary pattern |
| Multi-agent | 5 orchestration patterns | 4 team types |
| Languages | C#, Python, Java | Python (primary) |
