# Vercel AI SDK Agent Loop

## Overview
- TypeScript, `ai` npm package
- Core loop inside `generateText` / `streamText`, `ToolLoopAgent` is convenience wrapper
- `do...while` loop (generateText) / recursive `streamStep` (streamText)

## Core Loop (generateText)
```ts
do {
  // 1. prepareStep callback (override model, tools, system per step)
  // 2. Convert prompt to LLM format
  // 3. Call LLM
  // 4. Parse tool calls from response
  // 5. Execute tools (executeToolCall for each)
  // 6. Append response messages for next iteration
  // 7. Create StepResult, push to steps[]
  // 8. Fire onStepFinish callback
} while (
  (clientToolCalls.length > 0 && allExecuted || pendingDeferred) &&
  !isStopConditionMet(stopConditions, steps)
);
```

Terminates when: no tool calls, tool lacks execute, needs approval, or stop condition met.

## streamText Loop
- Recursive `streamStep()` instead of do-while
- Stitchable stream with sub-streams per step
- `runToolsTransformation` TransformStream: parses tool calls inline, executes, emits results
- Streaming chunks: tool-input-start, tool-input-delta, tool-input-end, tool-call, tool-result

## Tool Definition
```ts
const weatherTool = tool({
  description: 'Get weather',
  inputSchema: z.object({ location: z.string() }),
  execute: async ({ location }) => ({ temperature: 72 }),
  // Optional: needsApproval, outputSchema, toModelOutput, strict
});
```
- Schema: Zod v3/v4, Valibot, or raw JSON Schema
- Tools without `execute` halt the loop (client-side tools)
- Error in execute -> `tool-error` content part (LLM sees error, can retry)

## Stop Conditions
```ts
stopWhen: stepCountIs(5)  // or array (OR logic)
stopWhen: [stepCountIs(20), hasToolCall('finalAnswer')]
// Custom:
const budgetExceeded = ({ steps }) => estimateCost(steps) > 0.5;
```
- Default generateText: `stepCountIs(1)` (single step)
- Default ToolLoopAgent: `stepCountIs(20)`
- Built-in: `stepCountIs(n)`, `hasToolCall(name)`

## Callbacks
- `onStepFinish(StepResult)` — after each step
- `onFinish` — after all steps, with totalUsage
- `experimental_onStepStart` — before each step
- `experimental_onToolCallStart/Finish` — tool lifecycle
- `prepareStep` — dynamic per-step control (model, tools, toolChoice)

## ToolLoopAgent
```ts
const agent = new ToolLoopAgent({
  model: openai('gpt-4o'),
  tools: { weather: weatherTool },
  stopWhen: stepCountIs(20),
  prepareStep: async ({ stepNumber }) => {
    if (stepNumber === 0) return { toolChoice: { type: 'tool', toolName: 'search' } };
    return {};
  },
});
const result = await agent.generate({ prompt: '...' });
```

## Key Design
- Single engine: loop logic inside generateText/streamText, not external
- Message accumulation: each step's messages appended for next step
- prepareStep: dynamic model/tool/system changes per step
- Tool dispatch: auto for tools with execute, halt for client-side tools
- Streaming: tool calls parsed inline from stream, results merged back
