import { useState, useEffect, useRef, useCallback } from 'react'

export type ServerEvent = {
  id: string
  type: string
  title: string
  message: string
  severity: 'info' | 'success' | 'error' | 'warning'
  timestamp: string
  meta?: Record<string, unknown>
}

export type Notification = ServerEvent & {
  read: boolean
}

const MAX_NOTIFICATIONS = 50

export function useEventStream() {
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [connected, setConnected] = useState(false)
  const esRef = useRef<EventSource | null>(null)

  useEffect(() => {
    const es = new EventSource('/api/events')
    esRef.current = es

    es.onopen = () => setConnected(true)

    es.onmessage = (e) => {
      try {
        const ev: ServerEvent = JSON.parse(e.data)
        setNotifications(prev => {
          const next = [{ ...ev, read: false }, ...prev]
          return next.slice(0, MAX_NOTIFICATIONS)
        })
      } catch {
        // ignore malformed messages
      }
    }

    es.onerror = () => {
      setConnected(false)
      // EventSource auto-reconnects
    }

    return () => {
      es.close()
      esRef.current = null
    }
  }, [])

  const markAllRead = useCallback(() => {
    setNotifications(prev => prev.map(n => ({ ...n, read: true })))
  }, [])

  const dismiss = useCallback((id: string) => {
    setNotifications(prev => prev.filter(n => n.id !== id))
  }, [])

  const clearAll = useCallback(() => {
    setNotifications([])
  }, [])

  const unreadCount = notifications.filter(n => !n.read).length

  return { notifications, connected, unreadCount, markAllRead, dismiss, clearAll }
}
