import { useState, useEffect } from 'react'
import PageHeader from '../components/PageHeader'
import { Grid, Plus, Trash, Edit2, ChevronRight } from '../components/Icons'
import { api } from '../lib/api'

type DataColumn = { id: string; name: string; type: string; index: number }
type DataTableRow = { id: number; data: Record<string, any>; createdAt: string; updatedAt: string }
type DataTable = { id: string; name: string; columns: DataColumn[]; createdAt: string; updatedAt: string }

const colTypes = ['string', 'number', 'boolean', 'date']

function DataTables() {
  const [tables, setTables] = useState<DataTable[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Table form
  const [showTableForm, setShowTableForm] = useState(false)
  const [editingTableId, setEditingTableId] = useState<string | null>(null)
  const [formName, setFormName] = useState('')
  const [formColumns, setFormColumns] = useState<DataColumn[]>([])
  const [saving, setSaving] = useState(false)

  // Detail view
  const [activeTable, setActiveTable] = useState<DataTable | null>(null)
  const [rows, setRows] = useState<DataTableRow[]>([])
  const [loadingRows, setLoadingRows] = useState(false)

  // Row editing
  const [editingRowId, setEditingRowId] = useState<number | null>(null)
  const [rowFormData, setRowFormData] = useState<Record<string, any>>({})

  useEffect(() => { loadTables() }, [])

  async function loadTables() {
    setLoading(true)
    try {
      const data = await api.get<DataTable[]>('/api/v1/data-tables')
      setTables(data || [])
    } catch { setTables([]) } finally { setLoading(false) }
  }

  async function loadRows(tableId: string) {
    setLoadingRows(true)
    try {
      const data = await api.get<DataTableRow[]>(`/api/v1/data-tables/${tableId}/rows`)
      setRows(data || [])
    } catch { setRows([]) } finally { setLoadingRows(false) }
  }

  function openTable(t: DataTable) {
    setActiveTable(t)
    setEditingRowId(null)
    loadRows(t.id)
  }

  function backToList() {
    setActiveTable(null)
    setRows([])
    setEditingRowId(null)
  }

  // --- Table CRUD ---

  function openCreateTable() {
    setEditingTableId(null)
    setFormName('')
    setFormColumns([{ id: `col_${Date.now()}`, name: '', type: 'string', index: 0 }])
    setShowTableForm(true)
    setError('')
  }

  function openEditTable(t: DataTable) {
    setEditingTableId(t.id)
    setFormName(t.name)
    setFormColumns([...t.columns])
    setShowTableForm(true)
    setError('')
  }

  function addColumn() {
    setFormColumns(prev => [...prev, { id: `col_${Date.now()}`, name: '', type: 'string', index: prev.length }])
  }

  function removeColumn(idx: number) {
    setFormColumns(prev => prev.filter((_, i) => i !== idx))
  }

  function updateColumn(idx: number, field: 'name' | 'type', value: string) {
    setFormColumns(prev => prev.map((c, i) => i === idx ? { ...c, [field]: value } : c))
  }

  async function saveTable() {
    if (!formName.trim()) { setError('Name is required'); return }
    const validCols = formColumns.filter(c => c.name.trim())
    if (validCols.length === 0) { setError('At least one column is required'); return }
    setSaving(true)
    setError('')
    try {
      const body = { name: formName.trim(), columns: validCols.map((c, i) => ({ ...c, name: c.name.trim(), index: i })) }
      if (editingTableId) {
        await api.patch(`/api/v1/data-tables/${editingTableId}`, body)
      } else {
        await api.post('/api/v1/data-tables', body)
      }
      setShowTableForm(false)
      await loadTables()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally { setSaving(false) }
  }

  async function deleteTable(id: string) {
    try {
      await api.del(`/api/v1/data-tables/${id}`)
      setTables(prev => prev.filter(t => t.id !== id))
      if (activeTable?.id === id) backToList()
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed to delete') }
  }

  // --- Row CRUD ---

  function openAddRow() {
    if (!activeTable) return
    setEditingRowId(-1) // -1 = new row
    const data: Record<string, any> = {}
    activeTable.columns.forEach(c => { data[c.name] = '' })
    setRowFormData(data)
  }

  function openEditRow(row: DataTableRow) {
    setEditingRowId(row.id)
    setRowFormData({ ...row.data })
  }

  async function saveRow() {
    if (!activeTable) return
    setSaving(true)
    try {
      if (editingRowId === -1) {
        await api.post(`/api/v1/data-tables/${activeTable.id}/rows`, [{ data: rowFormData }])
      } else {
        await api.patch(`/api/v1/data-tables/${activeTable.id}/rows/${editingRowId}`, rowFormData)
      }
      setEditingRowId(null)
      await loadRows(activeTable.id)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save row')
    } finally { setSaving(false) }
  }

  async function deleteRow(rowId: number) {
    if (!activeTable) return
    try {
      await api.del(`/api/v1/data-tables/${activeTable.id}/rows/${rowId}`)
      setRows(prev => prev.filter(r => r.id !== rowId))
    } catch (e) { setError(e instanceof Error ? e.message : 'Failed to delete row') }
  }

  const inputStyle: React.CSSProperties = {
    backgroundColor: 'var(--color-surface-tertiary)',
    border: '1px solid var(--color-border-primary)',
    color: 'var(--color-text-primary)',
  }

  // --- Detail View ---
  if (activeTable) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title={activeTable.name}
          description={`${activeTable.columns.length} columns, ${rows.length} rows`}
          actions={
            <div className="flex gap-2">
              <button onClick={backToList} className="text-sm rounded-lg px-3 py-1.5 transition-colors" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>
                Back
              </button>
              <button onClick={openAddRow} className="flex items-center gap-1.5 text-sm rounded-lg px-3 py-1.5 transition-colors" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
                <Plus size={16} /> Add Row
              </button>
            </div>
          }
        />
        {error && <div className="px-6 py-2 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>}

        <div className="flex-1 overflow-auto p-6">
          {loadingRows ? (
            <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
          ) : (
            <div className="rounded-xl overflow-hidden" style={{ border: '1px solid var(--color-border-primary)' }}>
              <table className="w-full text-sm">
                <thead>
                  <tr style={{ backgroundColor: 'var(--color-surface-secondary)' }}>
                    <th className="text-left px-4 py-2.5 text-xs font-medium" style={{ color: 'var(--color-text-tertiary)', borderBottom: '1px solid var(--color-border-primary)' }}>#</th>
                    {activeTable.columns.map(col => (
                      <th key={col.id} className="text-left px-4 py-2.5 text-xs font-medium" style={{ color: 'var(--color-text-tertiary)', borderBottom: '1px solid var(--color-border-primary)' }}>
                        {col.name} <span className="opacity-50">({col.type})</span>
                      </th>
                    ))}
                    <th className="w-20 px-4 py-2.5" style={{ borderBottom: '1px solid var(--color-border-primary)' }} />
                  </tr>
                </thead>
                <tbody>
                  {/* New row form */}
                  {editingRowId === -1 && (
                    <tr style={{ backgroundColor: 'color-mix(in srgb, var(--color-accent) 5%, transparent)' }}>
                      <td className="px-4 py-2" style={{ color: 'var(--color-text-tertiary)', borderBottom: '1px solid var(--color-border-primary)' }}>new</td>
                      {activeTable.columns.map(col => (
                        <td key={col.id} className="px-4 py-2" style={{ borderBottom: '1px solid var(--color-border-primary)' }}>
                          <input
                            type={col.type === 'number' ? 'number' : 'text'}
                            value={rowFormData[col.name] ?? ''}
                            onChange={e => setRowFormData(prev => ({ ...prev, [col.name]: col.type === 'number' ? Number(e.target.value) : e.target.value }))}
                            className="w-full rounded px-2 py-1 text-sm outline-none"
                            style={inputStyle}
                          />
                        </td>
                      ))}
                      <td className="px-4 py-2" style={{ borderBottom: '1px solid var(--color-border-primary)' }}>
                        <div className="flex gap-1">
                          <button onClick={saveRow} disabled={saving} className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
                            {saving ? '...' : 'Save'}
                          </button>
                          <button onClick={() => setEditingRowId(null)} className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>
                            Cancel
                          </button>
                        </div>
                      </td>
                    </tr>
                  )}
                  {rows.map(row => (
                    <tr key={row.id}>
                      <td className="px-4 py-2.5" style={{ color: 'var(--color-text-tertiary)', borderBottom: '1px solid var(--color-border-primary)' }}>{row.id}</td>
                      {activeTable.columns.map(col => (
                        <td key={col.id} className="px-4 py-2.5" style={{ color: 'var(--color-text-primary)', borderBottom: '1px solid var(--color-border-primary)' }}>
                          {editingRowId === row.id ? (
                            <input
                              type={col.type === 'number' ? 'number' : 'text'}
                              value={rowFormData[col.name] ?? ''}
                              onChange={e => setRowFormData(prev => ({ ...prev, [col.name]: col.type === 'number' ? Number(e.target.value) : e.target.value }))}
                              className="w-full rounded px-2 py-1 text-sm outline-none"
                              style={inputStyle}
                            />
                          ) : (
                            String(row.data[col.name] ?? '')
                          )}
                        </td>
                      ))}
                      <td className="px-4 py-2.5" style={{ borderBottom: '1px solid var(--color-border-primary)' }}>
                        {editingRowId === row.id ? (
                          <div className="flex gap-1">
                            <button onClick={saveRow} disabled={saving} className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
                              {saving ? '...' : 'Save'}
                            </button>
                            <button onClick={() => setEditingRowId(null)} className="text-xs px-2 py-1 rounded" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>
                              Cancel
                            </button>
                          </div>
                        ) : (
                          <div className="flex gap-1">
                            <button onClick={() => openEditRow(row)} className="p-1 rounded-lg transition-colors" style={{ color: 'var(--color-text-tertiary)' }}
                              onMouseEnter={e => e.currentTarget.style.color = 'var(--color-text-primary)'}
                              onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}>
                              <Edit2 size={13} />
                            </button>
                            <button onClick={() => deleteRow(row.id)} className="p-1 rounded-lg transition-colors" style={{ color: 'var(--color-text-tertiary)' }}
                              onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
                              onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}>
                              <Trash size={13} />
                            </button>
                          </div>
                        )}
                      </td>
                    </tr>
                  ))}
                  {rows.length === 0 && editingRowId !== -1 && (
                    <tr>
                      <td colSpan={activeTable.columns.length + 2} className="text-center py-8 text-sm" style={{ color: 'var(--color-text-tertiary)' }}>
                        No rows yet
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    )
  }

  // --- List View ---
  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Data Tables"
        description="Structured data storage"
        actions={
          <button onClick={openCreateTable} className="flex items-center gap-1.5 text-sm rounded-lg px-3 py-1.5 transition-colors" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
            <Plus size={16} /> New Table
          </button>
        }
      />
      {error && <div className="px-6 py-2 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>}

      <div className="flex-1 overflow-y-auto p-6 space-y-4">
        {/* Table form */}
        {showTableForm && (
          <div className="rounded-xl p-5 space-y-4" style={{ backgroundColor: 'var(--color-surface-secondary)', border: '1px solid var(--color-border-primary)' }}>
            <h3 className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>
              {editingTableId ? 'Edit Table' : 'New Table'}
            </h3>
            <input
              type="text" placeholder="Table name" value={formName} onChange={e => setFormName(e.target.value)}
              className="w-full rounded-lg px-3 py-2 text-sm outline-none" style={inputStyle}
            />
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-medium" style={{ color: 'var(--color-text-secondary)' }}>Columns</span>
                <button onClick={addColumn} className="text-xs px-2 py-1 rounded transition-colors" style={{ color: 'var(--color-accent)' }}>
                  + Add Column
                </button>
              </div>
              {formColumns.map((col, i) => (
                <div key={i} className="flex gap-2">
                  <input
                    type="text" placeholder="Column name" value={col.name} onChange={e => updateColumn(i, 'name', e.target.value)}
                    className="flex-1 rounded-lg px-3 py-2 text-sm outline-none" style={inputStyle}
                  />
                  <select value={col.type} onChange={e => updateColumn(i, 'type', e.target.value)}
                    className="rounded-lg px-3 py-2 text-sm outline-none" style={inputStyle}>
                    {colTypes.map(t => <option key={t} value={t}>{t}</option>)}
                  </select>
                  {formColumns.length > 1 && (
                    <button onClick={() => removeColumn(i)} className="p-2 rounded-lg transition-colors" style={{ color: 'var(--color-text-tertiary)' }}
                      onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
                      onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}>
                      <Trash size={14} />
                    </button>
                  )}
                </div>
              ))}
            </div>
            <div className="flex gap-2">
              <button onClick={saveTable} disabled={saving} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
                {saving ? 'Saving...' : 'Save'}
              </button>
              <button onClick={() => setShowTableForm(false)} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>
                Cancel
              </button>
            </div>
          </div>
        )}

        {/* Table list */}
        {loading ? (
          <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
        ) : tables.length === 0 && !showTableForm ? (
          <div className="text-sm text-center py-8" style={{ color: 'var(--color-text-tertiary)' }}>No data tables yet</div>
        ) : (
          tables.map(t => (
            <div key={t.id} className="rounded-xl p-5 transition-colors cursor-pointer"
              style={{ backgroundColor: 'var(--color-surface-secondary)', border: '1px solid var(--color-border-primary)' }}
              onClick={() => openTable(t)}>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg" style={{ backgroundColor: 'var(--color-surface-tertiary)' }}>
                    <Grid size={16} style={{ color: 'var(--color-accent)' }} />
                  </div>
                  <div>
                    <h3 className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>{t.name}</h3>
                    <div className="flex items-center gap-2 mt-1">
                      <span className="text-xs px-2 py-0.5 rounded-full" style={{
                        backgroundColor: 'color-mix(in srgb, var(--color-accent) 15%, transparent)',
                        color: 'var(--color-accent)',
                      }}>
                        {t.columns.length} columns
                      </span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <button onClick={e => { e.stopPropagation(); openEditTable(t) }}
                    className="p-1.5 rounded-lg transition-colors" style={{ color: 'var(--color-text-tertiary)' }}
                    onMouseEnter={e => e.currentTarget.style.color = 'var(--color-text-primary)'}
                    onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}>
                    <Edit2 size={14} />
                  </button>
                  <button onClick={e => { e.stopPropagation(); deleteTable(t.id) }}
                    className="p-1.5 rounded-lg transition-colors" style={{ color: 'var(--color-text-tertiary)' }}
                    onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
                    onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}>
                    <Trash size={14} />
                  </button>
                  <ChevronRight size={16} style={{ color: 'var(--color-text-tertiary)' }} />
                </div>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

export default DataTables
