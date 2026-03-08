import { Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import AppLayout from './components/AppLayout'
import Chat from './pages/Chat'
import Pipelines from './pages/Pipelines'
import Skills from './pages/Skills'
import Vault from './pages/Vault'
import Logs from './pages/Logs'
import Settings from './pages/Settings'
import { getViewPref } from './lib/navConfig'

const Documents = lazy(() => import('./pages/Documents'))
const Desktop = lazy(() => import('./components/desktop/Desktop'))

function LoadingFallback() {
  return (
    <div className="flex-1 flex items-center justify-center h-full text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
      Loading...
    </div>
  )
}

function RootRedirect() {
  const pref = getViewPref()
  return <Navigate to={pref === 'desktop' ? '/desktop' : '/app/chat'} replace />
}

function App() {
  return (
    <Routes>
      {/* Root redirect based on view preference */}
      <Route path="/" element={<RootRedirect />} />

      {/* App view */}
      <Route path="/app" element={<AppLayout />}>
        <Route index element={<Navigate to="/app/chat" replace />} />
        <Route path="chat" element={<Chat />} />
        <Route path="pipelines" element={<Pipelines />} />
        <Route path="skills" element={<Skills />} />
        <Route path="documents" element={<Suspense fallback={<LoadingFallback />}><Documents /></Suspense>} />
        <Route path="vault" element={<Vault />} />
        <Route path="logs" element={<Logs />} />
        <Route path="settings" element={<Settings />} />
      </Route>

      {/* Desktop view */}
      <Route path="/desktop" element={
        <Suspense fallback={<LoadingFallback />}>
          <Desktop />
        </Suspense>
      } />

      {/* Legacy routes redirect to /app/* */}
      <Route path="/chat" element={<Navigate to="/app/chat" replace />} />
      <Route path="/pipelines" element={<Navigate to="/app/pipelines" replace />} />
      <Route path="/skills" element={<Navigate to="/app/skills" replace />} />
      <Route path="/documents" element={<Navigate to="/app/documents" replace />} />
      <Route path="/vault" element={<Navigate to="/app/vault" replace />} />
      <Route path="/logs" element={<Navigate to="/app/logs" replace />} />
      <Route path="/settings" element={<Navigate to="/app/settings" replace />} />

      {/* Catch-all */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default App
