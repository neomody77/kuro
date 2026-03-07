# OpenAI Codex CLI Agent Loop

## Overview
- Rust (96% of codebase), `codex-rs/core/src/`
- **SQ/EQ pattern**: Submission Queue (user ops) / Event Queue (agent events)
- Decouples UI (TUI or exec mode) from core logic

## Three-Level Loop

### Level 1: submission_loop -- top-level dispatch
```rust
while let Ok(sub) = rx_sub.recv().await {
    match sub.op {
        Op::UserInput | Op::UserTurn => handlers::user_input_or_turn(&sess, ...).await,
        Op::ExecApproval { decision } => handlers::exec_approval(&sess, ...).await,
        Op::Interrupt => handlers::interrupt(&sess).await,
        Op::Shutdown => return,
    }
}
```

### Level 2: run_turn -- one user turn (may loop)
```rust
loop {
    match run_sampling_request(sess, turn_context, input, token).await {
        Ok(SamplingRequestResult { needs_follow_up: false, .. }) => break,  // done
        Ok(SamplingRequestResult { needs_follow_up: true, .. }) => continue, // tool calls executed, loop
        Err(TurnAborted) => break,
        Err(e) => { emit error; break }
    }
}
```
`needs_follow_up = true` whenever model emits a tool call.

### Level 3: try_run_sampling_request -- one streaming API call
```rust
let mut stream = client_session.stream(&prompt, &model_info).await;
let mut in_flight: FuturesOrdered<BoxFuture<ResponseInputItem>> = FuturesOrdered::new();

loop {
    match stream.next().await {
        OutputItemDone(item) => {
            if let Some(tool_future) = handle_output_item_done(item) {
                in_flight.push_back(tool_future);  // queue tool execution
            }
            needs_follow_up |= has_tool_calls;
        }
        OutputTextDelta(delta) => { /* stream to UI */ }
        Completed { token_usage } => break,
    }
}
drain_in_flight(&mut in_flight).await;  // wait for all tool calls
```

## Tool System
- ToolRouter dispatches to handlers from ToolRegistry
- ToolPayload: Function, LocalShell, Mcp, Custom
- Shell: ExecParams -> ToolOrchestrator (approval + sandbox + retry)
- **Parallel execution** via FuturesOrdered + RwLock (read lock = parallel, write lock = serial)

## Tool Result Format
```rust
ResponseInputItem::FunctionCallOutput {
    call_id: "call_abc123",
    output: FunctionCallOutputPayload {
        body: Text("Exit code: 0\nWall time: 1.2s\nOutput:\n..."),
        success: Some(true),
    },
}
```
Uses OpenAI Responses API format.

## Stop Conditions
1. No tool calls (`needs_follow_up == false`)
2. User interrupt (CancellationToken)
3. Fatal error (context window, auth, stream disconnect)
4. Hook abort (after_agent hook fails with FailedAbort)

## Streaming
- ResponseEvent enum: Created, OutputItemAdded, OutputItemDone, OutputTextDelta, Completed, etc.
- Text deltas forwarded as AgentMessageDelta events
- Tool calls dispatched on OutputItemDone (complete args), not on deltas
- Transport: WebSocket (preferred) + SSE (fallback)

## Approval System
```rust
enum AskForApproval {
    UnlessTrusted,  // "suggest" mode, read-only auto-approved
    OnRequest,      // default interactive
    Never,          // non-interactive, never prompt
}
```

CLI flags:
- Default = OnRequest + read-only sandbox
- `--full-auto` = OnRequest + WorkspaceWrite sandbox
- `--yolo` = Never + DangerFullAccess

Flow: check approval -> if needed, emit ExecApprovalRequest -> suspend -> user responds with ReviewDecision -> execute with sandbox -> if denied, optionally retry without sandbox

ReviewDecision: Approved, ApprovedForSession, Denied, Abort, NetworkPolicyAmendment

## Sandbox Policies
- DangerFullAccess, ReadOnly, ExternalSandbox, WorkspaceWrite
- WorkspaceWrite: .git/.codex/.agents dirs read-only even in writable roots
- macOS Seatbelt / Linux Landlock+bubblewrap

## Key Design
- SQ/EQ pattern decouples UI from core
- Parallel tool execution with FuturesOrdered
- Multi-layer sandbox + approval + exec policy
- Stateless conversation (full history resent each turn)
- WebSocket with sticky routing for connection reuse
