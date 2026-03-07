import { useState, useEffect, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import PageHeader from '../components/PageHeader'
import { Folder, File, Plus, Trash, ChevronRight } from '../components/Icons'
import { api } from '../lib/api'

type Doc = { path: string; content?: string; size: number; modified_at: string; is_dir: boolean }

// Build a tree structure from a flat list of docs
type TreeNode = {
  name: string
  path: string
  isDir: boolean
  doc?: Doc
  children: TreeNode[]
}

function buildTree(docs: Doc[]): TreeNode[] {
  const root: TreeNode = { name: '', path: '', isDir: true, children: [] }

  for (const doc of docs) {
    const parts = doc.path.split('/')
    let current = root

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      const isLast = i === parts.length - 1
      const partPath = parts.slice(0, i + 1).join('/')

      let child = current.children.find(c => c.name === part)
      if (!child) {
        child = {
          name: part,
          path: partPath,
          isDir: isLast ? doc.is_dir : true,
          doc: isLast ? doc : undefined,
          children: [],
        }
        current.children.push(child)
      } else if (isLast) {
        child.doc = doc
        child.isDir = doc.is_dir
      }
      current = child
    }
  }

  // Sort: directories first, then alphabetical
  function sortTree(nodes: TreeNode[]) {
    nodes.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
      return a.name.localeCompare(b.name)
    })
    for (const n of nodes) sortTree(n.children)
  }
  sortTree(root.children)

  return root.children
}

function TreeItem({
  node,
  depth,
  selectedPath,
  onSelect,
  onDelete,
}: {
  node: TreeNode
  depth: number
  selectedPath: string
  onSelect: (doc: Doc) => void
  onDelete: (path: string) => void
}) {
  const [expanded, setExpanded] = useState(depth < 2)
  const isActive = selectedPath === node.path

  if (node.isDir) {
    return (
      <div>
        <div
          className="flex items-center gap-1 py-1 px-2 rounded-md cursor-pointer transition-colors text-xs"
          style={{
            paddingLeft: `${depth * 16 + 8}px`,
            color: 'var(--color-text-secondary)',
          }}
          onClick={() => setExpanded(!expanded)}
          onMouseEnter={e => e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'}
          onMouseLeave={e => e.currentTarget.style.backgroundColor = 'transparent'}
        >
          <span style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.12s', display: 'inline-flex' }}>
            <ChevronRight size={12} />
          </span>
          <Folder size={14} style={{ color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
          <span className="truncate">{node.name}</span>
        </div>
        {expanded && node.children.map(child => (
          <TreeItem
            key={child.path}
            node={child}
            depth={depth + 1}
            selectedPath={selectedPath}
            onSelect={onSelect}
            onDelete={onDelete}
          />
        ))}
      </div>
    )
  }

  return (
    <div
      className="group flex items-center gap-1.5 py-1 px-2 rounded-md cursor-pointer transition-colors text-xs"
      style={{
        paddingLeft: `${depth * 16 + 24}px`,
        backgroundColor: isActive ? 'var(--color-surface-active)' : 'transparent',
        color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
      }}
      onClick={() => node.doc && onSelect(node.doc)}
      onMouseEnter={e => { if (!isActive) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)' }}
      onMouseLeave={e => { if (!isActive) e.currentTarget.style.backgroundColor = 'transparent' }}
    >
      <File size={14} style={{ color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
      <span className="flex-1 truncate">{node.name}</span>
      <button
        onClick={e => { e.stopPropagation(); onDelete(node.path) }}
        className="opacity-0 group-hover:opacity-100 p-0.5 rounded transition-opacity shrink-0"
        style={{ color: 'var(--color-text-tertiary)' }}
        onMouseEnter={e => e.currentTarget.style.color = 'var(--color-error)'}
        onMouseLeave={e => e.currentTarget.style.color = 'var(--color-text-tertiary)'}
      >
        <Trash size={11} />
      </button>
    </div>
  )
}

function Documents() {
  const [docs, setDocs] = useState<Doc[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedDoc, setSelectedDoc] = useState<Doc | null>(null)
  const [editing, setEditing] = useState(false)
  const [editContent, setEditContent] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [showNew, setShowNew] = useState(false)
  const [newPath, setNewPath] = useState('')
  const [newContent, setNewContent] = useState('')

  const loadDocs = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.get<Doc[]>('/api/documents')
      setDocs(data || [])
    } catch {
      setDocs([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadDocs() }, [loadDocs])

  const tree = buildTree(docs)

  async function selectDoc(doc: Doc) {
    if (doc.is_dir) return
    setError('')
    try {
      const full = await api.get<Doc>(`/api/documents/${doc.path}`)
      setSelectedDoc(full)
      setEditing(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load document')
    }
  }

  async function saveDoc() {
    if (!selectedDoc) return
    setSaving(true)
    setError('')
    try {
      await api.put(`/api/documents/${selectedDoc.path}`, { content: editContent })
      setSelectedDoc({ ...selectedDoc, content: editContent })
      setEditing(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function deleteDoc(path: string) {
    setError('')
    try {
      await api.del(`/api/documents/${path}`)
      setDocs(prev => prev.filter(d => d.path !== path))
      if (selectedDoc?.path === path) setSelectedDoc(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  async function createDoc() {
    if (!newPath.trim()) return
    setSaving(true)
    setError('')
    try {
      await api.put(`/api/documents/${newPath.trim()}`, { content: newContent })
      setShowNew(false)
      setNewPath('')
      setNewContent('')
      await loadDocs()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    } finally {
      setSaving(false)
    }
  }

  function startEdit() {
    if (!selectedDoc) return
    setEditContent(selectedDoc.content || '')
    setEditing(true)
  }

  const isMarkdown = selectedDoc?.path.endsWith('.md')

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Documents"
        description="Notes, templates & knowledge"
        actions={
          <button
            onClick={() => setShowNew(!showNew)}
            className="flex items-center gap-1.5 text-sm rounded-lg px-3 py-1.5 transition-colors"
            style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}
          >
            <Plus size={16} /> New
          </button>
        }
      />

      {error && <div className="px-6 py-2 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>}

      {showNew && (
        <div className="px-6 py-4" style={{ borderBottom: '1px solid var(--color-border-primary)' }}>
          <div className="space-y-3 max-w-lg">
            <input
              type="text"
              placeholder="Path (e.g. notes/meeting.md)"
              value={newPath}
              onChange={e => setNewPath(e.target.value)}
              className="w-full rounded-lg px-3 py-2 text-sm outline-none"
              style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
            />
            <textarea
              placeholder="Content (markdown)"
              value={newContent}
              onChange={e => setNewContent(e.target.value)}
              rows={4}
              className="w-full rounded-lg px-3 py-2 text-sm outline-none resize-y"
              style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-primary)', color: 'var(--color-text-primary)' }}
            />
            <div className="flex gap-2">
              <button onClick={createDoc} disabled={saving} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
                {saving ? 'Creating...' : 'Create'}
              </button>
              <button onClick={() => setShowNew(false)} className="text-sm rounded-lg px-4 py-2 transition-colors" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>Cancel</button>
            </div>
          </div>
        </div>
      )}

      <div className="flex-1 flex min-h-0">
        {/* File tree sidebar */}
        <div className="w-64 overflow-y-auto shrink-0" style={{ borderRight: '1px solid var(--color-border-primary)' }}>
          <div className="p-2">
            {loading ? (
              <div className="text-xs text-center py-4" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
            ) : tree.length === 0 ? (
              <div className="text-xs text-center py-4" style={{ color: 'var(--color-text-tertiary)' }}>No documents</div>
            ) : (
              tree.map(node => (
                <TreeItem
                  key={node.path}
                  node={node}
                  depth={0}
                  selectedPath={selectedDoc?.path || ''}
                  onSelect={selectDoc}
                  onDelete={deleteDoc}
                />
              ))
            )}
          </div>
        </div>

        {/* Content area */}
        <div className="flex-1 flex flex-col overflow-hidden">
          {selectedDoc ? (
            <>
              <div className="flex items-center justify-between px-6 py-3 shrink-0" style={{ borderBottom: '1px solid var(--color-border-secondary)' }}>
                <div>
                  <div className="text-sm font-medium" style={{ color: 'var(--color-text-primary)' }}>{selectedDoc.path}</div>
                  <div className="text-xs" style={{ color: 'var(--color-text-tertiary)' }}>
                    {selectedDoc.size} bytes
                  </div>
                </div>
                <div className="flex gap-2">
                  {editing ? (
                    <>
                      <button onClick={saveDoc} disabled={saving} className="text-sm rounded-lg px-3 py-1.5 transition-colors" style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}>
                        {saving ? 'Saving...' : 'Save'}
                      </button>
                      <button onClick={() => setEditing(false)} className="text-sm rounded-lg px-3 py-1.5 transition-colors" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>Cancel</button>
                    </>
                  ) : (
                    <button onClick={startEdit} className="text-sm rounded-lg px-3 py-1.5 transition-colors" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-primary)' }}>Edit</button>
                  )}
                </div>
              </div>
              <div className="flex-1 overflow-y-auto">
                {editing ? (
                  <textarea
                    value={editContent}
                    onChange={e => setEditContent(e.target.value)}
                    className="w-full h-full p-6 text-sm font-mono resize-none outline-none"
                    style={{ backgroundColor: 'var(--color-surface-primary)', color: 'var(--color-text-primary)' }}
                  />
                ) : isMarkdown ? (
                  <div className="p-6 prose-kuro text-sm">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{selectedDoc.content || ''}</ReactMarkdown>
                  </div>
                ) : (
                  <pre className="p-6 text-sm font-mono whitespace-pre-wrap" style={{ color: 'var(--color-text-primary)' }}>
                    {selectedDoc.content}
                  </pre>
                )}
              </div>
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center">
                <File size={32} className="mx-auto mb-3" style={{ color: 'var(--color-text-quaternary)' }} />
                <div className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Select a document to view</div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default Documents
