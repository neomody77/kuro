import { useState, useEffect, useRef, useCallback, useSyncExternalStore } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import TextareaAutosize from 'react-textarea-autosize'
import { Send } from '../components/Icons'
import { chatStore } from '../lib/chatStore'
import { useChatStream, type StreamMessage, type ToolCallEntry } from '../hooks/useChatStream'

function useActiveSession() {
  return useSyncExternalStore(
    (cb) => chatStore.subscribe(cb),
    () => chatStore.activeSession,
  )
}

function Chat() {
  const activeSession = useActiveSession()
  const [messages, setMessages] = useState<StreamMessage[]>([])
  const [input, setInput] = useState('')
  const [error, setError] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)

  const { sendStream, sendConfirmation, abort, streaming } = useChatStream(setMessages)

  useEffect(() => {
    chatStore.load()
  }, [])

  // Load messages when session changes
  useEffect(() => {
    if (!activeSession) {
      setMessages([])
      return
    }
    fetch(`/api/chat/history?session_id=${activeSession}`)
      .then(r => r.json())
      .then((msgs: Array<{ id: string; role: 'user' | 'assistant'; content: string; timestamp: string }>) =>
        setMessages((msgs || []).map(m => ({ ...m, toolCalls: [] })))
      )
      .catch(() => setMessages([]))
  }, [activeSession])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const send = useCallback(async () => {
    const text = input.trim()
    if (!text || streaming) return

    // Auto-create session if none active
    let sid = activeSession
    if (!sid) {
      try {
        const info = await chatStore.createSession()
        sid = info.id
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to create session')
        return
      }
    }

    setInput('')
    setError('')

    // Add user message
    const userMsg: StreamMessage = {
      id: `usr-${Date.now()}`,
      role: 'user',
      content: text,
      toolCalls: [],
      timestamp: new Date().toISOString(),
    }
    setMessages(prev => [...prev, userMsg])

    // Update session title from first message only
    const session = chatStore.sessions.find(s => s.id === sid)
    if (!session || session.title === 'New Chat') {
      chatStore.updateTitle(sid, text.length > 40 ? text.slice(0, 40) + '...' : text)
    }

    try {
      await sendStream(text, sid)
    } catch (e) {
      if (!(e instanceof DOMException && e.name === 'AbortError')) {
        setError(e instanceof Error ? e.message : 'Failed to send')
      }
    }
  }, [input, streaming, activeSession, sendStream])

  const handleConfirm = useCallback(async (callId: string, confirmed: boolean, msgId: string) => {
    const sid = activeSession
    if (!sid || streaming) return
    try {
      await sendConfirmation(sid, callId, confirmed, msgId)
    } catch (e) {
      if (!(e instanceof DOMException && e.name === 'AbortError')) {
        setError(e instanceof Error ? e.message : 'Confirmation failed')
      }
    }
  }, [activeSession, streaming, sendConfirmation])

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      send()
    }
  }

  const [copiedId, setCopiedId] = useState<string | null>(null)

  function copyMessage(id: string, content: string) {
    navigator.clipboard.writeText(content)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 1500)
  }

  function formatTime(ts: string) {
    try {
      return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    } catch { return '' }
  }

  return (
    <div className="flex flex-col h-full">
      {/* Chat area */}
      <div className="flex-1 overflow-y-auto px-4 py-6">
        <div className="max-w-3xl mx-auto">
          {messages.length === 0 && !streaming && (
            <div className="text-center py-20">
              <div className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
Send a message to start a conversation
              </div>
            </div>
          )}
          {messages.map((msg, i) => (
            <div key={`${msg.id}-${msg.role}-${i}`}>
              {msg.role === 'user' ? (
                <div className="py-4" style={{ borderBottom: '1px solid var(--color-border-secondary)' }}>
                  <div className="flex items-center gap-2 mb-1.5">
                    <span className="text-xs font-medium" style={{ color: 'var(--color-text-secondary)' }}>You</span>
                    <span className="text-xs" style={{ color: 'var(--color-text-quaternary)' }}>{formatTime(msg.timestamp)}</span>
                  </div>
                  <div className="text-sm leading-relaxed whitespace-pre-wrap" style={{ color: 'var(--color-text-primary)' }}>
                    {msg.content}
                  </div>
                </div>
              ) : (
                <div className="group py-4" style={{ borderBottom: '1px solid var(--color-border-secondary)' }}>
                  <div className="flex items-center gap-2 mb-1.5">
                    <span className="text-xs font-medium" style={{ color: 'var(--color-accent)' }}>Kuro</span>
                    <span className="text-xs" style={{ color: 'var(--color-text-quaternary)' }}>{formatTime(msg.timestamp)}</span>
                    {!msg.streaming && (
                      <button
                        onClick={() => copyMessage(msg.id, msg.content)}
                        className="opacity-0 group-hover:opacity-100 transition-opacity ml-auto p-0.5 rounded hover:bg-white/10"
                        title="Copy"
                      >
                        {copiedId === msg.id ? (
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--color-success, #22c55e)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12" /></svg>
                        ) : (
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-tertiary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
                        )}
                      </button>
                    )}
                  </div>

                  {/* Tool calls */}
                  {msg.toolCalls.length > 0 && (
                    <div className="mb-2 flex flex-col gap-1.5">
                      {msg.toolCalls.map((tc, j) => (
                        <ToolCallCard
                          key={`${tc.callId}-${j}`}
                          toolCall={tc}
                          onConfirm={(callId, confirmed) => handleConfirm(callId, confirmed, msg.id)}
                        />
                      ))}
                    </div>
                  )}

                  {/* Text content */}
                  {msg.content && (
                    <div className="prose-kuro text-sm leading-relaxed">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
                    </div>
                  )}

                  {/* Streaming cursor — show while waiting for any content */}
                  {msg.streaming && (
                    <div className="flex items-center gap-1 mt-1">
                      <span className="inline-block w-1.5 h-1.5 rounded-full animate-pulse" style={{ backgroundColor: 'var(--color-text-tertiary)' }} />
                      <span className="inline-block w-1.5 h-1.5 rounded-full animate-pulse [animation-delay:150ms]" style={{ backgroundColor: 'var(--color-text-tertiary)' }} />
                      <span className="inline-block w-1.5 h-1.5 rounded-full animate-pulse [animation-delay:300ms]" style={{ backgroundColor: 'var(--color-text-tertiary)' }} />
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
          {error && (
            <div className="py-3 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>
          )}
          <div ref={bottomRef} />
        </div>
      </div>

      <div className="px-4 py-2 shrink-0" style={{ borderTop: '1px solid var(--color-border-primary)' }}>
        <form
          className="max-w-3xl mx-auto w-full flex gap-2 items-end"
          onSubmit={(e) => { e.preventDefault(); send() }}
        >
          <TextareaAutosize
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message... (Shift+Enter for new line)"
            disabled={streaming}
            minRows={1}
            maxRows={6}
            className="flex-1 rounded-lg px-3.5 py-2 text-sm transition-colors outline-none resize-none"
            style={{
              backgroundColor: 'var(--color-surface-tertiary)',
              border: '1px solid var(--color-border-primary)',
              color: 'var(--color-text-primary)',
            }}
            onFocus={(e) => e.currentTarget.style.borderColor = 'var(--color-border-focus)'}
            onBlur={(e) => e.currentTarget.style.borderColor = 'var(--color-border-primary)'}
          />
          {streaming ? (
            <button
              type="button"
              onClick={abort}
              className="rounded-lg px-3.5 py-2 transition-colors shrink-0"
              style={{ backgroundColor: 'var(--color-error)', color: '#fff' }}
              title="Stop"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><rect x="3" y="3" width="10" height="10" rx="1.5" /></svg>
            </button>
          ) : (
            <button
              type="submit"
              disabled={!input.trim()}
              className="rounded-lg px-3.5 py-2 transition-colors shrink-0"
              style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)', opacity: !input.trim() ? 0.4 : 1 }}
            >
              <Send size={16} />
            </button>
          )}
        </form>
      </div>
    </div>
  )
}

function ToolCallCard({ toolCall, onConfirm }: { toolCall: ToolCallEntry; onConfirm?: (callId: string, confirmed: boolean) => void }) {
  const [expanded, setExpanded] = useState(false)

  const statusColor =
    toolCall.status === 'calling' ? 'var(--color-warning, #eab308)' :
    toolCall.status === 'done' || toolCall.status === 'confirmed' ? 'var(--color-success, #22c55e)' :
    toolCall.status === 'confirm' ? 'var(--color-accent, #6366f1)' :
    toolCall.status === 'rejected' ? 'var(--color-text-tertiary)' :
    'var(--color-error, #ef4444)'

  const statusIcon =
    toolCall.status === 'calling' ? null :
    toolCall.status === 'done' || toolCall.status === 'confirmed' ? '\u2713' :
    toolCall.status === 'confirm' ? '?' :
    toolCall.status === 'rejected' ? '\u2717' :
    '\u2717'

  // Brief summary of key params for collapsed view
  const paramSummary = summarizeParams(toolCall.input)

  return (
    <div
      className="rounded-md text-xs overflow-hidden"
      style={{ border: '1px solid var(--color-border-secondary)', backgroundColor: 'var(--color-surface-secondary)' }}
    >
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-2.5 py-1.5 text-left hover:bg-white/5 transition-colors"
      >
        {toolCall.status === 'calling' ? (
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" className="animate-spin" stroke={statusColor} strokeWidth="3" strokeLinecap="round">
            <path d="M12 2a10 10 0 0 1 10 10" />
          </svg>
        ) : (
          <span style={{ color: statusColor, fontWeight: 600, fontSize: '11px' }}>
            {statusIcon}
          </span>
        )}
        <code style={{ color: 'var(--color-text-secondary)' }}>{toolCall.name}</code>
        {toolCall.status === 'confirm' && (
          <span className="text-[10px] font-medium px-1.5 py-0.5 rounded-full" style={{ backgroundColor: 'var(--color-accent)', color: '#fff' }}>
            Needs Approval
          </span>
        )}
        {toolCall.status === 'rejected' && (
          <span className="text-[10px]" style={{ color: 'var(--color-text-tertiary)' }}>Rejected</span>
        )}
        {paramSummary && toolCall.status !== 'confirm' && (
          <span className="truncate" style={{ color: 'var(--color-text-quaternary)', maxWidth: '200px' }}>
            {paramSummary}
          </span>
        )}
        <svg
          width="12" height="12" viewBox="0 0 24 24" fill="none"
          stroke="var(--color-text-quaternary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
          className="ml-auto shrink-0 transition-transform" style={{ transform: expanded ? 'rotate(180deg)' : 'rotate(0)' }}
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {/* Confirmation banner */}
      {toolCall.status === 'confirm' && onConfirm && (
        <div
          className="px-2.5 py-2 flex items-center gap-3"
          style={{ borderTop: '1px solid var(--color-border-secondary)', backgroundColor: 'color-mix(in srgb, var(--color-accent) 8%, transparent)' }}
        >
          <div className="flex-1">
            <div className="text-xs" style={{ color: 'var(--color-text-secondary)' }}>
              {toolCall.hint || `Confirm execution of ${toolCall.name}?`}
            </div>
            <div className="text-[10px] mt-0.5" style={{ color: 'var(--color-text-quaternary)' }}>
              {summarizeParams(toolCall.input)}
            </div>
          </div>
          <div className="flex gap-1.5 shrink-0">
            <button
              onClick={(e) => { e.stopPropagation(); onConfirm(toolCall.callId, true) }}
              className="px-2.5 py-1 rounded text-xs font-medium transition-colors"
              style={{ backgroundColor: 'var(--color-success, #22c55e)', color: '#fff' }}
            >
              Approve
            </button>
            <button
              onClick={(e) => { e.stopPropagation(); onConfirm(toolCall.callId, false) }}
              className="px-2.5 py-1 rounded text-xs font-medium transition-colors"
              style={{ backgroundColor: 'var(--color-error, #ef4444)', color: '#fff' }}
            >
              Reject
            </button>
          </div>
        </div>
      )}

      {expanded && (
        <div className="px-2.5 pb-2 space-y-1.5" style={{ borderTop: '1px solid var(--color-border-secondary)' }}>
          <div className="pt-1.5">
            <div className="text-[10px] font-medium uppercase tracking-wider mb-0.5" style={{ color: 'var(--color-text-quaternary)' }}>Input</div>
            <pre className="overflow-x-auto whitespace-pre-wrap rounded px-2 py-1.5" style={{ color: 'var(--color-text-secondary)', backgroundColor: 'var(--color-surface-tertiary)' }}>
              {formatJson(toolCall.input)}
            </pre>
          </div>
          {toolCall.output && (
            <div>
              <div className="text-[10px] font-medium uppercase tracking-wider mb-0.5" style={{ color: 'var(--color-text-quaternary)' }}>Output</div>
              <pre className="overflow-x-auto whitespace-pre-wrap rounded px-2 py-1.5 max-h-60 overflow-y-auto" style={{ color: 'var(--color-text-secondary)', backgroundColor: 'var(--color-surface-tertiary)' }}>
                {formatJson(toolCall.output)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function summarizeParams(input: Record<string, unknown>): string {
  const parts: string[] = []
  for (const [k, v] of Object.entries(input)) {
    if (typeof v === 'string' && v.length > 0) {
      const display = v.length > 30 ? v.slice(0, 30) + '...' : v
      parts.push(`${k}=${display}`)
    }
  }
  return parts.slice(0, 2).join(', ')
}

function formatJson(obj: unknown): string {
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
  }
}

export default Chat
