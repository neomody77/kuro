import { useState, useEffect } from 'react'
import PageHeader from '../components/PageHeader'
import { Shield, EyeOff, Trash, Plus, Edit2 } from '../components/Icons'
import { api } from '../lib/api'

type Credential = { id: string; name: string; type: string; data: Record<string, string> }

const credentialTypes: Record<string, string[]> = {
  email: ['imap_host', 'imap_user', 'imap_pass', 'smtp_host', 'smtp_user', 'smtp_pass'],
  'http-basic': ['username', 'password'],
  'http-bearer': ['token'],
  openai: ['api_key'],
  anthropic: ['api_key'],
  'telegram-bot': ['bot_token'],
  generic: [],
}

const typeColors: Record<string, { bg: string; text: string }> = {
  email: { bg: 'color-mix(in srgb, var(--color-accent) 15%, transparent)', text: 'var(--color-accent)' },
  openai: { bg: 'color-mix(in srgb, var(--color-success) 15%, transparent)', text: 'var(--color-success)' },
  anthropic: { bg: 'color-mix(in srgb, var(--color-warning) 15%, transparent)', text: 'var(--color-warning)' },
}

function Vault() {
  const [creds, setCreds] = useState<Credential[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [formName, setFormName] = useState('')
  const [formType, setFormType] = useState('email')
  const [formData, setFormData] = useState<Record<string, string>>({})
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => { loadCreds() }, [])

  async function loadCreds() {
    setLoading(true)
    try {
      const data = await api.get<Credential[]>('/api/credentials')
      setCreds(data || [])
    } catch {
      setCreds([])
    } finally {
      setLoading(false)
    }
  }

  function openCreate() {
    setEditingId(null)
    setFormName('')
    setFormType('email')
    setFormData({})
    setShowForm(true)
    setError('')
  }

  function openEdit(cred: Credential) {
    setEditingId(cred.id)
    setFormName(cred.name)
    setFormType(cred.type)
    setFormData({ ...cred.data })
    setShowForm(true)
    setError('')
  }

  async function saveCred() {
    if (!formName.trim()) { setError('Name is required'); return }
    setSaving(true)
    setError('')
    try {
      if (editingId) {
        await api.put(`/api/credentials/${encodeURIComponent(editingId)}`, { name: formName.trim(), type: formType, data: formData })
      } else {
        await api.post('/api/credentials', { name: formName.trim(), type: formType, data: formData })
      }
      setShowForm(false)
      await loadCreds()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function deleteCred(id: string) {
    try {
      await api.del(`/api/credentials/${encodeURIComponent(id)}`)
      setCreds(prev => prev.filter(c => c.id !== id))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  function updateFormField(field: string, value: string) {
    setFormData(prev => ({ ...prev, [field]: value }))
  }

  const fields = credentialTypes[formType] || []

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Vault"
        description="Credential management"
        actions={
          <button
            onClick={openCreate}
            className="flex items-center gap-1.5 text-sm rounded-lg px-3 py-1.5 transition-colors"
            style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}
          >
            <Plus size={16} /> Add
          </button>
        }
      />

      {error && <div className="px-6 py-2 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>}

      <div className="flex-1 overflow-y-auto p-6 space-y-4">
        {showForm && (
          <div className="rounded-xl p-5 space-y-4" style={{ backgroundColor: 'var(--color-surface-secondary)', border: '1px solid var(--color-border-primary)' }}>
            <h3 className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>
              {editingId ? 'Edit Credential' : 'New Credential'}
            </h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <input
                type="text"
                placeholder="Name"
                value={formName}
                onChange={e => setFormName(e.target.value)}
                className="rounded-lg px-3 py-2 text-sm outline-none"
                style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
              />
              <select
                value={formType}
                onChange={e => { setFormType(e.target.value); setFormData({}) }}
                className="rounded-lg px-3 py-2 text-sm outline-none"
                style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
              >
                {Object.keys(credentialTypes).map(t => (
                  <option key={t} value={t}>{t}</option>
                ))}
              </select>
            </div>
            {fields.length > 0 && (
              <div className="space-y-2">
                {fields.map(field => (
                  <div key={field}>
                    <label className="block text-xs mb-1" style={{ color: 'var(--color-text-tertiary)' }}>{field}</label>
                    <input
                      type={field.includes('pass') || field.includes('key') || field.includes('token') || field.includes('secret') ? 'password' : 'text'}
                      placeholder={field}
                      value={formData[field] || ''}
                      onChange={e => updateFormField(field, e.target.value)}
                      className="w-full rounded-lg px-3 py-2 text-sm outline-none"
                      style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
                    />
                  </div>
                ))}
              </div>
            )}
            {formType === 'generic' && (
              <div>
                <label className="block text-xs mb-1" style={{ color: 'var(--color-text-tertiary)' }}>Key</label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    placeholder="key"
                    id="generic-key"
                    className="flex-1 rounded-lg px-3 py-2 text-sm outline-none"
                    style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
                  />
                  <input
                    type="text"
                    placeholder="value"
                    id="generic-value"
                    className="flex-1 rounded-lg px-3 py-2 text-sm outline-none"
                    style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
                  />
                  <button
                    onClick={() => {
                      const k = (document.getElementById('generic-key') as HTMLInputElement)?.value
                      const v = (document.getElementById('generic-value') as HTMLInputElement)?.value
                      if (k) { updateFormField(k, v); (document.getElementById('generic-key') as HTMLInputElement).value = ''; (document.getElementById('generic-value') as HTMLInputElement).value = '' }
                    }}
                    className="text-sm rounded-lg px-3 py-2 transition-colors"
                    style={{ backgroundColor: 'var(--color-surface-active)', color: 'var(--color-text-primary)' }}
                  >
                    Add
                  </button>
                </div>
                {Object.keys(formData).length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1">
                    {Object.entries(formData).map(([k, v]) => (
                      <span key={k} className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-surface-active)', color: 'var(--color-text-secondary)' }}>
                        {k}: {v ? '***' : '(empty)'}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            )}
            <div className="flex gap-2">
              <button onClick={saveCred} disabled={saving} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
                {saving ? 'Saving...' : 'Save'}
              </button>
              <button onClick={() => setShowForm(false)} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>Cancel</button>
            </div>
          </div>
        )}

        {loading ? (
          <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
        ) : creds.length === 0 && !showForm ? (
          <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>No credentials stored</div>
        ) : (
          creds.map(cred => (
            <div
              key={cred.id}
              className="rounded-xl p-5 transition-colors"
              style={{ backgroundColor: 'var(--color-surface-secondary)', border: '1px solid var(--color-border-primary)' }}
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg" style={{ backgroundColor: 'var(--color-surface-tertiary)' }}>
                    <Shield size={16} style={{ color: 'var(--color-accent)' }} />
                  </div>
                  <div>
                    <h3 className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>{cred.name}</h3>
                    <span
                      className="inline-block text-xs px-2 py-0.5 rounded-full mt-1"
                      style={{
                        backgroundColor: typeColors[cred.type]?.bg || 'var(--color-surface-active)',
                        color: typeColors[cred.type]?.text || 'var(--color-text-secondary)',
                      }}
                    >
                      {cred.type}
                    </span>
                  </div>
                </div>
                <div className="flex gap-1">
                  <button
                    onClick={() => openEdit(cred)}
                    className="p-1.5 rounded-lg transition-colors"
                    style={{ color: 'var(--color-text-tertiary)' }}
                    onMouseEnter={e => e.currentTarget.style.color = 'var(--color-text-primary)'}
                    onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
                  >
                    <Edit2 size={14} />
                  </button>
                  <button
                    onClick={() => deleteCred(cred.id)}
                    className="p-1.5 rounded-lg transition-colors"
                    style={{ color: 'var(--color-text-tertiary)' }}
                    onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
                    onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
                  >
                    <Trash size={14} />
                  </button>
                </div>
              </div>
              {cred.data && Object.keys(cred.data).length > 0 && (
                <div className="mt-3 flex flex-wrap gap-2">
                  {Object.keys(cred.data).map(field => (
                    <div key={field} className="flex items-center gap-1.5 text-xs rounded px-2 py-1" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-tertiary)' }}>
                      <EyeOff size={10} />
                      {field}
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  )
}

export default Vault
