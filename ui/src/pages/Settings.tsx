import { useState, useEffect, useRef, useCallback } from 'react'
import PageHeader from '../components/PageHeader'
import { useTheme } from '../hooks/useTheme'
import { api } from '../lib/api'

// --- Types ---

type Provider = {
  id: string
  name: string
  type: string
  base_url: string
  api_key: string
  models: string[]
}

type ActiveModel = {
  provider_id: string
  model: string
}

type SettingsData = {
  providers: Provider[]
  active_model: ActiveModel
  tavily_api_key?: string
}

type TestResult = {
  status: 'ok' | 'error'
  error?: string
}

// --- Preset providers ---

const PRESETS: { label: string; id: string; name: string; base_url: string }[] = [
  { label: 'OpenAI', id: 'openai', name: 'OpenAI', base_url: 'https://api.openai.com/v1' },
  { label: 'OpenRouter', id: 'openrouter', name: 'OpenRouter', base_url: 'https://openrouter.ai/api/v1' },
  { label: 'Anthropic', id: 'anthropic', name: 'Anthropic', base_url: 'https://api.anthropic.com/v1' },
  { label: 'Custom', id: '', name: '', base_url: '' },
]

// --- Helpers ---

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

function maskKey(key: string): string {
  if (!key || key.length < 8) return key
  return '***...' + key.slice(-4)
}

// --- Shared style constants ---

const inputStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-tertiary)',
  border: '1px solid var(--color-border-primary)',
  color: 'var(--color-text-primary)',
}

const sectionStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-secondary)',
  border: '1px solid var(--color-border-primary)',
}

// --- Component ---

function Settings() {
  const { theme, toggle } = useTheme()

  // Data state
  const [providers, setProviders] = useState<Provider[]>([])
  const [activeModel, setActiveModel] = useState<ActiveModel>({ provider_id: '', model: '' })
  const [selectedProviderId, setSelectedProviderId] = useState<string | null>(null)

  // Active model form
  const [activeProviderInput, setActiveProviderInput] = useState('')
  const [activeModelInput, setActiveModelInput] = useState('')
  const [savingActiveModel, setSavingActiveModel] = useState(false)
  const [activeModelSaved, setActiveModelSaved] = useState(false)

  // Provider form
  const [formName, setFormName] = useState('')
  const [formId, setFormId] = useState('')
  const [formType, setFormType] = useState('openai')
  const [formBaseUrl, setFormBaseUrl] = useState('')
  const [formApiKey, setFormApiKey] = useState('')
  const [formModels, setFormModels] = useState<string[]>([])
  const [modelInput, setModelInput] = useState('')
  const [showApiKey, setShowApiKey] = useState(false)
  const [isCreating, setIsCreating] = useState(false)

  // Provider actions
  const [savingProvider, setSavingProvider] = useState(false)
  const [providerSaved, setProviderSaved] = useState(false)
  const [deletingProvider, setDeletingProvider] = useState(false)
  const [testingConnection, setTestingConnection] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  // Add provider popover
  const [showAddPopover, setShowAddPopover] = useState(false)
  const addBtnRef = useRef<HTMLButtonElement>(null)
  const popoverRef = useRef<HTMLDivElement>(null)

  // Tavily integration
  const [tavilyKey, setTavilyKey] = useState('')
  const [showTavilyKey, setShowTavilyKey] = useState(false)
  const [savingTavily, setSavingTavily] = useState(false)
  const [tavilySaved, setTavilySaved] = useState(false)

  // Global error
  const [error, setError] = useState('')

  // --- Data loading ---

  const loadSettings = useCallback(async () => {
    try {
      const data = await api.get<SettingsData>('/api/settings')
      setProviders(data.providers || [])
      setActiveModel(data.active_model || { provider_id: '', model: '' })
      setActiveProviderInput(data.active_model?.provider_id || '')
      setActiveModelInput(data.active_model?.model || '')
      setTavilyKey(data.tavily_api_key || '')
    } catch {
      // Silently handle — page will show empty state
    }
  }, [])

  useEffect(() => {
    loadSettings()
  }, [loadSettings])

  // Close add-provider popover on outside click
  useEffect(() => {
    function onClickOutside(e: MouseEvent) {
      if (
        showAddPopover &&
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node) &&
        addBtnRef.current &&
        !addBtnRef.current.contains(e.target as Node)
      ) {
        setShowAddPopover(false)
      }
    }
    document.addEventListener('mousedown', onClickOutside)
    return () => document.removeEventListener('mousedown', onClickOutside)
  }, [showAddPopover])

  // --- Provider selection ---

  const selectProvider = useCallback((provider: Provider) => {
    setSelectedProviderId(provider.id)
    setFormName(provider.name)
    setFormId(provider.id)
    setFormType(provider.type)
    setFormBaseUrl(provider.base_url)
    setFormApiKey(provider.api_key)
    setFormModels([...provider.models])
    setShowApiKey(false)
    setIsCreating(false)
    setTestResult(null)
    setProviderSaved(false)
    setError('')
  }, [])

  const startCreateProvider = useCallback((preset: typeof PRESETS[number]) => {
    setShowAddPopover(false)
    setSelectedProviderId(null)
    setFormName(preset.name)
    setFormId(preset.id)
    setFormType('openai')
    setFormBaseUrl(preset.base_url)
    setFormApiKey('')
    setFormModels([])
    setShowApiKey(false)
    setIsCreating(true)
    setTestResult(null)
    setProviderSaved(false)
    setError('')
  }, [])

  // Auto-generate ID from name when creating
  const handleNameChange = useCallback((name: string) => {
    setFormName(name)
    if (isCreating) {
      setFormId(slugify(name))
    }
  }, [isCreating])

  // --- Model chips ---

  const addModel = useCallback(() => {
    const m = modelInput.trim()
    if (m && !formModels.includes(m)) {
      setFormModels(prev => [...prev, m])
    }
    setModelInput('')
  }, [modelInput, formModels])

  const removeModel = useCallback((model: string) => {
    setFormModels(prev => prev.filter(m => m !== model))
  }, [])

  // --- Actions ---

  async function saveActiveModel() {
    setSavingActiveModel(true)
    setError('')
    try {
      await api.put('/api/settings/active-model', {
        provider_id: activeProviderInput,
        model: activeModelInput,
      })
      setActiveModel({ provider_id: activeProviderInput, model: activeModelInput })
      setActiveModelSaved(true)
      setTimeout(() => setActiveModelSaved(false), 2000)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save active model')
    } finally {
      setSavingActiveModel(false)
    }
  }

  async function saveProvider() {
    setSavingProvider(true)
    setError('')
    setProviderSaved(false)
    try {
      const body: Record<string, unknown> = {
        id: formId,
        name: formName,
        type: formType,
        base_url: formBaseUrl,
        models: formModels,
      }
      // Only send api_key if it was changed (not masked)
      if (!formApiKey.startsWith('***')) {
        body.api_key = formApiKey
      }

      await api.post('/api/settings/providers', body)
      await loadSettings()

      // Select the saved provider
      setSelectedProviderId(formId)
      setIsCreating(false)
      setProviderSaved(true)
      setTimeout(() => setProviderSaved(false), 2000)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save provider')
    } finally {
      setSavingProvider(false)
    }
  }

  async function deleteProvider() {
    if (!selectedProviderId) return
    setDeletingProvider(true)
    setError('')
    try {
      await api.del(`/api/settings/providers/${selectedProviderId}`)
      await loadSettings()
      setSelectedProviderId(null)
      setIsCreating(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete provider')
    } finally {
      setDeletingProvider(false)
    }
  }

  async function testConnection() {
    setTestingConnection(true)
    setTestResult(null)
    try {
      const body: Record<string, unknown> = {
        base_url: formBaseUrl,
        type: formType,
      }
      if (!formApiKey.startsWith('***')) {
        body.api_key = formApiKey
      }
      const result = await api.post<TestResult>('/api/settings/providers/test', body)
      if (result.status === 'ok') {
        setTestResult({ ok: true, message: 'Connection successful' })
      } else {
        setTestResult({ ok: false, message: result.error || 'Connection failed' })
      }
    } catch (e) {
      setTestResult({ ok: false, message: e instanceof Error ? e.message : 'Connection failed' })
    } finally {
      setTestingConnection(false)
    }
  }

  async function saveTavilyKey() {
    setSavingTavily(true)
    setError('')
    try {
      // Don't send if it's masked (unchanged)
      if (!tavilyKey.startsWith('***')) {
        await api.put('/api/settings/tavily-key', { api_key: tavilyKey })
      }
      setTavilySaved(true)
      setTimeout(() => setTavilySaved(false), 2000)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save Tavily API key')
    } finally {
      setSavingTavily(false)
    }
  }

  // --- Derived state ---

  const selectedProvider = providers.find(p => p.id === selectedProviderId) || null
  const showForm = isCreating || selectedProvider !== null

  return (
    <div className="flex flex-col h-full">
      <PageHeader title="Settings" description="Configuration & preferences" />

      <div className="flex-1 overflow-y-auto p-6">
        <div className="max-w-4xl mx-auto space-y-6">
          {error && (
            <div
              className="text-sm px-4 py-2.5 rounded-lg"
              style={{ color: 'var(--color-error)', backgroundColor: 'var(--color-surface-tertiary)' }}
            >
              {error}
            </div>
          )}

          {/* ── Model Configuration ── */}
          <section className="rounded-xl p-5" style={sectionStyle}>
            <h2 className="text-sm font-semibold mb-4" style={{ color: 'var(--color-text-primary)' }}>
              Model Configuration
            </h2>
            {activeModel.provider_id && (
              <div className="text-xs mb-4 flex items-center gap-2" style={{ color: 'var(--color-text-tertiary)' }}>
                <span>Current:</span>
                <span
                  className="px-2 py-0.5 rounded-md text-xs"
                  style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-secondary)' }}
                >
                  {activeModel.provider_id} / {activeModel.model}
                </span>
              </div>
            )}
            <div className="flex flex-col sm:flex-row gap-3">
              <div className="flex-1">
                <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                  Provider
                </label>
                <select
                  value={activeProviderInput}
                  onChange={e => setActiveProviderInput(e.target.value)}
                  className="w-full rounded-lg px-3 py-2 text-sm outline-none"
                  style={inputStyle}
                >
                  <option value="">Select provider...</option>
                  {providers.map(p => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
              </div>
              <div className="flex-1">
                <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                  Model
                </label>
                <input
                  type="text"
                  value={activeModelInput}
                  onChange={e => setActiveModelInput(e.target.value)}
                  placeholder="e.g. gpt-4o, claude-sonnet-4-20250514"
                  className="w-full rounded-lg px-3 py-2 text-sm outline-none"
                  style={inputStyle}
                />
              </div>
              <div className="flex items-end gap-2">
                {activeModelSaved && (
                  <span className="text-xs pb-2" style={{ color: 'var(--color-success)' }}>Saved</span>
                )}
                <button
                  onClick={saveActiveModel}
                  disabled={savingActiveModel || !activeProviderInput || !activeModelInput}
                  className="rounded-lg px-5 py-2 text-sm font-medium transition-colors"
                  style={{
                    backgroundColor: 'var(--color-accent)',
                    color: 'var(--color-accent-text)',
                    opacity: savingActiveModel || !activeProviderInput || !activeModelInput ? 0.4 : 1,
                  }}
                >
                  {savingActiveModel ? 'Saving...' : 'Save'}
                </button>
              </div>
            </div>
          </section>

          {/* ── AI Providers ── */}
          <section className="rounded-xl overflow-hidden" style={sectionStyle}>
            <div
              className="flex items-center justify-between px-5 py-3"
              style={{ borderBottom: '1px solid var(--color-border-primary)' }}
            >
              <h2 className="text-sm font-semibold" style={{ color: 'var(--color-text-primary)' }}>
                AI Providers
              </h2>
            </div>

            <div className="flex" style={{ minHeight: '420px' }}>
              {/* Left column - provider list */}
              <div
                className="flex flex-col shrink-0"
                style={{
                  width: '240px',
                  borderRight: '1px solid var(--color-border-primary)',
                }}
              >
                {/* Add Provider button */}
                <div className="p-3 relative">
                  <button
                    ref={addBtnRef}
                    onClick={() => setShowAddPopover(!showAddPopover)}
                    className="w-full text-xs font-medium rounded-lg px-3 py-2 transition-colors text-left"
                    style={{
                      backgroundColor: 'var(--color-surface-tertiary)',
                      color: 'var(--color-text-secondary)',
                      border: '1px solid var(--color-border-primary)',
                    }}
                    onMouseEnter={e => {
                      e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'
                    }}
                    onMouseLeave={e => {
                      e.currentTarget.style.backgroundColor = 'var(--color-surface-tertiary)'
                    }}
                  >
                    + Add Provider
                  </button>

                  {/* Preset popover */}
                  {showAddPopover && (
                    <div
                      ref={popoverRef}
                      className="absolute left-3 right-3 top-full mt-1 rounded-lg py-1 z-10"
                      style={{
                        backgroundColor: 'var(--color-surface-elevated)',
                        border: '1px solid var(--color-border-primary)',
                        boxShadow: '0 4px 16px rgba(0,0,0,0.15)',
                      }}
                    >
                      {PRESETS.map(preset => (
                        <button
                          key={preset.label}
                          onClick={() => startCreateProvider(preset)}
                          className="w-full text-left px-3 py-2 text-sm transition-colors"
                          style={{ color: 'var(--color-text-primary)' }}
                          onMouseEnter={e => {
                            e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'
                          }}
                          onMouseLeave={e => {
                            e.currentTarget.style.backgroundColor = 'transparent'
                          }}
                        >
                          <div className="text-sm">{preset.label}</div>
                          {preset.base_url && (
                            <div className="text-xs mt-0.5" style={{ color: 'var(--color-text-tertiary)' }}>
                              {preset.base_url}
                            </div>
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>

                {/* Provider list */}
                <div className="flex-1 overflow-y-auto">
                  {providers.length === 0 && (
                    <div className="px-3 py-6 text-center text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                      No providers configured
                    </div>
                  )}
                  {providers.map(p => {
                    const isSelected = selectedProviderId === p.id && !isCreating
                    const isActive = activeModel.provider_id === p.id
                    return (
                      <div
                        key={p.id}
                        onClick={() => selectProvider(p)}
                        className="flex items-center gap-2.5 px-4 py-2.5 cursor-pointer transition-colors"
                        style={{
                          backgroundColor: isSelected ? 'var(--color-surface-active)' : 'transparent',
                          color: isSelected ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                        }}
                        onMouseEnter={e => {
                          if (!isSelected) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'
                        }}
                        onMouseLeave={e => {
                          if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent'
                        }}
                      >
                        {/* Active indicator dot */}
                        <span
                          className="shrink-0 w-2 h-2 rounded-full"
                          style={{
                            backgroundColor: isActive ? 'var(--color-success)' : 'transparent',
                          }}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="text-sm font-medium truncate">{p.name}</div>
                          <div
                            className="text-xs truncate mt-0.5"
                            style={{ color: 'var(--color-text-tertiary)' }}
                          >
                            {p.base_url}
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* Right column - provider detail form */}
              <div className="flex-1 overflow-y-auto">
                {!showForm ? (
                  <div className="flex items-center justify-center h-full">
                    <span className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                      Select a provider to configure
                    </span>
                  </div>
                ) : (
                  <div className="p-5 space-y-4">
                    {/* Name */}
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                        Name
                      </label>
                      <input
                        type="text"
                        value={formName}
                        onChange={e => handleNameChange(e.target.value)}
                        placeholder="My Provider"
                        className="w-full rounded-lg px-3 py-2 text-sm outline-none"
                        style={inputStyle}
                      />
                    </div>

                    {/* ID */}
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                        ID
                      </label>
                      <input
                        type="text"
                        value={formId}
                        onChange={e => isCreating && setFormId(e.target.value)}
                        readOnly={!isCreating}
                        placeholder="provider-id"
                        className="w-full rounded-lg px-3 py-2 text-sm outline-none"
                        style={{
                          ...inputStyle,
                          opacity: !isCreating ? 0.6 : 1,
                          cursor: !isCreating ? 'default' : undefined,
                        }}
                      />
                    </div>

                    {/* Type */}
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                        Type
                      </label>
                      <select
                        value={formType}
                        onChange={e => setFormType(e.target.value)}
                        className="w-full rounded-lg px-3 py-2 text-sm outline-none"
                        style={inputStyle}
                      >
                        <option value="openai">OpenAI-compatible</option>
                      </select>
                    </div>

                    {/* Base URL */}
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                        API Base URL
                      </label>
                      <input
                        type="text"
                        value={formBaseUrl}
                        onChange={e => setFormBaseUrl(e.target.value)}
                        placeholder="https://api.openai.com/v1"
                        className="w-full rounded-lg px-3 py-2 text-sm outline-none"
                        style={inputStyle}
                      />
                    </div>

                    {/* API Key */}
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                        API Key
                      </label>
                      <div className="relative">
                        <input
                          type={showApiKey ? 'text' : 'password'}
                          value={formApiKey}
                          onChange={e => setFormApiKey(e.target.value)}
                          placeholder="sk-..."
                          className="w-full rounded-lg px-3 py-2 pr-16 text-sm outline-none"
                          style={inputStyle}
                        />
                        <button
                          type="button"
                          onClick={() => setShowApiKey(!showApiKey)}
                          className="absolute right-2 top-1/2 -translate-y-1/2 text-xs px-2 py-1 rounded transition-colors"
                          style={{ color: 'var(--color-text-tertiary)' }}
                          onMouseEnter={e => {
                            e.currentTarget.style.color = 'var(--color-text-primary)'
                          }}
                          onMouseLeave={e => {
                            e.currentTarget.style.color = 'var(--color-text-tertiary)'
                          }}
                        >
                          {showApiKey ? 'Hide' : 'Show'}
                        </button>
                      </div>
                      {!isCreating && selectedProvider?.api_key && (
                        <div className="text-xs mt-1" style={{ color: 'var(--color-text-quaternary)' }}>
                          Stored: {maskKey(selectedProvider.api_key)}
                        </div>
                      )}
                    </div>

                    {/* Models */}
                    <div>
                      <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                        Models
                      </label>
                      <div className="flex flex-wrap gap-1.5 mb-2 min-h-[28px]">
                        {formModels.map(m => (
                          <span
                            key={m}
                            className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs"
                            style={{
                              backgroundColor: 'var(--color-surface-tertiary)',
                              color: 'var(--color-text-secondary)',
                              border: '1px solid var(--color-border-primary)',
                            }}
                          >
                            {m}
                            <button
                              onClick={() => removeModel(m)}
                              className="ml-0.5 transition-colors"
                              style={{ color: 'var(--color-text-tertiary)' }}
                              onMouseEnter={e => {
                                e.currentTarget.style.color = 'var(--color-error)'
                              }}
                              onMouseLeave={e => {
                                e.currentTarget.style.color = 'var(--color-text-tertiary)'
                              }}
                            >
                              &times;
                            </button>
                          </span>
                        ))}
                      </div>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value={modelInput}
                          onChange={e => setModelInput(e.target.value)}
                          onKeyDown={e => {
                            if (e.key === 'Enter') {
                              e.preventDefault()
                              addModel()
                            }
                          }}
                          placeholder="Add model name..."
                          className="flex-1 rounded-lg px-3 py-2 text-sm outline-none"
                          style={inputStyle}
                        />
                        <button
                          onClick={addModel}
                          disabled={!modelInput.trim()}
                          className="rounded-lg px-3 py-2 text-sm transition-colors"
                          style={{
                            backgroundColor: 'var(--color-surface-tertiary)',
                            color: 'var(--color-text-secondary)',
                            border: '1px solid var(--color-border-primary)',
                            opacity: !modelInput.trim() ? 0.4 : 1,
                          }}
                        >
                          Add
                        </button>
                      </div>
                    </div>

                    {/* Test connection */}
                    <div
                      className="pt-4"
                      style={{ borderTop: '1px solid var(--color-border-secondary)' }}
                    >
                      <div className="flex items-center gap-3">
                        <button
                          onClick={testConnection}
                          disabled={testingConnection || !formBaseUrl}
                          className="rounded-lg px-4 py-2 text-sm transition-colors"
                          style={{
                            backgroundColor: 'var(--color-surface-tertiary)',
                            color: 'var(--color-text-secondary)',
                            border: '1px solid var(--color-border-primary)',
                            opacity: testingConnection || !formBaseUrl ? 0.4 : 1,
                          }}
                        >
                          {testingConnection ? 'Testing...' : 'Test Connection'}
                        </button>
                        {testResult && (
                          <span
                            className="text-xs"
                            style={{ color: testResult.ok ? 'var(--color-success)' : 'var(--color-error)' }}
                          >
                            {testResult.message}
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Save / Delete buttons */}
                    <div
                      className="flex items-center justify-between pt-4"
                      style={{ borderTop: '1px solid var(--color-border-secondary)' }}
                    >
                      <div>
                        {!isCreating && (
                          <button
                            onClick={deleteProvider}
                            disabled={deletingProvider}
                            className="rounded-lg px-4 py-2 text-sm transition-colors"
                            style={{
                              color: 'var(--color-error)',
                              backgroundColor: 'transparent',
                              border: '1px solid var(--color-border-primary)',
                              opacity: deletingProvider ? 0.4 : 1,
                            }}
                            onMouseEnter={e => {
                              e.currentTarget.style.borderColor = 'var(--color-error)'
                            }}
                            onMouseLeave={e => {
                              e.currentTarget.style.borderColor = 'var(--color-border-primary)'
                            }}
                          >
                            {deletingProvider ? 'Deleting...' : 'Delete Provider'}
                          </button>
                        )}
                      </div>
                      <div className="flex items-center gap-3">
                        {providerSaved && (
                          <span className="text-xs" style={{ color: 'var(--color-success)' }}>
                            Saved
                          </span>
                        )}
                        <button
                          onClick={saveProvider}
                          disabled={savingProvider || !formId || !formName || !formBaseUrl}
                          className="rounded-lg px-5 py-2 text-sm font-medium transition-colors"
                          style={{
                            backgroundColor: 'var(--color-accent)',
                            color: 'var(--color-accent-text)',
                            opacity: savingProvider || !formId || !formName || !formBaseUrl ? 0.4 : 1,
                          }}
                        >
                          {savingProvider
                            ? 'Saving...'
                            : isCreating
                              ? 'Save Provider'
                              : 'Update Provider'}
                        </button>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </section>

          {/* ── Integrations ── */}
          <section className="rounded-xl p-5" style={sectionStyle}>
            <h2 className="text-sm font-semibold mb-4" style={{ color: 'var(--color-text-primary)' }}>
              Integrations
            </h2>
            <div className="space-y-3">
              <div>
                <label className="block text-xs mb-1.5" style={{ color: 'var(--color-text-tertiary)' }}>
                  Tavily API Key
                </label>
                <div className="text-xs mb-2" style={{ color: 'var(--color-text-quaternary)' }}>
                  Required for the web_search skill. Get a free key at{' '}
                  <a
                    href="https://tavily.com"
                    target="_blank"
                    rel="noopener noreferrer"
                    style={{ color: 'var(--color-accent)' }}
                  >
                    tavily.com
                  </a>
                </div>
                <div className="flex gap-3">
                  <div className="relative flex-1">
                    <input
                      type={showTavilyKey ? 'text' : 'password'}
                      value={tavilyKey}
                      onChange={e => setTavilyKey(e.target.value)}
                      placeholder="tvly-..."
                      className="w-full rounded-lg px-3 py-2 pr-16 text-sm outline-none"
                      style={inputStyle}
                    />
                    <button
                      type="button"
                      onClick={() => setShowTavilyKey(!showTavilyKey)}
                      className="absolute right-2 top-1/2 -translate-y-1/2 text-xs px-2 py-1 rounded transition-colors"
                      style={{ color: 'var(--color-text-tertiary)' }}
                      onMouseEnter={e => {
                        e.currentTarget.style.color = 'var(--color-text-primary)'
                      }}
                      onMouseLeave={e => {
                        e.currentTarget.style.color = 'var(--color-text-tertiary)'
                      }}
                    >
                      {showTavilyKey ? 'Hide' : 'Show'}
                    </button>
                  </div>
                  <div className="flex items-center gap-2">
                    {tavilySaved && (
                      <span className="text-xs" style={{ color: 'var(--color-success)' }}>Saved</span>
                    )}
                    <button
                      onClick={saveTavilyKey}
                      disabled={savingTavily || !tavilyKey}
                      className="rounded-lg px-5 py-2 text-sm font-medium transition-colors"
                      style={{
                        backgroundColor: 'var(--color-accent)',
                        color: 'var(--color-accent-text)',
                        opacity: savingTavily || !tavilyKey ? 0.4 : 1,
                      }}
                    >
                      {savingTavily ? 'Saving...' : 'Save'}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* ── Appearance ── */}
          <section className="rounded-xl p-5" style={sectionStyle}>
            <h2 className="text-sm font-semibold mb-4" style={{ color: 'var(--color-text-primary)' }}>
              Appearance
            </h2>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm" style={{ color: 'var(--color-text-primary)' }}>Dark Mode</div>
                <div className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                  Toggle between light and dark themes
                </div>
              </div>
              <button
                onClick={toggle}
                className="w-10 h-6 rounded-full relative transition-colors"
                style={{
                  backgroundColor: theme === 'dark' ? 'var(--color-accent)' : 'var(--color-surface-active)',
                }}
              >
                <div
                  className="absolute top-1 w-4 h-4 rounded-full transition-all"
                  style={{
                    left: theme === 'dark' ? '22px' : '4px',
                    backgroundColor:
                      theme === 'dark' ? 'var(--color-accent-text)' : 'var(--color-text-tertiary)',
                  }}
                />
              </button>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

export default Settings
