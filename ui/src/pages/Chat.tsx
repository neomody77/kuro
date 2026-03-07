import { useState, useEffect, useRef, useCallback, useSyncExternalStore } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import TextareaAutosize from 'react-textarea-autosize'
import { Send } from '../components/Icons'
import { api } from '../lib/api'
import { chatStore } from '../lib/chatStore'

type Message = {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: string
}

type SkillCall = {
  skill: string
  inputs: Record<string, unknown>
  confirm: boolean
}

type ChatResponse = {
  message: Message
  skillCall?: SkillCall
}

function useActiveSession() {
  return useSyncExternalStore(
    (cb) => chatStore.subscribe(cb),
    () => chatStore.activeSession,
  )
}

function Chat() {
  const activeSession = useActiveSession()
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [pendingSkill, setPendingSkill] = useState<SkillCall | null>(null)
  const [error, setError] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    chatStore.load()
  }, [])

  // Load messages when session changes
  useEffect(() => {
    if (!activeSession) {
      setMessages([])
      return
    }
    api.get<Message[]>(`/api/chat/history?session_id=${activeSession}`)
      .then(msgs => setMessages(msgs || []))
      .catch(() => setMessages([]))
  }, [activeSession])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const abort = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort()
      abortRef.current = null
      setLoading(false)
    }
  }, [])

  async function send() {
    const text = input.trim()
    if (!text || loading) return

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
    setLoading(true)

    const controller = new AbortController()
    abortRef.current = controller

    const userMsg: Message = { id: `tmp-${Date.now()}`, role: 'user', content: text, timestamp: new Date().toISOString() }
    setMessages(prev => [...prev, userMsg])

    try {
      const resp = await api.post<ChatResponse>('/api/chat', { message: text, session_id: sid }, controller.signal)
      setMessages(prev => {
        const filtered = prev.filter(m => m.id !== userMsg.id)
        return [...filtered, { ...userMsg, id: resp.message.id ? `usr-${resp.message.id}` : userMsg.id }, resp.message]
      })
      // Update session title from first message
      chatStore.updateTitle(sid, text.length > 40 ? text.slice(0, 40) + '...' : text)
      if (resp.skillCall?.confirm) {
        setPendingSkill(resp.skillCall)
      }
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') {
        // User cancelled
      } else {
        setError(e instanceof Error ? e.message : 'Failed to send')
      }
    } finally {
      setLoading(false)
      abortRef.current = null
    }
  }

  async function handleConfirm(approve: boolean) {
    setPendingSkill(null)
    setLoading(true)
    try {
      const resp = await api.post<ChatResponse>('/api/chat/confirm', { approve, session_id: activeSession })
      setMessages(prev => [...prev, resp.message])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to confirm')
    } finally {
      setLoading(false)
    }
  }

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
          {messages.length === 0 && !loading && (
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
                  </div>
                  <div className="prose-kuro text-sm leading-relaxed">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
                  </div>
                </div>
              )}
            </div>
          ))}
          {loading && (
            <div className="py-4">
              <div className="flex items-center gap-2 mb-1.5">
                <span className="text-xs font-medium" style={{ color: 'var(--color-accent)' }}>Kuro</span>
              </div>
              <div className="flex items-center gap-1">
                <span className="inline-block w-1.5 h-1.5 rounded-full animate-pulse" style={{ backgroundColor: 'var(--color-text-tertiary)' }} />
                <span className="inline-block w-1.5 h-1.5 rounded-full animate-pulse [animation-delay:150ms]" style={{ backgroundColor: 'var(--color-text-tertiary)' }} />
                <span className="inline-block w-1.5 h-1.5 rounded-full animate-pulse [animation-delay:300ms]" style={{ backgroundColor: 'var(--color-text-tertiary)' }} />
              </div>
            </div>
          )}
          {error && (
            <div className="py-3 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>
          )}
          <div ref={bottomRef} />
        </div>
      </div>

      {pendingSkill && (
        <div className="px-4 py-3 shrink-0" style={{ borderTop: '1px solid var(--color-border-primary)', backgroundColor: 'var(--color-surface-secondary)' }}>
          <div className="max-w-3xl mx-auto">
            <div className="text-xs font-medium mb-1" style={{ color: 'var(--color-warning)' }}>Confirm action</div>
            <div className="text-sm mb-1.5" style={{ color: 'var(--color-text-primary)' }}>
              <code className="text-xs px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-surface-tertiary)' }}>{pendingSkill.skill}</code>
            </div>
            <pre className="text-xs mb-3 overflow-x-auto" style={{ color: 'var(--color-text-secondary)' }}>
              {JSON.stringify(pendingSkill.inputs, null, 2)}
            </pre>
            <div className="flex gap-2">
              <button
                onClick={() => handleConfirm(true)}
                className="text-xs font-medium rounded-md px-3 py-1.5 transition-colors"
                style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}
              >
                Approve
              </button>
              <button
                onClick={() => handleConfirm(false)}
                className="text-xs font-medium rounded-md px-3 py-1.5 transition-colors"
                style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)', border: '1px solid var(--color-border-primary)' }}
              >
                Deny
              </button>
            </div>
          </div>
        </div>
      )}

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
            disabled={loading}
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
          {loading ? (
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

export default Chat
