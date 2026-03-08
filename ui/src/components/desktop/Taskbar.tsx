import { useState, useEffect } from 'react'
import { Grid, Bell, Sun, Moon } from '../Icons'
import { useTheme } from '../../hooks/useTheme'
import type { WindowState } from './Window'

type Props = {
  windows: WindowState[]
  onWindowClick: (id: string) => void
  onLauncherToggle: () => void
  onNotificationsToggle: () => void
  launcherOpen: boolean
  notificationsOpen: boolean
  unreadCount?: number
}

function Clock() {
  const [time, setTime] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setTime(new Date()), 60_000)
    return () => clearInterval(id)
  }, [])
  const h = time.getHours().toString().padStart(2, '0')
  const m = time.getMinutes().toString().padStart(2, '0')
  return <span className="text-xs font-mono" style={{ color: 'var(--color-text-secondary)' }}>{h}:{m}</span>
}

export default function Taskbar({ windows, onWindowClick, onLauncherToggle, onNotificationsToggle, launcherOpen, notificationsOpen, unreadCount = 0 }: Props) {
  const { theme, toggle } = useTheme()

  return (
    <div
      className="absolute bottom-0 left-0 right-0 h-12 flex items-center px-2 gap-1 backdrop-blur-md"
      style={{
        backgroundColor: 'color-mix(in srgb, var(--color-surface-secondary) 85%, transparent)',
        borderTop: '1px solid var(--color-border-primary)',
        zIndex: 9999,
      }}
    >
      {/* App Launcher */}
      <button
        onClick={onLauncherToggle}
        className="p-2 rounded-lg transition-colors shrink-0"
        style={{
          color: launcherOpen ? 'var(--color-accent)' : 'var(--color-text-secondary)',
          backgroundColor: launcherOpen ? 'var(--color-surface-active)' : 'transparent',
        }}
        onMouseEnter={e => { if (!launcherOpen) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)' }}
        onMouseLeave={e => { if (!launcherOpen) e.currentTarget.style.backgroundColor = 'transparent' }}
      >
        <Grid size={18} />
      </button>

      {/* Separator */}
      <div className="w-px h-6 mx-1" style={{ backgroundColor: 'var(--color-border-primary)' }} />

      {/* Open windows */}
      <div className="flex-1 flex items-center gap-1 overflow-x-auto">
        {windows.map(w => (
          <button
            key={w.id}
            onClick={() => onWindowClick(w.id)}
            className="px-3 py-1.5 rounded-md text-xs truncate max-w-[160px] transition-colors"
            style={{
              backgroundColor: w.minimized ? 'transparent' : 'var(--color-surface-active)',
              color: w.minimized ? 'var(--color-text-tertiary)' : 'var(--color-text-primary)',
              border: '1px solid var(--color-border-primary)',
            }}
            onMouseEnter={e => { if (w.minimized) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)' }}
            onMouseLeave={e => { if (w.minimized) e.currentTarget.style.backgroundColor = 'transparent' }}
          >
            {w.title}
          </button>
        ))}
      </div>

      {/* System tray */}
      <div className="flex items-center gap-1 shrink-0">
        <button
          onClick={onNotificationsToggle}
          className="p-2 rounded-lg transition-colors relative"
          style={{
            color: notificationsOpen ? 'var(--color-accent)' : 'var(--color-text-tertiary)',
            backgroundColor: notificationsOpen ? 'var(--color-surface-active)' : 'transparent',
          }}
          onMouseEnter={e => { if (!notificationsOpen) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)' }}
          onMouseLeave={e => { if (!notificationsOpen) e.currentTarget.style.backgroundColor = 'transparent' }}
        >
          <Bell size={16} />
          {unreadCount > 0 && (
            <span
              className="absolute top-1 right-1 w-2 h-2 rounded-full"
              style={{ backgroundColor: 'var(--color-error)' }}
            />
          )}
        </button>
        <button
          onClick={toggle}
          className="p-2 rounded-lg transition-colors"
          style={{ color: 'var(--color-text-tertiary)' }}
          onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
          onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
        >
          {theme === 'light' ? <Moon size={16} /> : <Sun size={16} />}
        </button>
        <Clock />
      </div>
    </div>
  )
}
