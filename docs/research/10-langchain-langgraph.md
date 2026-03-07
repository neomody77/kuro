# LangChain / LangGraph Agent Loop

## Overview
- Python, `langchain` + `langgraph` packages
- Legacy: `AgentExecutor` (while loop), Current: `create_agent` / `create_react_agent` (graph-based)
- ReAct (Reasoning + Acting) pattern: LLM -> tool calls -> observation -> loop
- Graph-based runtime via LangGraph `StateGraph`

## LangChain AgentExecutor (Legacy)

### Core Loop
```python
# Simplified AgentExecutor._call():
iterations = 0
while self._should_continue(iterations, time_elapsed):
    next_step = agent.plan(intermediate_steps, **inputs)  # LLM call
    if isinstance(next_step, AgentFinish):
        return next_step.return_values               # DONE
    # next_step is AgentAction(s)
    for action in next_step:
        observation = tool_map[action.tool].run(action.tool_input)
        intermediate_steps.append((action, observation))
    iterations += 1
# max_iterations reached: return early or raise
```

### Stop Conditions
1. `AgentFinish` returned (no tool calls)
2. `max_iterations` reached (default: 15)
3. `max_execution_time` exceeded
4. `early_stopping_method`: "force" (return directly) or "generate" (one more LLM call without tools)

## LangGraph create_agent (Current)

### Core Loop
Graph-based: nodes + conditional edges replace the while loop.

```
┌──────────┐    tool_calls?    ┌───────────┐
│  agent   │──── YES ─────────▶│   tools   │
│ (LLM)   │                   │ (ToolNode)│
│          │◀──────────────────│           │
└──────────┘                   └───────────┘
     │
     │ NO tool_calls
     ▼
  __end__
```

```python
from langchain.agents import create_agent

agent = create_agent("openai:gpt-4", tools=[search, weather])

result = agent.invoke({
    "messages": [{"role": "user", "content": "What's the weather?"}]
})
```

Internally builds a `StateGraph` with:
- `agent` node: calls LLM with bound tools
- `tools` node: executes tool calls (parallel by default)
- `tools_condition`: routes to "tools" if `tool_calls` present, else `__end__`

### Tool System
```python
from langchain.tools import tool

@tool
def search(query: str) -> str:
    """Search for information."""
    return f"Results for: {query}"

# Tools bound to model automatically
agent = create_agent(model, tools=[search])
```

- `@tool` decorator: auto JSON schema from type hints + docstring
- `BaseTool` class: for complex tools with custom validation
- `ToolNode`: executes all tool calls from an AIMessage, returns `ToolMessage` per call
- Dynamic tools via middleware: filter/add tools per step based on state

### Middleware System (New in create_agent)
```python
from langchain.agents.middleware import wrap_model_call, ModelRequest

@wrap_model_call
def dynamic_selection(request: ModelRequest, handler):
    if len(request.state["messages"]) > 10:
        request = request.override(model=advanced_model)
    return handler(request)

@wrap_tool_call
def handle_errors(request, handler):
    try:
        return handler(request)
    except Exception as e:
        return ToolMessage(content=f"Error: {e}", tool_call_id=request.tool_call["id"])

agent = create_agent(model, tools=tools, middleware=[dynamic_selection, handle_errors])
```

### Stop Conditions
1. LLM response has no `tool_calls` → route to `__end__`
2. `remaining_steps` < 2 with tool calls → returns "Sorry, need more steps to process this request."
3. `recursion_limit` on the graph (configurable)
4. `return_direct=True` on a tool → tool output becomes final response
5. `interrupt_before` / `interrupt_after` for human-in-the-loop pauses

### Streaming
```python
# Stream intermediate steps
for chunk in agent.stream(
    {"messages": [{"role": "user", "content": "Search news"}]},
    stream_mode="values"   # or "updates", "messages"
):
    print(chunk["messages"][-1])

# stream_mode options:
# "values" — full state after each step
# "updates" — only changed keys per step
# "messages" — token-level LLM output streaming
```

### Multi-Agent
```python
# Named agents as subgraphs
research = create_agent(model, tools=[search], name="researcher")
writer = create_agent(model, tools=[write], name="writer")

# Compose in a parent graph
from langgraph.graph import StateGraph
graph = StateGraph(State)
graph.add_node("researcher", research)
graph.add_node("writer", writer)
graph.add_edge("researcher", "writer")
```

Patterns:
- **Subgraphs**: agent as a node in a larger graph
- **Supervisor**: one LLM routes to worker agents
- **Handoff**: agents transfer control via special tools
- **Hierarchical**: nested supervisor patterns

### Persistence & Checkpointing
```python
from langgraph.checkpoint.memory import MemorySaver

agent = create_agent(model, tools=tools, checkpointer=MemorySaver())
# Each invoke with thread_id resumes from checkpoint
result = agent.invoke(input, config={"configurable": {"thread_id": "123"}})
```

## Key Design
- Graph-based: nodes + edges replace imperative while loop, enabling visualization and composition
- `tools_condition` is the routing function: has tool_calls → "tools", else → `__end__`
- Middleware system for cross-cutting concerns (model selection, tool filtering, error handling)
- State is a TypedDict with `messages` list (append-only by default via `Annotated[list, add_messages]`)
- Checkpointing enables persistence, time-travel, and human-in-the-loop
- `create_agent` replaces both legacy `AgentExecutor` and `create_react_agent`
