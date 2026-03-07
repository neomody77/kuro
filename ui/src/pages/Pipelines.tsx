import { useState, useEffect } from 'react'
import PageHeader from '../components/PageHeader'
import { Play, Clock, CheckCircle, XCircle, Plus, Trash } from '../components/Icons'
import { api } from '../lib/api'

type Node = { id?: string; name: string; type: string; typeVersion?: number; position?: [number, number]; parameters?: Record<string, unknown> }
type ConnectionTarget = { node: string; type: string; index: number }
type NodeConnection = { main?: ConnectionTarget[][] }
type WorkflowSettings = { executionTimeout?: number; timezone?: string }
type Workflow = {
  id: string; name: string; active?: boolean;
  nodes: Node[]; connections: Record<string, NodeConnection>;
  settings?: WorkflowSettings; createdAt?: string; updatedAt?: string
}
type Execution = {
  id: string; workflowId: string; status: string; mode?: string;
  startedAt: string; stoppedAt?: string; finished?: boolean;
  data?: { resultData?: { runData?: Record<string, unknown[]>; lastNodeExecuted?: string; error?: { message: string } } }
}

function Pipelines() {
  const [pipelines, setPipelines] = useState<Workflow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<Workflow | null>(null)
  const [runs, setRuns] = useState<Execution[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [running, setRunning] = useState<string | null>(null)

  useEffect(() => { loadPipelines() }, [])

  async function loadPipelines() {
    setLoading(true)
    try {
      const data = await api.get<Workflow[]>('/api/pipelines')
      setPipelines(data || [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  async function selectPipeline(p: Workflow) {
    setSelected(p)
    try {
      const data = await api.get<Execution[]>(`/api/pipelines/${encodeURIComponent(p.id)}/runs`)
      setRuns(data || [])
    } catch {
      setRuns([])
    }
  }

  async function runPipeline(id: string) {
    setRunning(id)
    try {
      await api.post(`/api/pipelines/${encodeURIComponent(id)}/run`)
      if (selected?.id === id) {
        const data = await api.get<Execution[]>(`/api/pipelines/${encodeURIComponent(id)}/runs`)
        setRuns(data || [])
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Run failed')
    } finally {
      setRunning(null)
    }
  }

  async function deletePipeline(id: string) {
    try {
      await api.del(`/api/pipelines/${encodeURIComponent(id)}`)
      setPipelines(prev => prev.filter(p => p.id !== id))
      if (selected?.id === id) { setSelected(null); setRuns([]) }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  async function createPipeline() {
    if (!newName.trim()) return
    const w = {
      name: newName.trim(),
      nodes: [] as Node[],
      connections: {} as Record<string, NodeConnection>,
      settings: { timezone: Intl.DateTimeFormat().resolvedOptions().timeZone },
    }
    try {
      const created = await api.post<Workflow>('/api/pipelines', w)
      setPipelines(prev => [...prev, created])
      setShowCreate(false)
      setNewName('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Create failed')
    }
  }

  async function clearRuns(workflowId: string) {
    try {
      await api.post(`/api/v1/executions/clear?workflowId=${encodeURIComponent(workflowId)}`)
      setRuns([])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Clear failed')
    }
  }

  function nodeTargets(w: Workflow, nodeName: string): string[] {
    const conn = w.connections?.[nodeName]
    if (!conn?.main) return []
    return conn.main.flatMap(targets => targets.map(t => t.node))
  }

  const statusColor = (s: string) =>
    s === 'success' ? 'var(--color-success)' :
    s === 'error' || s === 'crashed' ? 'var(--color-error)' :
    'var(--color-warning)'

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Pipelines"
        description="Automated workflows"
        actions={
          <button
            onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-1.5 text-sm rounded-lg px-3 py-1.5 transition-colors"
            style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}
          >
            <Plus size={16} /> New
          </button>
        }
      />

      {error && (
        <div className="px-6 py-2 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>
      )}

      {showCreate && (
        <div className="px-6 py-4" style={{ borderBottom: '1px solid var(--color-border-primary)' }}>
          <div className="space-y-3 max-w-lg">
            <input
              type="text"
              placeholder="Pipeline name"
              value={newName}
              onChange={e => setNewName(e.target.value)}
              className="w-full rounded-lg px-3 py-2 text-sm outline-none"
              style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
              onKeyDown={e => e.key === 'Enter' && createPipeline()}
            />
            <div className="flex gap-2">
              <button onClick={createPipeline} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>Create</button>
              <button onClick={() => setShowCreate(false)} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>Cancel</button>
            </div>
          </div>
        </div>
      )}

      <div className="flex-1 flex min-h-0">
        {/* Pipeline list */}
        <div className="w-full md:w-80 overflow-y-auto" style={{ borderRight: '1px solid var(--color-border-primary)' }}>
          {loading ? (
            <div className="px-6 py-8 text-sm text-center" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
          ) : pipelines.length === 0 ? (
            <div className="px-6 py-8 text-sm text-center" style={{ color: 'var(--color-text-tertiary)' }}>No pipelines yet</div>
          ) : (
            pipelines.map(p => (
              <div
                key={p.id}
                onClick={() => selectPipeline(p)}
                className="flex items-center justify-between px-6 py-4 cursor-pointer transition-colors"
                style={{
                  backgroundColor: selected?.id === p.id ? 'var(--color-surface-active)' : 'transparent',
                  borderBottom: '1px solid var(--color-border-secondary)',
                }}
                onMouseEnter={e => { if (selected?.id !== p.id) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)' }}
                onMouseLeave={e => { if (selected?.id !== p.id) e.currentTarget.style.backgroundColor = 'transparent' }}
              >
                <div className="min-w-0">
                  <div className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>{p.name}</div>
                  <div className="text-xs flex items-center gap-1.5 mt-0.5" style={{ color: 'var(--color-text-tertiary)' }}>
                    <Clock size={12} />
                    {p.nodes?.length || 0} nodes
                    {p.settings?.timezone ? ` · ${p.settings.timezone}` : ''}
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <button
                    onClick={e => { e.stopPropagation(); runPipeline(p.id) }}
                    className="p-1.5 rounded-lg transition-colors"
                    style={{ color: running === p.id ? 'var(--color-accent)' : 'var(--color-text-tertiary)' }}
                    disabled={running === p.id}
                  >
                    <Play size={16} />
                  </button>
                  <button
                    onClick={e => { e.stopPropagation(); deletePipeline(p.id) }}
                    className="p-1.5 rounded-lg transition-colors"
                    style={{ color: 'var(--color-text-tertiary)' }}
                    onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
                    onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
                  >
                    <Trash size={14} />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>

        {/* Detail panel */}
        <div className="hidden md:flex flex-1 flex-col overflow-y-auto">
          {selected ? (
            <div className="p-6 space-y-6">
              <div>
                <h2 className="text-lg font-semibold" style={{ color: 'var(--color-text-primary)' }}>{selected.name}</h2>
                <div className="text-xs mt-2 flex gap-3" style={{ color: 'var(--color-text-tertiary)' }}>
                  {selected.settings?.timezone && <span>TZ: {selected.settings.timezone}</span>}
                  {selected.active !== undefined && <span>{selected.active ? 'Active' : 'Inactive'}</span>}
                  {selected.createdAt && <span>Created: {new Date(selected.createdAt).toLocaleDateString()}</span>}
                </div>
              </div>

              {/* Nodes */}
              <div>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Nodes ({selected.nodes?.length || 0})</h3>
                <div className="space-y-2">
                  {(selected.nodes || []).map((node, i) => {
                    const targets = nodeTargets(selected, node.name)
                    return (
                      <div key={node.name || i} className="rounded-lg p-3" style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-secondary)' }}>
                        <div className="flex items-center justify-between">
                          <span className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>{node.name}</span>
                          <span className="text-xs px-2 py-0.5 rounded-full" style={{ backgroundColor: 'var(--color-surface-active)', color: 'var(--color-text-secondary)' }}>{node.type}</span>
                        </div>
                        {targets.length > 0 && (
                          <div className="text-xs mt-1" style={{ color: 'var(--color-text-tertiary)' }}>→ {targets.join(', ')}</div>
                        )}
                        {node.parameters && Object.keys(node.parameters).length > 0 && (
                          <div className="text-xs mt-1 font-mono" style={{ color: 'var(--color-text-tertiary)' }}>
                            {Object.entries(node.parameters).map(([k, v]) => (
                              <div key={k} className="truncate">{k}: {typeof v === 'string' ? v : JSON.stringify(v)}</div>
                            ))}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* Run history */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>Run History</h3>
                  {runs.length > 0 && (
                    <button
                      onClick={() => clearRuns(selected.id)}
                      className="flex items-center gap-1 text-xs px-2 py-1 rounded-md transition-colors"
                      style={{ color: 'var(--color-text-tertiary)' }}
                      onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
                      onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
                      title="Clear all runs"
                    >
                      <Trash size={12} /> Clear
                    </button>
                  )}
                </div>
                {runs.length === 0 ? (
                  <div className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>No runs yet</div>
                ) : (
                  <div className="space-y-2">
                    {runs.map(run => (
                      <div key={run.id} className="rounded-lg p-3" style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-secondary)' }}>
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            {run.status === 'success' ? <CheckCircle size={14} /> : <XCircle size={14} />}
                            <span className="text-xs font-mono" style={{ color: statusColor(run.status) }}>{run.status}</span>
                          </div>
                          <span className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                            {new Date(run.startedAt).toLocaleString()}
                          </span>
                        </div>
                        {run.data?.resultData?.error && (
                          <div className="text-xs mt-1" style={{ color: 'var(--color-error)' }}>{run.data.resultData.error.message}</div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center">
                <div className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Select a pipeline to view details</div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default Pipelines
