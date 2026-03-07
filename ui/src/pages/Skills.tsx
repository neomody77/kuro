import { useState, useEffect } from 'react'
import PageHeader from '../components/PageHeader'
import { Zap, Search, ChevronRight } from '../components/Icons'
import { api } from '../lib/api'

type SkillParam = { name: string; type?: string; required?: boolean; default?: string }
type Skill = { name: string; description?: string; inputs?: SkillParam[]; outputs?: SkillParam[]; nodes?: Record<string, unknown> }

function Skills() {
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<Skill | null>(null)

  useEffect(() => {
    api.get<Skill[]>('/api/skills')
      .then(data => setSkills(Array.isArray(data) ? data : []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const filtered = skills.filter(s =>
    !search || s.name.toLowerCase().includes(search.toLowerCase()) ||
    s.description?.toLowerCase().includes(search.toLowerCase())
  )

  if (selected) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title={selected.name}
          description={selected.description}
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
            {selected.inputs && selected.inputs.length > 0 && (
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Inputs</h3>
                <div className="space-y-2">
                  {selected.inputs.map(p => (
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
            {selected.outputs && selected.outputs.length > 0 && (
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Outputs</h3>
                <div className="space-y-2">
                  {selected.outputs.map(p => (
                    <div key={p.name} className="rounded-lg p-3" style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-secondary)' }}>
                      <span className="text-sm font-mono" style={{ color: 'var(--color-text-primary)' }}>{p.name}</span>
                      {p.type && <span className="text-xs ml-2" style={{ color: 'var(--color-text-tertiary)' }}>{p.type}</span>}
                    </div>
                  ))}
                </div>
              </section>
            )}
            {selected.nodes && Object.keys(selected.nodes).length > 0 && (
              <section>
                <h3 className="text-sm font-medium mb-3" style={{ color: 'var(--color-text-primary)' }}>Nodes</h3>
                <pre className="rounded-lg p-3 text-xs overflow-x-auto" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}>
                  {JSON.stringify(selected.nodes, null, 2)}
                </pre>
              </section>
            )}
          </div>
        </div>
      </div>
    )
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
                onClick={() => setSelected(skill)}
                className="rounded-xl p-4 cursor-pointer transition-colors"
                style={{ backgroundColor: 'var(--color-surface-secondary)', border: '1px solid var(--color-border-primary)' }}
                onMouseEnter={e => e.currentTarget.style.borderColor = 'var(--color-border-focus)'}
                onMouseLeave={e => e.currentTarget.style.borderColor = 'var(--color-border-primary)'}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="p-2 rounded-lg" style={{ backgroundColor: 'var(--color-surface-tertiary)' }}>
                    <Zap size={16} style={{ color: 'var(--color-accent)' }} />
                  </div>
                  <ChevronRight size={14} style={{ color: 'var(--color-text-tertiary)' }} />
                </div>
                <h3 className="text-sm font-medium mt-3" style={{ color: 'var(--color-text-primary)' }}>{skill.name}</h3>
                {skill.description && <p className="text-xs mt-1" style={{ color: 'var(--color-text-tertiary)' }}>{skill.description}</p>}
                {skill.inputs && skill.inputs.length > 0 && (
                  <div className="text-xs mt-2" style={{ color: 'var(--color-text-quaternary)' }}>
                    {skill.inputs.length} input{skill.inputs.length !== 1 ? 's' : ''}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export default Skills
