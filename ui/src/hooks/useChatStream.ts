import { useState, useCallback, useRef } from 'react'

export type StreamEvent =
  | { type: 'text_delta'; text: string }
  | { type: 'text'; text: string }
  | { type: 'tool_call'; tool_name: string; tool_input: Record<string, unknown>; call_id: string }
  | { type: 'tool_result'; tool_name: string; tool_output: Record<string, unknown>; call_id: string }
  | { type: 'error'; error: string }
  | { type: 'done' }

export type ToolCallEntry = {
  callId: string
  name: string
  input: Record<string, unknown>
  output?: Record<string, unknown>
  status: 'calling' | 'done' | 'error'
}

export type StreamMessage = {
  id: string
  role: 'user' | 'assistant'
  content: string
  toolCalls: ToolCallEntry[]
  timestamp: string
  streaming?: boolean
}

type UseChatStreamReturn = {
  sendStream: (text: string, sessionId: string) => Promise<void>
  abort: () => void
  streaming: boolean
}

export function useChatStream(
  onUpdate: (updater: (msgs: StreamMessage[]) => StreamMessage[]) => void
): UseChatStreamReturn {
  const [streaming, setStreaming] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const abort = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort()
      abortRef.current = null
      setStreaming(false)
    }
  }, [])

  const sendStream = useCallback(async (text: string, sessionId: string) => {
    const controller = new AbortController()
    abortRef.current = controller
    setStreaming(true)

    const assistantId = `stream-${Date.now()}`

    // Add placeholder assistant message
    onUpdate(msgs => [...msgs, {
      id: assistantId,
      role: 'assistant' as const,
      content: '',
      toolCalls: [],
      timestamp: new Date().toISOString(),
      streaming: true,
    }])

    try {
      const res = await fetch('/api/chat/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: text, session_id: sessionId }),
        signal: controller.signal,
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        throw new Error(err.error || res.statusText)
      }

      const reader = res.body!.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const jsonStr = line.slice(6).trim()
          if (!jsonStr) continue

          let event: StreamEvent
          try {
            event = JSON.parse(jsonStr)
          } catch {
            continue
          }

          onUpdate(msgs => {
            const idx = msgs.findIndex(m => m.id === assistantId)
            if (idx < 0) return msgs
            const msg = { ...msgs[idx] }

            switch (event.type) {
              case 'text_delta':
                msg.content += event.text
                break
              case 'text':
                msg.content += event.text
                break
              case 'tool_call':
                msg.toolCalls = [...msg.toolCalls, {
                  callId: event.call_id,
                  name: event.tool_name,
                  input: event.tool_input,
                  status: 'calling',
                }]
                break
              case 'tool_result': {
                msg.toolCalls = msg.toolCalls.map(tc =>
                  tc.callId === event.call_id || tc.name === event.tool_name
                    ? { ...tc, output: event.tool_output, status: 'done' as const }
                    : tc
                )
                break
              }
              case 'error':
                msg.content += `\n\n**Error:** ${event.error}`
                break
              case 'done':
                msg.streaming = false
                break
            }

            const next = [...msgs]
            next[idx] = msg
            return next
          })
        }
      }
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') {
        // User cancelled
      } else {
        onUpdate(msgs => {
          const idx = msgs.findIndex(m => m.id === assistantId)
          if (idx < 0) return msgs
          const msg = { ...msgs[idx], streaming: false }
          msg.content += `\n\n**Error:** ${e instanceof Error ? e.message : 'Stream failed'}`
          const next = [...msgs]
          next[idx] = msg
          return next
        })
      }
    } finally {
      // Mark stream complete
      onUpdate(msgs => {
        const idx = msgs.findIndex(m => m.id === assistantId)
        if (idx < 0) return msgs
        if (msgs[idx].streaming) {
          const next = [...msgs]
          next[idx] = { ...msgs[idx], streaming: false }
          return next
        }
        return msgs
      })
      setStreaming(false)
      abortRef.current = null
    }
  }, [onUpdate])

  return { sendStream, abort, streaming }
}
