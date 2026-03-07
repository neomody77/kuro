import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import Chat from './pages/Chat'
import Pipelines from './pages/Pipelines'
import Skills from './pages/Skills'
import Documents from './pages/Documents'
import Vault from './pages/Vault'
import Logs from './pages/Logs'
import Settings from './pages/Settings'

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Navigate to="/chat" replace />} />
        <Route path="/chat" element={<Chat />} />
        <Route path="/pipelines" element={<Pipelines />} />
        <Route path="/skills" element={<Skills />} />
        <Route path="/documents" element={<Documents />} />
        <Route path="/vault" element={<Vault />} />
        <Route path="/logs" element={<Logs />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
    </Routes>
  )
}

export default App
