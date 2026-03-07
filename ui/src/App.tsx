import { Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import Layout from './components/Layout'
import Chat from './pages/Chat'
import Pipelines from './pages/Pipelines'
import Skills from './pages/Skills'
import Vault from './pages/Vault'
import Logs from './pages/Logs'
import Settings from './pages/Settings'

const Documents = lazy(() => import('./pages/Documents'))

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Navigate to="/chat" replace />} />
        <Route path="/chat" element={<Chat />} />
        <Route path="/pipelines" element={<Pipelines />} />
        <Route path="/skills" element={<Skills />} />
        <Route path="/documents" element={<Suspense fallback={<div className="flex-1 flex items-center justify-center text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>}><Documents /></Suspense>} />
        <Route path="/vault" element={<Vault />} />
        <Route path="/logs" element={<Logs />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
    </Routes>
  )
}

export default App
