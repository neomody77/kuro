import { allPages } from '../../lib/navConfig'
import { Layout } from '../Icons'
import { setViewPref } from '../../lib/navConfig'
import { useNavigate } from 'react-router-dom'

type Props = {
  onLaunchApp: (appId: string) => void
  onClose: () => void
}

export default function AppLauncher({ onLaunchApp, onClose }: Props) {
  const navigate = useNavigate()

  function switchToApp() {
    setViewPref('app')
    navigate('/app/chat')
  }

  return (
    <div
      className="absolute bottom-14 left-2 rounded-xl p-4 w-64"
      style={{
        backgroundColor: 'var(--color-surface-secondary)',
        border: '1px solid var(--color-border-primary)',
        boxShadow: '0 8px 32px rgba(0,0,0,0.15)',
        zIndex: 10000,
      }}
      onClick={e => e.stopPropagation()}
    >
      <div className="text-xs font-medium mb-3 px-1" style={{ color: 'var(--color-text-tertiary)' }}>
        Apps
      </div>
      <div className="grid grid-cols-3 gap-2">
        {allPages.map(page => (
          <button
            key={page.id}
            onClick={() => { onLaunchApp(page.id); onClose() }}
            className="flex flex-col items-center gap-1.5 p-2 rounded-lg transition-colors"
            style={{ color: 'var(--color-text-secondary)' }}
            onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
            onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-secondary)' }}
          >
            <page.icon size={20} />
            <span className="text-[10px]">{page.label}</span>
          </button>
        ))}
      </div>
      <div className="mt-3 pt-3" style={{ borderTop: '1px solid var(--color-border-primary)' }}>
        <button
          onClick={switchToApp}
          className="w-full flex items-center gap-2 px-2 py-1.5 rounded-md text-xs transition-colors"
          style={{ color: 'var(--color-text-tertiary)' }}
          onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
          onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
        >
          <Layout size={14} />
          Switch to App View
        </button>
      </div>
    </div>
  )
}
