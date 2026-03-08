import { X, CheckCircle, XCircle, Clock } from '../Icons'
import type { Notification } from '../../hooks/useEventStream'

const typeIcon: Record<string, typeof CheckCircle> = {
  success: CheckCircle,
  error: XCircle,
  warning: Clock,
  info: Clock,
}

const typeColor: Record<string, string> = {
  success: 'var(--color-success, #22c55e)',
  error: 'var(--color-error)',
  warning: '#f59e0b',
  info: 'var(--color-accent)',
}

function formatTime(ts: string): string {
  const d = new Date(ts)
  const now = Date.now()
  const diff = now - d.getTime()
  if (diff < 60_000) return 'Just now'
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)} min ago`
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)} hours ago`
  return d.toLocaleDateString()
}

type Props = {
  notifications: Notification[]
  unreadCount: number
  onMarkAllRead: () => void
  onDismiss: (id: string) => void
  onClose: () => void
}

export default function NotificationCenter({ notifications, unreadCount, onMarkAllRead, onDismiss, onClose }: Props) {
  return (
    <div
      className="absolute bottom-14 right-2 rounded-xl w-80 overflow-hidden"
      style={{
        backgroundColor: 'var(--color-surface-secondary)',
        border: '1px solid var(--color-border-primary)',
        boxShadow: '0 8px 32px rgba(0,0,0,0.15)',
        zIndex: 10000,
        maxHeight: 'calc(100vh - 80px)',
      }}
      onClick={e => e.stopPropagation()}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-4 py-3"
        style={{ borderBottom: '1px solid var(--color-border-primary)' }}
      >
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>Notifications</span>
          {unreadCount > 0 && (
            <span
              className="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
              style={{ backgroundColor: 'var(--color-error)', color: '#fff' }}
            >
              {unreadCount}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {unreadCount > 0 && (
            <button
              onClick={onMarkAllRead}
              className="text-xs px-2 py-1 rounded transition-colors"
              style={{ color: 'var(--color-accent)' }}
              onMouseEnter={e => e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'}
              onMouseLeave={e => e.currentTarget.style.backgroundColor = 'transparent'}
            >
              Mark all read
            </button>
          )}
          <button
            onClick={onClose}
            className="p-1 rounded transition-colors"
            style={{ color: 'var(--color-text-tertiary)' }}
            onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
            onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
          >
            <X size={14} />
          </button>
        </div>
      </div>

      {/* List */}
      <div className="overflow-y-auto" style={{ maxHeight: '360px' }}>
        {notifications.length === 0 ? (
          <div className="py-8 text-center text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
            No notifications
          </div>
        ) : (
          notifications.map(n => {
            const Icon = typeIcon[n.severity] || Clock
            return (
              <div
                key={n.id}
                className="group flex gap-3 px-4 py-3 transition-colors"
                style={{
                  backgroundColor: n.read ? 'transparent' : 'color-mix(in srgb, var(--color-accent) 5%, transparent)',
                  borderBottom: '1px solid var(--color-border-secondary, var(--color-border-primary))',
                }}
                onMouseEnter={e => e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'}
                onMouseLeave={e => e.currentTarget.style.backgroundColor = n.read ? 'transparent' : 'color-mix(in srgb, var(--color-accent) 5%, transparent)'}
              >
                <div className="shrink-0 mt-0.5" style={{ color: typeColor[n.severity] || typeColor.info }}>
                  <Icon size={16} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-medium truncate" style={{ color: 'var(--color-text-primary)' }}>
                      {n.title}
                    </span>
                    <button
                      onClick={() => onDismiss(n.id)}
                      className="hidden group-hover:block shrink-0 p-0.5 rounded transition-colors"
                      style={{ color: 'var(--color-text-tertiary)' }}
                      onMouseEnter={e => e.currentTarget.style.color = 'var(--color-text-primary)'}
                      onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
                    >
                      <X size={10} />
                    </button>
                  </div>
                  <div className="text-[11px] mt-0.5 truncate" style={{ color: 'var(--color-text-secondary)' }}>
                    {n.message}
                  </div>
                  <div className="text-[10px] mt-1" style={{ color: 'var(--color-text-tertiary)' }}>
                    {formatTime(n.timestamp)}
                  </div>
                </div>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
