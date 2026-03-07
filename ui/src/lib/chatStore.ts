import { api } from './api'

export type SessionInfo = {
  id: string
  title: string
  created: string
}

type Listener = () => void

class ChatStore {
  sessions: SessionInfo[] = []
  activeSession = ''
  private listeners: Set<Listener> = new Set()
  private loaded = false

  subscribe(fn: Listener) {
    this.listeners.add(fn)
    return () => { this.listeners.delete(fn) }
  }

  private notify() {
    this.listeners.forEach(fn => fn())
  }

  async load() {
    if (this.loaded) return
    try {
      const list = await api.get<SessionInfo[]>('/api/chat/sessions')
      this.sessions = list || []
      if (this.sessions.length > 0 && !this.activeSession) {
        this.activeSession = this.sessions[0].id
      }
      this.loaded = true
      this.notify()
    } catch { /* ignore */ }
  }

  async createSession() {
    const info = await api.post<SessionInfo>('/api/chat/sessions')
    this.sessions = [info, ...this.sessions]
    this.activeSession = info.id
    this.notify()
    return info
  }

  async deleteSession(id: string) {
    await api.del(`/api/chat/sessions/${id}`)
    this.sessions = this.sessions.filter(s => s.id !== id)
    if (this.activeSession === id) {
      this.activeSession = this.sessions.length > 0 ? this.sessions[0].id : ''
    }
    this.notify()
  }

  setActive(id: string) {
    this.activeSession = id
    this.notify()
  }

  updateTitle(id: string, title: string) {
    this.sessions = this.sessions.map(s =>
      s.id === id ? { ...s, title } : s
    )
    this.notify()
  }
}

export const chatStore = new ChatStore()
