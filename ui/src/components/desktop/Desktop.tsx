import { useState, useCallback, useEffect, lazy, Suspense } from 'react'
import Window from './Window'
import Taskbar from './Taskbar'
import AppLauncher from './AppLauncher'
import NotificationCenter from './NotificationCenter'
import { useEventStream } from '../../hooks/useEventStream'
import { allPages } from '../../lib/navConfig'
import { loadWindowLayout, saveWindowLayout, getNextZIndex, createWindowId } from '../../lib/windowStore'
import type { WindowState } from './Window'

// Lazy-load page components
const Chat = lazy(() => import('../../pages/Chat'))
const Pipelines = lazy(() => import('../../pages/Pipelines'))
const Skills = lazy(() => import('../../pages/Skills'))
const Documents = lazy(() => import('../../pages/Documents'))
const Vault = lazy(() => import('../../pages/Vault'))
const Logs = lazy(() => import('../../pages/Logs'))
const Settings = lazy(() => import('../../pages/Settings'))

const pageComponents: Record<string, React.LazyExoticComponent<any>> = {
  chat: Chat,
  pipelines: Pipelines,
  skills: Skills,
  documents: Documents,
  vault: Vault,
  logs: Logs,
  settings: Settings,
}

function getDefaultPosition(index: number): { x: number; y: number } {
  const offset = (index % 6) * 40
  return { x: 80 + offset, y: 40 + offset }
}

function PageFallback() {
  return (
    <div className="flex-1 flex items-center justify-center text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
      Loading...
    </div>
  )
}

export default function Desktop() {
  const [windows, setWindows] = useState<WindowState[]>(() => loadWindowLayout())
  const [launcherOpen, setLauncherOpen] = useState(false)
  const [notificationsOpen, setNotificationsOpen] = useState(false)
  const { notifications, unreadCount, markAllRead, dismiss } = useEventStream()

  // Persist window layout
  useEffect(() => {
    saveWindowLayout(windows)
  }, [windows])

  const launchApp = useCallback((appId: string) => {
    // Focus existing window for this app if already open
    setWindows(prev => {
      const existing = prev.find(w => w.appId === appId)
      if (existing) {
        const z = getNextZIndex()
        return prev.map(w =>
          w.id === existing.id ? { ...w, minimized: false, zIndex: z } : w
        )
      }
      const page = allPages.find(p => p.id === appId)
      if (!page) return prev
      const pos = getDefaultPosition(prev.length)
      const newWin: WindowState = {
        id: createWindowId(),
        appId,
        title: page.label,
        x: pos.x,
        y: pos.y,
        width: 800,
        height: 560,
        minimized: false,
        maximized: false,
        zIndex: getNextZIndex(),
      }
      return [...prev, newWin]
    })
  }, [])

  const closeWindow = useCallback((id: string) => {
    setWindows(prev => prev.filter(w => w.id !== id))
  }, [])

  const minimizeWindow = useCallback((id: string) => {
    setWindows(prev => prev.map(w =>
      w.id === id ? { ...w, minimized: true } : w
    ))
  }, [])

  const maximizeWindow = useCallback((id: string) => {
    setWindows(prev => prev.map(w =>
      w.id === id ? { ...w, maximized: !w.maximized } : w
    ))
  }, [])

  const focusWindow = useCallback((id: string) => {
    setWindows(prev => {
      const z = getNextZIndex()
      return prev.map(w => w.id === id ? { ...w, zIndex: z } : w)
    })
  }, [])

  const moveWindow = useCallback((id: string, x: number, y: number) => {
    setWindows(prev => prev.map(w =>
      w.id === id ? { ...w, x, y } : w
    ))
  }, [])

  const resizeWindow = useCallback((id: string, width: number, height: number, x: number, y: number) => {
    setWindows(prev => prev.map(w =>
      w.id === id ? { ...w, width, height, x, y } : w
    ))
  }, [])

  const handleWindowClick = useCallback((id: string) => {
    setWindows(prev => {
      const win = prev.find(w => w.id === id)
      if (!win) return prev
      const z = getNextZIndex()
      if (win.minimized) {
        return prev.map(w => w.id === id ? { ...w, minimized: false, zIndex: z } : w)
      }
      return prev.map(w => w.id === id ? { ...w, zIndex: z } : w)
    })
  }, [])

  // Close panels when clicking desktop background
  function handleDesktopClick() {
    if (launcherOpen) setLauncherOpen(false)
    if (notificationsOpen) setNotificationsOpen(false)
  }

  return (
    <div
      className="relative w-full h-full overflow-hidden select-none"
      style={{
        backgroundColor: 'var(--color-surface-primary)',
        backgroundImage: 'radial-gradient(circle at 20% 50%, color-mix(in srgb, var(--color-accent) 8%, transparent) 0%, transparent 50%), radial-gradient(circle at 80% 50%, color-mix(in srgb, var(--color-accent) 5%, transparent) 0%, transparent 50%)',
      }}
      onClick={handleDesktopClick}
      onContextMenu={e => e.preventDefault()}
    >
      {/* Desktop icon grid */}
      <div className="absolute top-4 left-4 grid grid-cols-1 gap-3" style={{ zIndex: 1 }}>
        {allPages.map(page => (
          <button
            key={page.id}
            onDoubleClick={() => launchApp(page.id)}
            className="flex flex-col items-center gap-1 p-2 rounded-lg transition-colors w-16"
            style={{ color: 'var(--color-text-secondary)' }}
            onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'color-mix(in srgb, var(--color-text-primary) 10%, transparent)' }}
            onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent' }}
          >
            <page.icon size={28} />
            <span className="text-[10px] leading-tight text-center">{page.label}</span>
          </button>
        ))}
      </div>

      {/* Window area */}
      <div className="absolute inset-0 bottom-12" style={{ zIndex: 10 }}>
        {windows.map(win => {
          const PageComponent = pageComponents[win.appId]
          if (!PageComponent) return null
          return (
            <Window
              key={win.id}
              state={win}
              onClose={closeWindow}
              onMinimize={minimizeWindow}
              onMaximize={maximizeWindow}
              onFocus={focusWindow}
              onMove={moveWindow}
              onResize={resizeWindow}
            >
              <Suspense fallback={<PageFallback />}>
                <PageComponent />
              </Suspense>
            </Window>
          )
        })}
      </div>

      {/* Launcher */}
      {launcherOpen && (
        <AppLauncher
          onLaunchApp={launchApp}
          onClose={() => setLauncherOpen(false)}
        />
      )}

      {/* Notification Center */}
      {notificationsOpen && (
        <NotificationCenter
          notifications={notifications}
          unreadCount={unreadCount}
          onMarkAllRead={markAllRead}
          onDismiss={dismiss}
          onClose={() => setNotificationsOpen(false)}
        />
      )}

      {/* Taskbar */}
      <Taskbar
        windows={windows}
        onWindowClick={handleWindowClick}
        onLauncherToggle={() => { setLauncherOpen(!launcherOpen); setNotificationsOpen(false) }}
        onNotificationsToggle={() => { setNotificationsOpen(!notificationsOpen); setLauncherOpen(false) }}
        launcherOpen={launcherOpen}
        notificationsOpen={notificationsOpen}
        unreadCount={unreadCount}
      />
    </div>
  )
}
