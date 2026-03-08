import { Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { useState, useEffect, useSyncExternalStore } from 'react'
import {
  MessageSquare,
  GitBranch,
  Zap,
  FileText,
  KeyRound,
  ScrollText,
  Settings,
  MoreHorizontal,
  Sun,
  Moon,
  Plus,
  PanelLeftClose,
  PanelLeftOpen,
} from './Icons'
import { useTheme } from '../hooks/useTheme'
import { chatStore } from '../lib/chatStore'
import { setViewPref } from '../lib/navConfig'

const navItems = [
  { to: '/app/pipelines', label: 'Pipelines', icon: GitBranch },
  { to: '/app/skills', label: 'Skills', icon: Zap },
  { to: '/app/documents', label: 'Documents', icon: FileText },
  { to: '/app/vault', label: 'Vault', icon: KeyRound },
  { to: '/app/logs', label: 'Logs', icon: ScrollText },
  { to: '/app/settings', label: 'Settings', icon: Settings },
]

const mobileItems = [
  { to: '/app/chat', label: 'Chat', icon: MessageSquare },
  { to: '/app/pipelines', label: 'Pipelines', icon: GitBranch },
  { to: '/app/vault', label: 'Vault', icon: KeyRound },
]

const morePages = ['/app/skills', '/app/documents', '/app/logs', '/app/settings']

function useChatSessions() {
  return useSyncExternalStore(
    (cb) => chatStore.subscribe(cb),
    () => chatStore.sessions,
  )
}

function useChatActive() {
  return useSyncExternalStore(
    (cb) => chatStore.subscribe(cb),
    () => chatStore.activeSession,
  )
}

function AppLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const isMoreActive = morePages.includes(location.pathname)
  const { theme, toggle } = useTheme()
  const sessions = useChatSessions()
  const activeSession = useChatActive()
  const [chatExpanded, setChatExpanded] = useState(true)
  const [collapsed, setCollapsed] = useState(() => {
    try { return localStorage.getItem('kuro:sidebar') === 'collapsed' } catch { return false }
  })

  const isChat = location.pathname === '/app/chat'

  useEffect(() => {
    chatStore.load()
  }, [])

  function toggleSidebar() {
    const next = !collapsed
    setCollapsed(next)
    try { localStorage.setItem('kuro:sidebar', next ? 'collapsed' : 'expanded') } catch { /* */ }
  }

  async function handleNewSession() {
    try {
      const empty = sessions.find(s => s.title === 'New Chat')
      if (empty) {
        chatStore.setActive(empty.id)
      } else {
        await chatStore.createSession()
      }
      if (!isChat) navigate('/app/chat')
    } catch { /* ignore */ }
  }

  function selectSession(id: string) {
    chatStore.setActive(id)
    if (!isChat) navigate('/app/chat')
  }

  function switchToDesktop() {
    setViewPref('desktop')
    navigate('/desktop')
  }

  return (
    <div className="flex h-full" style={{ backgroundColor: 'var(--color-surface-primary)' }}>
      {/* Desktop Sidebar */}
      <aside
        className="hidden md:flex flex-col shrink-0 transition-[width] duration-200"
        style={{
          width: collapsed ? '56px' : '224px',
          backgroundColor: 'var(--color-surface-secondary)',
          borderRight: '1px solid var(--color-border-primary)',
        }}
      >
        {/* Header */}
        <div
          className="flex items-center h-14 shrink-0"
          style={{
            borderBottom: '1px solid var(--color-border-primary)',
            padding: collapsed ? '0 10px' : '0 20px',
            justifyContent: collapsed ? 'center' : 'space-between',
          }}
        >
          {collapsed ? (
            <button
              onClick={toggleSidebar}
              className="p-1.5 rounded-lg transition-colors"
              style={{ color: 'var(--color-text-tertiary)' }}
              onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
              onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
              title="Expand sidebar"
            >
              <PanelLeftOpen size={18} />
            </button>
          ) : (
            <>
              <div className="flex items-center gap-2">
                <div className="w-7 h-7 rounded-lg bg-indigo-600 dark:bg-indigo-500 flex items-center justify-center text-white text-sm font-bold">
                  K
                </div>
                <span className="text-lg font-semibold" style={{ color: 'var(--color-text-primary)' }}>Kuro</span>
              </div>
              <button
                onClick={toggleSidebar}
                className="p-1.5 rounded-lg transition-colors"
                style={{ color: 'var(--color-text-tertiary)' }}
                onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
                onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
                title="Collapse sidebar"
              >
                <PanelLeftClose size={18} />
              </button>
            </>
          )}
        </div>

        {/* Nav */}
        <nav className="flex-1 py-2 overflow-y-auto" style={{ padding: collapsed ? '8px 6px' : '8px 12px' }}>
          {collapsed ? (
            /* Collapsed: icon-only nav */
            <div className="flex flex-col items-center gap-1">
              <NavLink
                to="/app/chat"
                className="p-2 rounded-lg transition-colors"
                style={({ isActive }) => ({
                  backgroundColor: isActive ? 'var(--color-surface-active)' : 'transparent',
                  color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                })}
                title="Chat"
              >
                <MessageSquare size={18} />
              </NavLink>
              {navItems.map(item => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className="p-2 rounded-lg transition-colors"
                  style={({ isActive }) => ({
                    backgroundColor: isActive ? 'var(--color-surface-active)' : 'transparent',
                    color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                  })}
                  title={item.label}
                >
                  <item.icon size={18} />
                </NavLink>
              ))}
            </div>
          ) : (
            /* Expanded: full nav with chat sessions */
            <div className="space-y-0.5">
              {/* Chat section with session dropdown */}
              <div>
                <div className="flex items-center">
                  <NavLink
                    to="/app/chat"
                    className="flex-1 flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors"
                    style={({ isActive }) => ({
                      backgroundColor: isActive ? 'var(--color-surface-active)' : 'transparent',
                      color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                      fontWeight: isActive ? 500 : 400,
                    })}
                    onMouseEnter={(e) => {
                      if (!isChat) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.backgroundColor = isChat ? 'var(--color-surface-active)' : 'transparent'
                    }}
                  >
                    <MessageSquare size={18} />
                    Chat
                  </NavLink>
                  <button
                    onClick={() => setChatExpanded(!chatExpanded)}
                    className="p-1 rounded transition-colors shrink-0 mr-1"
                    style={{ color: 'var(--color-text-tertiary)' }}
                    onMouseEnter={e => e.currentTarget.style.color = 'var(--color-text-primary)'}
                    onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
                  >
                    <svg
                      width="12" height="12" viewBox="0 0 12 12"
                      style={{ transform: chatExpanded ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.15s' }}
                    >
                      <path d="M4.5 2.5L8 6L4.5 9.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
                    </svg>
                  </button>
                </div>
                {chatExpanded && (
                  <div className="ml-3 mt-0.5 space-y-px">
                    <button
                      onClick={handleNewSession}
                      className="w-full flex items-center gap-2 px-3 py-1.5 rounded-md text-xs transition-colors"
                      style={{ color: 'var(--color-text-tertiary)' }}
                      onMouseEnter={e => {
                        e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'
                        e.currentTarget.style.color = 'var(--color-text-primary)'
                      }}
                      onMouseLeave={e => {
                        e.currentTarget.style.backgroundColor = 'transparent'
                        e.currentTarget.style.color = 'var(--color-text-tertiary)'
                      }}
                    >
                      <Plus size={12} /> New Chat
                    </button>
                    {sessions.map(s => (
                      <div
                        key={s.id}
                        className="group flex items-center gap-1 px-3 py-1.5 rounded-md cursor-pointer transition-colors"
                        style={{
                          backgroundColor: activeSession === s.id ? 'var(--color-surface-active)' : 'transparent',
                          color: activeSession === s.id ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                        }}
                        onClick={() => selectSession(s.id)}
                        onMouseEnter={e => { if (activeSession !== s.id) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)' }}
                        onMouseLeave={e => { if (activeSession !== s.id) e.currentTarget.style.backgroundColor = 'transparent' }}
                      >
                        <span className="flex-1 text-xs truncate">{s.title}</span>
                        <button
                          onClick={(e) => { e.stopPropagation(); chatStore.deleteSession(s.id) }}
                          className="hidden group-hover:block text-xs shrink-0 px-0.5 rounded transition-colors"
                          style={{ color: 'var(--color-text-tertiary)' }}
                          onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
                          onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
                        >
                          &times;
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Other nav items */}
              {navItems.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors"
                  style={({ isActive }) => ({
                    backgroundColor: isActive ? 'var(--color-surface-active)' : 'transparent',
                    color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                    fontWeight: isActive ? 500 : 400,
                  })}
                  onMouseEnter={(e) => {
                    if (!e.currentTarget.classList.contains('active')) {
                      e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    const active = e.currentTarget.getAttribute('aria-current') === 'page'
                    e.currentTarget.style.backgroundColor = active ? 'var(--color-surface-active)' : 'transparent'
                  }}
                >
                  <item.icon size={18} />
                  {item.label}
                </NavLink>
              ))}
            </div>
          )}
        </nav>

        {/* Footer */}
        <div
          className="flex items-center shrink-0 h-14"
          style={{
            borderTop: '1px solid var(--color-border-primary)',
            justifyContent: collapsed ? 'center' : 'space-between',
            padding: collapsed ? '0' : '0 12px',
          }}
        >
          {!collapsed && (
            <div className="flex items-center gap-2">
              <div className="text-xs px-2" style={{ color: 'var(--color-text-tertiary)' }}>Kuro v0.1.0</div>
              <button
                onClick={switchToDesktop}
                className="text-xs px-1.5 py-0.5 rounded transition-colors"
                style={{ color: 'var(--color-text-tertiary)', border: '1px solid var(--color-border-primary)' }}
                onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
                onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
                title="Switch to Desktop view"
              >
                Desktop
              </button>
            </div>
          )}
          <button
            onClick={toggle}
            className="p-1.5 rounded-lg transition-colors"
            style={{ color: 'var(--color-text-tertiary)' }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'
              e.currentTarget.style.color = 'var(--color-text-primary)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = 'transparent'
              e.currentTarget.style.color = 'var(--color-text-tertiary)'
            }}
            title={theme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'}
          >
            {theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}
          </button>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col min-w-0 pb-14 md:pb-0">
        <Outlet />
      </main>

      {/* Mobile Bottom Tabs */}
      <nav
        className="md:hidden fixed bottom-0 left-0 right-0 flex backdrop-blur-sm"
        style={{ borderTop: '1px solid var(--color-border-primary)', backgroundColor: 'color-mix(in srgb, var(--color-surface-primary) 95%, transparent)' }}
      >
        {mobileItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className="flex-1 flex flex-col items-center gap-1 py-2 text-xs transition-colors"
            style={({ isActive }) => ({
              color: isActive ? 'var(--color-accent)' : 'var(--color-text-tertiary)',
            })}
          >
            <item.icon size={20} />
            {item.label}
          </NavLink>
        ))}
        <NavLink
          to="/app/skills"
          className="flex-1 flex flex-col items-center gap-1 py-2 text-xs transition-colors"
          style={{ color: isMoreActive ? 'var(--color-accent)' : 'var(--color-text-tertiary)' }}
        >
          <MoreHorizontal size={20} />
          More
        </NavLink>
      </nav>
    </div>
  )
}

export default AppLayout
