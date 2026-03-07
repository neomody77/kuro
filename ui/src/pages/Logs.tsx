import { useState, useEffect } from 'react'
import PageHeader from '../components/PageHeader'
import { CheckCircle, XCircle, Clock, Search } from '../components/Icons'
import { api } from '../lib/api'

type NodeResult = { node_id: string; status: string; output?: unknown; error?: string; duration: number }
type RunState = { id: string; pipeline_id: string; status: string; started_at: string; finished_at?: string; node_results: Record<string, NodeResult> }

function Logs() {
  const [runs, setRuns] = useState<RunState[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')
  const [selected, setSelected] = useState<RunState | null>(null)

  useEffect(() => {
    api.get<RunState[]>('/api/logs')
      .then(data => setRuns(data || []))
      .catch(() => setRuns([]))
      .finally(() => setLoading(false))
  }, [])

  async function selectRun(run: RunState) {
    try {
      const detail = await api.get<RunState>(`/api/logs/${run.id}`)
      setSelected(detail)
    } catch {
      setSelected(run)
    }
  }

  const filtered = runs.filter(r =>
    !filter ||
    r.pipeline_id.toLowerCase().includes(filter.toLowerCase()) ||
    r.id.toLowerCase().includes(filter.toLowerCase()) ||
    r.status.toLowerCase().includes(filter.toLowerCase())
  )

  const statusColor = (s: string) => s === 'completed' ? 'var(--color-success)' : s === 'failed' ? 'var(--color-error)' : 'var(--color-warning)'

  function formatDuration(ns: number): string {
    if (ns === 0) return '-'
    const ms = ns / 1_000_000
    if (ms < 1000) return `${ms.toFixed(0)}ms`
    return `${(ms / 1000).toFixed(1)}s`
  }

  if (selected) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title={`Run ${selected.id.slice(0, 12)}`}
          description={`Pipeline: ${selected.pipeline_id}`}
          actions={
            <button
              onClick={() => setSelected(null)}
              className="text-sm rounded-lg px-3 py-1.5 transition-colors"
              style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}
            >
              Back
            </button>
          }
        />
        <div className="flex-1 overflow-y-auto p-6">
          <div className="max-w-2xl space-y-6">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              <div>
                <div className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>Status</div>
                <div className="text-sm font-medium mt-1" style={{ color: statusColor(selected.status) }}>{selected.status}</div>
              </div>
              <div>
                <div className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>Started</div>
                <div className="text-sm mt-1" style={{ color: 'var(--color-text-primary)' }}>{new Date(selected.started_at).toLocaleString()}</div>
              </div>
              <div>
                <div className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>Finished</div>
                <div className="text-sm mt-1" style={{ color: 'var(--color-text-primary)' }}>{selected.finished_at ? new Date(selected.finished_at).toLocaleString() : '-'}</div>
              </div>
              <div>
                <div className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>Nodes</div>
                <div className="text-sm mt-1" style={{ color: 'var(--color-text-primary)' }}>{selected.node_results ? Object.keys(selected.node_results).length : 0}</div>
              </div>
            </div>

            {selected.node_results && Object.keys(selected.node_results).length > 0 && (
              <div>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Node Results</h3>
                <div className="space-y-2">
                  {Object.entries(selected.node_results).map(([id, nr]) => (
                    <div key={id} className="rounded-lg p-4" style={{ backgroundColor: 'var(--color-surface-secondary)', border: '1px solid var(--color-border-primary)' }}>
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          {nr.status === 'completed' ? <CheckCircle size={14} /> : nr.status === 'failed' ? <XCircle size={14} /> : <Clock size={14} />}
                          <span className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>{id}</span>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="text-xs" style={{ color: statusColor(nr.status) }}>{nr.status}</span>
                          <span className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>{formatDuration(nr.duration)}</span>
                        </div>
                      </div>
                      {nr.error && (
                        <div className="mt-2 text-xs font-mono p-2 rounded" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-error)' }}>{nr.error}</div>
                      )}
                      {nr.output != null && (
                        <pre className="mt-2 text-xs font-mono p-2 rounded overflow-x-auto" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                          {String(typeof nr.output === 'string' ? nr.output : JSON.stringify(nr.output, null, 2))}
                        </pre>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader title="Logs" description="Execution history" />

      <div className="flex-1 overflow-y-auto">
        <div className="px-6 py-4">
          <div className="relative">
            <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: 'var(--color-text-tertiary)' }} />
            <input
              type="text"
              placeholder="Filter by pipeline, run ID, or status..."
              value={filter}
              onChange={e => setFilter(e.target.value)}
              className="w-full rounded-lg pl-9 pr-4 py-2 text-sm outline-none transition-colors"
              style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
              onFocus={e => e.currentTarget.style.borderColor = 'var(--color-border-focus)'}
              onBlur={e => e.currentTarget.style.borderColor = 'var(--color-border-primary)'}
            />
          </div>
        </div>

        <div className="px-6">
          {loading ? (
            <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
          ) : filtered.length === 0 ? (
            <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>
              {filter ? 'No matching runs' : 'No pipeline runs yet'}
            </div>
          ) : (
            <table className="w-full">
              <thead>
                <tr className="text-xs" style={{ color: 'var(--color-text-tertiary)', borderBottom: '1px solid var(--color-border-primary)' }}>
                  <th className="text-left py-2 font-medium">Status</th>
                  <th className="text-left py-2 font-medium">Pipeline</th>
                  <th className="text-left py-2 font-medium hidden sm:table-cell">Run ID</th>
                  <th className="text-left py-2 font-medium hidden md:table-cell">Nodes</th>
                  <th className="text-left py-2 font-medium">Time</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map(run => (
                  <tr
                    key={run.id}
                    onClick={() => selectRun(run)}
                    className="cursor-pointer transition-colors"
                    style={{ borderBottom: '1px solid var(--color-border-secondary)' }}
                    onMouseEnter={e => e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'}
                    onMouseLeave={e => e.currentTarget.style.backgroundColor = 'transparent'}
                  >
                    <td className="py-3">
                      {run.status === 'completed' ? (
                        <CheckCircle size={16} style={{ color: 'var(--color-success)' }} />
                      ) : (
                        <XCircle size={16} style={{ color: 'var(--color-error)' }} />
                      )}
                    </td>
                    <td className="py-3 text-sm" style={{ color: 'var(--color-text-primary)' }}>{run.pipeline_id}</td>
                    <td className="py-3 text-xs font-mono hidden sm:table-cell" style={{ color: 'var(--color-text-tertiary)' }}>{run.id.slice(0, 12)}</td>
                    <td className="py-3 text-xs hidden md:table-cell" style={{ color: 'var(--color-text-tertiary)' }}>
                      {run.node_results ? Object.keys(run.node_results).length : 0} nodes
                    </td>
                    <td className="py-3 text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                      {new Date(run.started_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}

export default Logs
