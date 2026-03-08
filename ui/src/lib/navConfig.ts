import {
  MessageSquare,
  GitBranch,
  Zap,
  FileText,
  KeyRound,
  ScrollText,
  Settings,
} from '../components/Icons'

export type NavItem = {
  id: string
  to: string
  label: string
  icon: React.ComponentType<{ size?: number; className?: string; style?: React.CSSProperties }>
}

export const allPages: NavItem[] = [
  { id: 'chat', to: '/app/chat', label: 'Chat', icon: MessageSquare },
  { id: 'pipelines', to: '/app/pipelines', label: 'Pipelines', icon: GitBranch },
  { id: 'skills', to: '/app/skills', label: 'Skills', icon: Zap },
  { id: 'documents', to: '/app/documents', label: 'Documents', icon: FileText },
  { id: 'vault', to: '/app/vault', label: 'Vault', icon: KeyRound },
  { id: 'logs', to: '/app/logs', label: 'Logs', icon: ScrollText },
  { id: 'settings', to: '/app/settings', label: 'Settings', icon: Settings },
]

export const VIEW_PREF_KEY = 'kuro:viewPref'

export function getViewPref(): 'app' | 'desktop' {
  try {
    const v = localStorage.getItem(VIEW_PREF_KEY)
    if (v === 'desktop') return 'desktop'
  } catch { /* */ }
  return 'app'
}

export function setViewPref(pref: 'app' | 'desktop') {
  try { localStorage.setItem(VIEW_PREF_KEY, pref) } catch { /* */ }
}
