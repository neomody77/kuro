# Anthropic Claude API Agent Loop

## Overview

Anthropic provides two main approaches for building agents with Claude:

1. **Raw Messages API** -- The foundational HTTP API where you manually manage the tool-use loop. You send messages with tool definitions, check `stop_reason`, execute tools client-side, and send results back. This is the approach used across all Anthropic SDKs (Python, TypeScript, Go, Java, C#, PHP, Ruby).

2. **Claude Agent SDK** (`claude-agent-sdk`) -- A higher-level Python SDK that wraps the Claude Code CLI. It provides `query()` for one-shot interactions and `ClaudeSDKClient` for bidirectional conversations, with built-in support for tools (Read, Write, Bash), custom MCP servers, and hooks. This is **not** a thin wrapper around the Messages API; it orchestrates the full Claude Code agent.

3. **Tool Runner Helper** (`client.beta.messages.tool_runner`) -- A beta convenience layer in the official SDKs that automates the agentic loop: you define tools as decorated functions, and the runner handles execution, result formatting, and re-prompting until Claude is done.

---

## Core Loop (Raw Messages API)

### Tool Use Flow

The agentic loop follows this cycle:

```
User prompt --> Claude (with tools) --> stop_reason?
  |                                        |
  |   stop_reason == "tool_use"            |   stop_reason == "end_turn"
  |   <------------------------------------+----> Done, return response
  |
  v
Extract tool_use blocks --> Execute tools locally --> Send tool_result blocks
  |
  +-- Loop back to Claude with updated messages
```

Steps:
1. Send a request to `/v1/messages` with `tools` array and user `messages`.
2. Claude responds. If `stop_reason == "tool_use"`, the `content` array contains one or more `tool_use` blocks.
3. Extract each tool call's `name`, `id`, and `input`. Execute the tool on your side.
4. Append the assistant's response to the message history, then append a `user` message containing `tool_result` blocks (one per tool call).
5. Send the updated messages back to Claude. Repeat until `stop_reason != "tool_use"`.

### Python Implementation

```python
import anthropic

client = anthropic.Anthropic()

tools = [
    {
        "name": "get_weather",
        "description": "Get the current weather in a given location",
        "input_schema": {
            "type": "object",
            "properties": {
                "location": {
                    "type": "string",
                    "description": "The city and state, e.g. San Francisco, CA",
                }
            },
            "required": ["location"],
        },
    }
]

messages = [{"role": "user", "content": "What's the weather in San Francisco?"}]

while True:
    response = client.messages.create(
        model="claude-sonnet-4-20250514",
        max_tokens=1024,
        tools=tools,
        messages=messages,
    )

    # Check if Claude wants to use tools
    if response.stop_reason != "tool_use":
        break

    # Append assistant response to history
    messages.append({"role": "assistant", "content": response.content})

    # Execute tools and collect results
    tool_results = []
    for block in response.content:
        if block.type == "tool_use":
            result = execute_tool(block.name, block.input)
            tool_results.append({
                "type": "tool_result",
                "tool_use_id": block.id,
                "content": result,
            })

    # Send results back to Claude
    messages.append({"role": "user", "content": tool_results})

# Final response
print(response.content[0].text)
```

### Go Implementation

```go
client := anthropic.NewClient()

tools := []anthropic.ToolUnionParam{
    {OfTool: &anthropic.ToolParam{
        Name:        "get_weather",
        Description: anthropic.String("Get the current weather in a given location"),
        InputSchema: anthropic.ToolInputSchemaParam{
            Properties: map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "The city and state, e.g. San Francisco, CA",
                },
            },
            Required: []string{"location"},
        },
    }},
}

messages := []anthropic.MessageParam{
    anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in SF?")),
}

for {
    msg, _ := client.Messages.New(context.Background(), anthropic.MessageNewParams{
        Model:     anthropic.ModelClaudeSonnet4_20250514,
        MaxTokens: 1024,
        Tools:     tools,
        Messages:  messages,
    })

    if msg.StopReason != anthropic.MessageStopReasonToolUse {
        break
    }

    // Append assistant content, execute tools, append tool_result
    // ... (same pattern as Python)
}
```

### Tool Definition

Each tool is a JSON object with three fields:

```json
{
  "name": "get_weather",
  "description": "Get the current weather in a given location",
  "input_schema": {
    "type": "object",
    "properties": {
      "location": {
        "type": "string",
        "description": "The city and state, e.g. San Francisco, CA"
      },
      "unit": {
        "type": "string",
        "enum": ["celsius", "fahrenheit"],
        "description": "The unit of temperature"
      }
    },
    "required": ["location"]
  }
}
```

- **`name`**: String, regex `[a-zA-Z0-9_-]{1,64}`. Must be unique in the tools array.
- **`description`**: Detailed description of what the tool does, when to use it, and what it returns. 3-4+ sentences recommended.
- **`input_schema`**: Standard JSON Schema object. The `type` must be `"object"`.
- **`strict`** (optional): Set to `true` for guaranteed schema conformance via Structured Outputs.

### Tool Use Response Block

When Claude wants to call a tool, the response contains a `tool_use` content block:

```json
{
  "type": "tool_use",
  "id": "toolu_01A09q90qw90lq917835lq9",
  "name": "get_weather",
  "input": { "location": "San Francisco, CA", "unit": "celsius" }
}
```

- **`id`**: Unique identifier for this tool call. Must be echoed back in the `tool_result`.
- **`name`**: The tool name from your definitions.
- **`input`**: JSON object matching the tool's `input_schema`.

A single response can contain multiple `tool_use` blocks (parallel tool use) interspersed with `text` blocks.

### Tool Result Format

Tool results are sent as content blocks in a `user` message:

```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_01A09q90qw90lq917835lq9",
      "content": "15 degrees celsius, sunny"
    }
  ]
}
```

**Critical formatting rule**: `tool_result` blocks must come FIRST in the content array. Placing `text` blocks before `tool_result` blocks causes a 400 error.

```json
// CORRECT - tool_result first, text after
{
  "role": "user",
  "content": [
    {"type": "tool_result", "tool_use_id": "toolu_01", "content": "result"},
    {"type": "text", "text": "Additional context"}
  ]
}

// WRONG - text before tool_result causes 400 error
{
  "role": "user",
  "content": [
    {"type": "text", "text": "Here are results:"},
    {"type": "tool_result", "tool_use_id": "toolu_01", "content": "result"}
  ]
}
```

Rich tool results can include images and documents:

```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01",
  "content": [
    {"type": "text", "text": "Chart analysis:"},
    {"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "..."}}
  ]
}
```

Error results use `is_error`:

```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01",
  "content": "Connection timeout after 30s",
  "is_error": true
}
```

---

## Stop Conditions

The `stop_reason` field indicates why Claude stopped generating:

| Value | Meaning |
|-------|---------|
| `end_turn` | Claude finished its response naturally. Most common. |
| `tool_use` | Claude wants to call one or more tools. Execute them and send results back. |
| `max_tokens` | Hit the `max_tokens` limit. Response is truncated. |
| `stop_sequence` | Encountered a custom stop sequence from `stop_sequences` parameter. |
| `pause_turn` | Server-side tool loop (web search, web fetch) hit iteration limit (default 10). Send response back to continue. |
| `refusal` | Claude declined to respond due to safety concerns. |
| `model_context_window_exceeded` | Hit the model's context window limit. Available by default in Sonnet 4.5+. |

### Handling Pattern

```python
def handle_response(response):
    match response.stop_reason:
        case "tool_use":
            return handle_tool_use(response)
        case "end_turn":
            return response.content[0].text
        case "max_tokens" | "model_context_window_exceeded":
            return handle_truncation(response)
        case "pause_turn":
            return continue_conversation(response)
        case "refusal":
            return handle_refusal(response)
```

### Empty Responses with end_turn

Claude may return an empty response with `stop_reason: "end_turn"` when:
- Text blocks are added immediately after `tool_result` blocks (teaches Claude to expect user input after every tool use).
- The assistant's completed response is sent back without adding anything new.

Prevention: Never add text blocks immediately after tool results in the same user message.

---

## Streaming with Tool Use

Set `"stream": true` (raw API) or use `client.messages.stream()` (SDK) to get incremental SSE events.

### Event Flow

```
1. message_start      -- Message object with empty content
2. content_block_start -- Start of each content block (text or tool_use)
3. content_block_delta -- Incremental updates (text_delta or input_json_delta)
   ... (repeats)
4. content_block_stop  -- End of content block
   ... (blocks 2-4 repeat for each content block)
5. message_delta       -- Top-level changes (stop_reason, usage)
6. message_stop        -- Stream complete
```

Interleaved `ping` events may appear at any point.

### Event Format (SSE)

**message_start**:
```
event: message_start
data: {"type":"message_start","message":{"id":"msg_014p7gG3...","type":"message","role":"assistant","model":"claude-opus-4-6","stop_sequence":null,"usage":{"input_tokens":472,"output_tokens":2},"content":[],"stop_reason":null}}
```

**content_block_start** (text):
```
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
```

**content_block_start** (tool_use):
```
event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01T1x1fJ34qAmk2tNTrN7Up6","name":"get_weather","input":{}}}
```

**content_block_delta** (text):
```
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
```

**content_block_delta** (tool input JSON):
```
event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"location\": \"San Fra"}}
```

**content_block_stop**:
```
event: content_block_stop
data: {"type":"content_block_stop","index":1}
```

**message_delta** (contains stop_reason):
```
event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":89}}
```

**message_stop**:
```
event: message_stop
data: {"type":"message_stop"}
```

### input_json_delta Details

Tool use input is streamed as **partial JSON strings**. You accumulate the `partial_json` values and parse once you receive `content_block_stop`. Current models emit one complete key-value pair at a time, so there may be delays between streaming events while the model works.

SDKs provide helpers for parsing incremental values. Alternatively, use a library like Pydantic for partial JSON parsing.

### Streaming stop_reason

- `null` in the initial `message_start` event.
- Provided in the `message_delta` event near end of stream.
- Not provided in any other event types.

### SDK Streaming Examples

**Python**:
```python
with client.messages.stream(
    model="claude-opus-4-6",
    max_tokens=1024,
    tools=tools,
    messages=messages,
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)
```

**TypeScript**:
```typescript
const stream = client.messages.stream({
  model: "claude-opus-4-6",
  max_tokens: 1024,
  tools: tools,
  messages: messages,
});

for await (const event of stream) {
  if (event.type === "content_block_delta" && event.delta.type === "text_delta") {
    process.stdout.write(event.delta.text);
  }
}
```

**Go**:
```go
stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{...})
for stream.Next() {
    event := stream.Current()
    switch ev := event.AsAny().(type) {
    case anthropic.ContentBlockDeltaEvent:
        switch d := ev.Delta.AsAny().(type) {
        case anthropic.TextDelta:
            fmt.Print(d.Text)
        }
    }
}
```

---

## Tool Choice Control

The `tool_choice` parameter controls whether and how Claude uses tools:

| Type | Behavior |
|------|----------|
| `{"type": "auto"}` | **(Default)** Claude decides whether to use tools. May respond with text only. |
| `{"type": "any"}` | Claude **must** use at least one tool. Cannot respond with text only. |
| `{"type": "tool", "name": "get_weather"}` | Claude **must** use the specified tool. |
| `{"type": "none"}` | Claude cannot use any tools, even if they are provided. |

**Note**: With `any` or `tool`, Claude will not provide natural language before the tool call. The response will start directly with a `tool_use` block.

### Parallel Tool Use Control

The `disable_parallel_tool_use` field controls whether Claude can make multiple tool calls in a single response:

```python
# Allow parallel (default behavior)
tool_choice={"type": "auto"}
# Claude may return 0+ tool_use blocks

# Disable parallel with auto -- at most 1 tool call
tool_choice={"type": "auto", "disable_parallel_tool_use": True}

# Exactly 1 tool call (any tool)
tool_choice={"type": "any", "disable_parallel_tool_use": True}

# Exactly 1 call to a specific tool
tool_choice={"type": "tool", "name": "get_weather", "disable_parallel_tool_use": True}
```

When Claude makes parallel tool calls, all `tool_use` blocks appear in a single assistant message. You must return **all** corresponding `tool_result` blocks in a single subsequent user message.

To encourage parallel tool use, add to the system prompt:
```
For maximum efficiency, whenever you need to perform multiple independent
operations, invoke all relevant tools simultaneously rather than sequentially.
```

---

## Tool Runner Helper (Beta)

The tool runner (`client.beta.messages.tool_runner`) automates the agentic loop. You define tools as decorated functions, and the runner handles execution, result formatting, and re-prompting.

### Python

```python
from anthropic import beta_tool

@beta_tool
def get_weather(location: str, unit: str = "fahrenheit") -> str:
    """Get weather in a location.

    Args:
        location: City and state, e.g. San Francisco, CA
        unit: Temperature unit
    """
    return json.dumps({"temperature": "20C", "condition": "Sunny"})

# Runner handles the full loop automatically
runner = client.beta.messages.tool_runner(
    model="claude-opus-4-6",
    max_tokens=1024,
    tools=[get_weather],
    messages=[{"role": "user", "content": "What's the weather in Paris?"}],
)

# Get final message (blocks until done)
final_message = runner.until_done()
print(final_message.content[0].text)

# Or iterate over intermediate messages
for message in runner:
    print(message.content[0].text)
```

### TypeScript (Zod)

```typescript
import { betaZodTool } from "@anthropic-ai/sdk/helpers/beta/zod";
import { z } from "zod";

const getWeatherTool = betaZodTool({
  name: "get_weather",
  description: "Get the current weather",
  inputSchema: z.object({
    location: z.string().describe("City and state"),
    unit: z.enum(["celsius", "fahrenheit"]).default("fahrenheit"),
  }),
  run: async (input) => {
    return JSON.stringify({ temperature: "20C", condition: "Sunny" });
  },
});

const runner = await client.beta.messages.toolRunner({
  model: "claude-opus-4-6",
  max_tokens: 1024,
  tools: [getWeatherTool],
  messages: [{ role: "user", content: "What's the weather in Paris?" }],
});

console.log(runner.content[0].text);
```

### Tool Runner Features

- **Automatic execution**: Executes tools and manages the while loop.
- **Streaming support**: Set `stream=True` for streaming events within the loop.
- **Error handling**: Catches tool exceptions and returns them as `is_error: true` results.
- **Context compaction**: Automatically compresses long conversations.
- **Custom control**: Iterate over the runner to inspect intermediate tool results.

### Streaming with Tool Runner

```python
runner = client.beta.messages.tool_runner(
    model="claude-opus-4-6",
    max_tokens=1024,
    tools=[get_weather],
    messages=[{"role": "user", "content": "What's the weather?"}],
    stream=True,
)

for message_stream in runner:
    for event in message_stream:
        print("event:", event)
    print("message:", message_stream.get_final_message())
```

---

## Claude Agent SDK (`claude-agent-sdk`)

A separate Python package (MIT license, `pip install claude-agent-sdk`) that wraps the Claude Code CLI for building full agents. This is **not** a thin wrapper around the Messages API -- it orchestrates Claude Code as a subprocess.

- **Current version**: 0.1.48 (as of March 2026)
- **Python**: 3.10+
- **Bundles**: Claude Code CLI automatically

### Core APIs

**`query()`** -- Simple async iterator for one-shot queries:

```python
from claude_agent_sdk import query

async for message in query(prompt="What is 2 + 2?"):
    print(message)
```

**`ClaudeSDKClient`** -- Bidirectional conversations with tools and hooks:

```python
from claude_agent_sdk import ClaudeSDKClient, ClaudeAgentOptions

options = ClaudeAgentOptions(
    system_prompt="You are a helpful assistant.",
    max_turns=10,
    allowed_tools=["Read", "Write", "Bash"],
    permission_mode="acceptEdits",
)

async with ClaudeSDKClient(options=options) as client:
    await client.query("Create a hello.py file")
    async for msg in client.receive_response():
        print(msg)
```

### Custom Tools (In-Process MCP Servers)

```python
from claude_agent_sdk import tool, create_sdk_mcp_server

@tool("greet", "Greet a user", {"name": str})
async def greet_user(args):
    return {"content": [{"type": "text", "text": f"Hello, {args['name']}!"}]}

server = create_sdk_mcp_server(name="my-tools", version="1.0.0", tools=[greet_user])

options = ClaudeAgentOptions(
    mcp_servers={"tools": server},
    allowed_tools=["mcp__tools__greet"],
)
```

### Hooks

Deterministic event handlers at specific points in the agent loop:

```python
from claude_agent_sdk import HookMatcher

async def check_bash(input_data, tool_use_id, context):
    if input_data["tool_name"] != "Bash":
        return {}
    command = input_data["tool_input"].get("command", "")
    if "rm -rf" in command:
        return {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": "Dangerous command blocked",
            }
        }
    return {}

options = ClaudeAgentOptions(
    hooks={"PreToolUse": [HookMatcher(matcher="Bash", hooks=[check_bash])]},
)
```

### Message Types

- `AssistantMessage` -- Claude's responses
- `UserMessage` -- User inputs
- `SystemMessage` -- System context
- `ResultMessage` -- Tool execution results

Content blocks: `TextBlock` (`.text`), `ToolUseBlock`, `ToolResultBlock`.

### Error Hierarchy

- `ClaudeSDKError` -- Base exception
- `CLINotFoundError` -- Claude Code binary not found
- `CLIConnectionError` -- Communication failure
- `ProcessError` -- Process failed (has `.exit_code`)
- `CLIJSONDecodeError` -- Response parsing failure

---

## Tool Types

Claude supports two categories of tools:

### Client Tools
Execute on your infrastructure. You implement the tool and return results.
- User-defined custom tools (any function you want)
- Anthropic-defined client tools: computer use (`computer_20250124`), text editor (`text_editor_20250124`)

### Server Tools
Execute on Anthropic's servers. You specify them but do not implement them.
- Web search (`web_search_20250305`)
- Web fetch (`web_fetch_20250305`)

Server tools use a versioned type string and run in a server-side sampling loop (default 10 iterations). If the loop limit is hit, `stop_reason` is `pause_turn`.

---

## Key Design Patterns

### Best Practices for Tool Definitions
- Write detailed descriptions (3-4+ sentences): explain what, when, and how.
- Use meaningful names with service prefixes: `github_list_prs`, `slack_send_message`.
- Consolidate related operations into one tool with an `action` parameter rather than many separate tools.
- Provide `input_examples` for complex tools with nested objects.
- Return high-signal responses: only necessary data, use stable IDs.

### Agentic Loop Safety
- Always impose a maximum iteration count to prevent infinite loops.
- Check for both `tool_use` and `pause_turn` stop reasons in loops with server tools.
- Handle empty responses (end_turn with no content) by adding a continuation prompt in a new user message.
- Never add text blocks immediately after tool_result blocks.

### Parallel Tool Use
- Enabled by default. Claude may return multiple `tool_use` blocks in one response.
- All `tool_result` blocks must be in a single user message.
- Use system prompt instructions to encourage parallel calls for independent operations.
- Disable with `disable_parallel_tool_use: true` when sequential execution matters.

### Structured Outputs (Strict Mode)
- Add `"strict": true` to tool definitions for guaranteed schema conformance.
- Eliminates type mismatches and missing fields in tool inputs.
- Critical for production agents where invalid parameters cause failures.

### MCP Integration
- MCP tool definitions use `inputSchema` (camelCase); Claude API uses `input_schema` (snake_case). Rename when converting.
- The MCP connector (`mcp_connector`) allows direct connection to remote MCP servers from the Messages API without building a client.
