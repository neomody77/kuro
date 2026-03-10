import { useState, useEffect } from 'react'
import PageHeader from '../components/PageHeader'
import { Zap, Search, ChevronRight } from '../components/Icons'
import { api } from '../lib/api'

type SkillParam = { name: string; type?: string; required?: boolean; default?: string }
type Skill = {
  name: string
  description?: string
  inputs?: SkillParam[]
  outputs?: SkillParam[]
  config?: SkillParam[]
  command?: string
  endpoint?: string
  on?: string[]
  require?: { env?: string[]; bins?: string[]; os?: string[] }
  destructive?: boolean
  source?: string
  nodes?: Record<string, unknown>
}

type SkillListItem = {
  name: string
  description?: string
  source?: string
  destructive?: boolean
  has_config?: boolean
}

function SkillDetail({ skill, onBack }: { skill: Skill; onBack: () => void }) {
  const [configValues, setConfigValues] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [showSecrets, setShowSecrets] = useState<Record<string, boolean>>({})

  useEffect(() => {
    if (skill.config && skill.config.length > 0) {
      api.get<Record<string, string>>(`/api/skills/${skill.name}/config`)
        .then(data => setConfigValues(data || {}))
        .catch(() => {})
    }
  }, [skill.name, skill.config])

  async function saveConfig() {
    setSaving(true)
    try {
      await api.put(`/api/skills/${skill.name}/config`, configValues)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch {
      // ignore
    } finally {
      setSaving(false)
    }
  }

  const sourceLabel: Record<string, string> = {
    builtin: 'Built-in',
    global: 'Global',
    workspace: 'Workspace',
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={skill.name}
        description={skill.description}
        actions={
          <button
            onClick={onBack}
            className="text-sm rounded-lg px-3 py-1.5 transition-colors"
            style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}
          >
            Back
          </button>
        }
      />
      <div className="flex-1 overflow-y-auto p-6">
        <div className="max-w-2xl space-y-6">

          {/* Meta badges */}
          <div className="flex flex-wrap gap-2">
            {skill.source && (
              <span className="text-xs px-2 py-1 rounded-full" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                {sourceLabel[skill.source] || skill.source}
              </span>
            )}
            {skill.destructive && (
              <span className="text-xs px-2 py-1 rounded-full" style={{ backgroundColor: '#7f1d1d', color: '#fca5a5' }}>
                Destructive
              </span>
            )}
            {skill.command && (
              <span className="text-xs px-2 py-1 rounded-full" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                Shell
              </span>
            )}
            {skill.endpoint && (
              <span className="text-xs px-2 py-1 rounded-full" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                HTTP
              </span>
            )}
          </div>

          {/* Command / Endpoint */}
          {skill.command && (
            <section>
              <h3 className="text-sm font-medium mb-2" style={{ color: 'var(--color-text-primary)' }}>Command</h3>
              <pre className="rounded-lg p-3 text-xs overflow-x-auto font-mono" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                {skill.command}
              </pre>
            </section>
          )}
          {skill.endpoint && (
            <section>
              <h3 className="text-sm font-medium mb-2" style={{ color: 'var(--color-text-primary)' }}>Endpoint</h3>
              <pre className="rounded-lg p-3 text-xs overflow-x-auto font-mono" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                {skill.endpoint}
              </pre>
            </section>
          )}

          {/* Event triggers */}
          {skill.on && skill.on.length > 0 && (
            <section>
              <h3 className="text-sm font-medium mb-2" style={{ color: 'var(--color-text-primary)' }}>Event Triggers</h3>
              <div className="flex flex-wrap gap-2">
                {skill.on.map(ev => (
                  <span key={ev} className="text-xs px-2 py-1 rounded font-mono" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                    {ev}
                  </span>
                ))}
              </div>
            </section>
          )}

          {/* Requirements */}
          {skill.require && (
            <section>
              <h3 className="text-sm font-medium mb-2" style={{ color: 'var(--color-text-primary)' }}>Requirements</h3>
              <div className="space-y-1 text-xs" style={{ color: 'var(--color-text-secondary)' }}>
                {skill.require.env && skill.require.env.length > 0 && (
                  <div>Env: <span className="font-mono">{skill.require.env.join(', ')}</span></div>
                )}
                {skill.require.bins && skill.require.bins.length > 0 && (
                  <div>Bins: <span className="font-mono">{skill.require.bins.join(', ')}</span></div>
                )}
                {skill.require.os && skill.require.os.length > 0 && (
                  <div>OS: <span className="font-mono">{skill.require.os.join(', ')}</span></div>
                )}
              </div>
            </section>
          )}

          {/* Inputs */}
          {skill.inputs && skill.inputs.length > 0 && (
            <section>
              <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Inputs</h3>
              <div className="space-y-2">
                {skill.inputs.map(p => (
                  <div key={p.name} className="rounded-lg p-3" style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-secondary)' }}>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-mono" style={{ color: 'var(--color-text-primary)' }}>{p.name}</span>
                      {p.required && <span className="text-xs px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>required</span>}
                      {p.type && <span className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>{p.type}</span>}
                    </div>
                    {p.default && <div className="text-xs mt-1" style={{ color: 'var(--color-text-tertiary)' }}>Default: {p.default}</div>}
                  </div>
                ))}
              </div>
            </section>
          )}

          {/* Outputs */}
          {skill.outputs && skill.outputs.length > 0 && (
            <section>
              <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Outputs</h3>
              <div className="space-y-2">
                {skill.outputs.map(p => (
                  <div key={p.name} className="rounded-lg p-3" style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-secondary)' }}>
                    <span className="text-sm font-mono" style={{ color: 'var(--color-text-primary)' }}>{p.name}</span>
                    {p.type && <span className="text-xs ml-2" style={{ color: 'var(--color-text-tertiary)' }}>{p.type}</span>}
                  </div>
                ))}
              </div>
            </section>
          )}

          {/* Configuration */}
          {skill.config && skill.config.length > 0 && (
            <section>
              <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Configuration</h3>
              <div className="space-y-3">
                {skill.config.map(c => {
                  const isSecret = c.type === 'password'
                  const show = showSecrets[c.name] || false
                  return (
                    <div key={c.name}>
                      <label className="block text-xs font-medium mb-1" style={{ color: 'var(--color-text-secondary)' }}>
                        {c.name}
                        {c.required && <span style={{ color: '#f87171' }}> *</span>}
                      </label>
                      <div className="flex gap-2">
                        <input
                          type={isSecret && !show ? 'password' : 'text'}
                          value={configValues[c.name] || ''}
                          onChange={e => setConfigValues(prev => ({ ...prev, [c.name]: e.target.value }))}
                          placeholder={c.default || ''}
                          className="flex-1 rounded-lg px-3 py-2 text-sm outline-none transition-colors font-mono"
                          style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
                          onFocus={e => e.currentTarget.style.borderColor = 'var(--color-border-focus)'}
                          onBlur={e => e.currentTarget.style.borderColor = 'var(--color-border-primary)'}
                        />
                        {isSecret && (
                          <button
                            onClick={() => setShowSecrets(prev => ({ ...prev, [c.name]: !show }))}
                            className="text-xs px-2 rounded-lg transition-colors"
                            style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}
                          >
                            {show ? 'Hide' : 'Show'}
                          </button>
                        )}
                      </div>
                    </div>
                  )
                })}
                <div className="flex items-center gap-3 pt-1">
                  <button
                    onClick={saveConfig}
                    disabled={saving}
                    className="text-sm rounded-lg px-4 py-2 transition-colors"
                    style={{
                      backgroundColor: 'var(--color-accent)',
                      color: 'var(--color-accent-text)',
                      opacity: saving ? 0.5 : 1,
                    }}
                  >
                    {saving ? 'Saving...' : 'Save'}
                  </button>
                  {saved && <span className="text-xs" style={{ color: '#4ade80' }}>Saved</span>}
                </div>
              </div>
            </section>
          )}

          {/* Workflow nodes (legacy) */}
          {skill.nodes && Object.keys(skill.nodes).length > 0 && (
            <section>
              <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Nodes</h3>
              <pre className="rounded-lg p-3 text-xs overflow-x-auto" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                {JSON.stringify(skill.nodes, null, 2)}
              </pre>
            </section>
          )}
        </div>
      </div>
    </div>
  )
}

function Skills() {
  const [skills, setSkills] = useState<SkillListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<Skill | null>(null)

  useEffect(() => {
    api.get<SkillListItem[]>('/api/skills')
      .then(data => setSkills(Array.isArray(data) ? data : []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  function openSkill(name: string) {
    api.get<Skill>(`/api/skills/${name}`)
      .then(data => setSelected(data))
      .catch(() => {})
  }

  const filtered = skills.filter(s =>
    !search || s.name.toLowerCase().includes(search.toLowerCase()) ||
    s.description?.toLowerCase().includes(search.toLowerCase())
  )

  const sourceColor: Record<string, string> = {
    builtin: 'var(--color-text-tertiary)',
    global: '#60a5fa',
    workspace: '#a78bfa',
  }

  if (selected) {
    return <SkillDetail skill={selected} onBack={() => setSelected(null)} />
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader title="Skills" description="Reusable actions & pipeline fragments" />

      <div className="flex-1 overflow-y-auto p-6">
        <div className="relative mb-6">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: 'var(--color-text-tertiary)' }} />
          <input
            type="text"
            placeholder="Search skills..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full rounded-lg pl-9 pr-4 py-2 text-sm outline-none transition-colors"
            style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
            onFocus={e => e.currentTarget.style.borderColor = 'var(--color-border-focus)'}
            onBlur={e => e.currentTarget.style.borderColor = 'var(--color-border-primary)'}
          />
        </div>

        {loading ? (
          <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
        ) : filtered.length === 0 ? (
          <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>
            {search ? 'No matching skills' : 'No skills registered'}
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {filtered.map(skill => (
              <div
                key={skill.name}
                onClick={() => openSkill(skill.name)}
                className="rounded-xl p-4 cursor-pointer transition-colors"
                style={{ backgroundColor: 'var(--color-surface-secondary)', border: '1px solid var(--color-border-primary)' }}
                onMouseEnter={e => e.currentTarget.style.borderColor = 'var(--color-border-focus)'}
                onMouseLeave={e => e.currentTarget.style.borderColor = 'var(--color-border-primary)'}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="p-2 rounded-lg" style={{ backgroundColor: 'var(--color-surface-tertiary)' }}>
                    <Zap size={16} style={{ color: 'var(--color-accent)' }} />
                  </div>
                  <div className="flex items-center gap-2">
                    {skill.source && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded-full" style={{ color: sourceColor[skill.source] || 'var(--color-text-tertiary)' }}>
                        {skill.source}
                      </span>
                    )}
                    <ChevronRight size={14} style={{ color: 'var(--color-text-tertiary)' }} />
                  </div>
                </div>
                <h3 className="text-sm font-medium mt-3" style={{ color: 'var(--color-text-primary)' }}>{skill.name}</h3>
                {skill.description && <p className="text-xs mt-1 line-clamp-2" style={{ color: 'var(--color-text-tertiary)' }}>{skill.description}</p>}
                <div className="flex items-center gap-2 mt-2">
                  {skill.has_config && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-quaternary)' }}>
                      configurable
                    </span>
                  )}
                  {skill.destructive && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded" style={{ backgroundColor: '#7f1d1d', color: '#fca5a5' }}>
                      destructive
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export default Skills
