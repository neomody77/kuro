import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { Tree } from 'react-arborist'
import type { NodeRendererProps } from 'react-arborist'
import { useCreateBlockNote } from '@blocknote/react'
import { BlockNoteView } from '@blocknote/mantine'
import '@blocknote/core/style.css'
import '@blocknote/mantine/style.css'
import PageHeader from '../components/PageHeader'
import { Folder, Plus, ChevronRight, Search } from '../components/Icons'
import { api } from '../lib/api'

type Doc = { path: string; content?: string; size: number; modified_at: string; is_dir: boolean }

type TreeData = {
  id: string
  name: string
  isDir: boolean
  doc?: Doc
  children?: TreeData[]
}

// --- File type icons ---

type IconProps = { size?: number; style?: React.CSSProperties }

function FileIconMd({ size = 14, style }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={style}>
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
      <path d="M16 13H8" />
      <path d="M16 17H8" />
      <path d="M10 9H8" />
    </svg>
  )
}

function FileIconCode({ size = 14, style }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={style}>
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
      <path d="M10 12l-2 2 2 2" />
      <path d="M14 12l2 2-2 2" />
    </svg>
  )
}

function FileIconConfig({ size = 14, style }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={style}>
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
      <circle cx="12" cy="15" r="2" />
      <path d="M12 11v2" />
      <path d="M12 17v2" />
    </svg>
  )
}

function FileIconGeneric({ size = 14, style }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={style}>
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
    </svg>
  )
}

function FileIconJson({ size = 14, style }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={style}>
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
      <path d="M8 16s0 2 2 2" />
      <path d="M16 16s0 2-2 2" />
      <path d="M10 12s0-2 2-2" />
      <path d="M14 12s0-2-2-2" />
    </svg>
  )
}

const iconColors: Record<string, string> = {
  md: '#6366f1',
  yaml: '#f59e0b',
  yml: '#f59e0b',
  json: '#10b981',
  toml: '#f59e0b',
  js: '#eab308',
  ts: '#3b82f6',
  go: '#06b6d4',
  py: '#3b82f6',
  sh: '#8b5cf6',
  txt: '#6b7280',
}

function getFileIcon(name: string) {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  const color = iconColors[ext] || 'var(--color-text-tertiary)'
  const s = { color, flexShrink: 0 } as React.CSSProperties

  switch (ext) {
    case 'md':
    case 'mdx':
      return <FileIconMd size={14} style={s} />
    case 'json':
      return <FileIconJson size={14} style={s} />
    case 'yaml':
    case 'yml':
    case 'toml':
    case 'ini':
    case 'conf':
      return <FileIconConfig size={14} style={s} />
    case 'js':
    case 'ts':
    case 'tsx':
    case 'jsx':
    case 'go':
    case 'py':
    case 'sh':
    case 'bash':
    case 'css':
    case 'html':
      return <FileIconCode size={14} style={s} />
    default:
      return <FileIconGeneric size={14} style={s} />
  }
}

// --- Context menu ---

type ContextMenuState = {
  x: number
  y: number
  path: string
  isDir: boolean
} | null

function ContextMenu({
  menu,
  onClose,
  onNewFile,
  onNewFolder,
  onRename,
  onDelete,
}: {
  menu: NonNullable<ContextMenuState>
  onClose: () => void
  onNewFile: (parentDir: string) => void
  onNewFolder: (parentDir: string) => void
  onRename: (path: string) => void
  onDelete: (path: string) => void
}) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [onClose])

  const parentDir = menu.isDir ? menu.path : menu.path.split('/').slice(0, -1).join('/')

  const items = [
    { label: 'New File', action: () => { onNewFile(parentDir); onClose() } },
    { label: 'New Folder', action: () => { onNewFolder(parentDir); onClose() } },
    { label: 'Rename', action: () => { onRename(menu.path); onClose() } },
    { type: 'sep' as const },
    { label: 'Delete', action: () => { onDelete(menu.path); onClose() }, danger: true },
  ]

  return (
    <div
      ref={ref}
      className="fixed z-50 py-1 rounded-lg shadow-lg min-w-[140px]"
      style={{
        left: menu.x,
        top: menu.y,
        backgroundColor: 'var(--color-surface-elevated)',
        border: '1px solid var(--color-border-primary)',
      }}
    >
      {items.map((item, i) =>
        'type' in item ? (
          <div key={i} className="my-1" style={{ borderTop: '1px solid var(--color-border-secondary)' }} />
        ) : (
          <button
            key={i}
            onClick={item.action}
            className="w-full text-left px-3 py-1.5 text-xs transition-colors"
            style={{ color: item.danger ? 'var(--color-error)' : 'var(--color-text-primary)' }}
            onMouseEnter={e => e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'}
            onMouseLeave={e => e.currentTarget.style.backgroundColor = 'transparent'}
          >
            {item.label}
          </button>
        )
      )}
    </div>
  )
}

// --- Tree ---

function buildTreeData(docs: Doc[]): TreeData[] {
  const root: TreeData = { id: '__root__', name: '', isDir: true, children: [] }

  for (const doc of docs) {
    const parts = doc.path.split('/')
    let current = root

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      const isLast = i === parts.length - 1
      const partPath = parts.slice(0, i + 1).join('/')

      if (!current.children) current.children = []
      let child = current.children.find(c => c.id === partPath)
      if (!child) {
        child = {
          id: partPath,
          name: part,
          isDir: isLast ? doc.is_dir : true,
          doc: isLast ? doc : undefined,
          children: isLast && !doc.is_dir ? undefined : [],
        }
        current.children.push(child)
      } else if (isLast) {
        child.doc = doc
        child.isDir = doc.is_dir
      }
      current = child
    }
  }

  function sortTree(nodes: TreeData[]) {
    nodes.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
      return a.name.localeCompare(b.name)
    })
    for (const n of nodes) {
      if (n.children) sortTree(n.children)
    }
  }
  if (root.children) sortTree(root.children)

  return root.children || []
}

function FileNode({
  node,
  style,
  dragHandle,
}: NodeRendererProps<TreeData>) {
  const data = node.data
  const isSelected = node.isSelected
  const indent = node.level * 16

  // Access context menu handler from tree props
  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    // Dispatch custom event with position and data
    window.dispatchEvent(new CustomEvent('doc-tree-context', {
      detail: { x: e.clientX, y: e.clientY, path: data.id, isDir: data.isDir },
    }))
  }

  if (data.isDir) {
    return (
      <div
        ref={dragHandle}
        style={{ ...style, paddingLeft: `${indent + 8}px` }}
        className="flex items-center gap-1 py-0.5 px-2 cursor-pointer transition-colors text-xs group"
        onClick={(e) => { e.stopPropagation(); node.toggle() }}
        onContextMenu={handleContextMenu}
        onMouseEnter={e => e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'}
        onMouseLeave={e => e.currentTarget.style.backgroundColor = 'transparent'}
      >
        <span style={{ transform: node.isOpen ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.12s', display: 'inline-flex' }}>
          <ChevronRight size={12} />
        </span>
        <Folder size={14} style={{ color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
        <span className="truncate" style={{ color: 'var(--color-text-secondary)' }}>{data.name}</span>
      </div>
    )
  }

  return (
    <div
      ref={dragHandle}
      style={{
        ...style,
        paddingLeft: `${indent + 24}px`,
        backgroundColor: isSelected ? 'var(--color-surface-active)' : 'transparent',
        color: isSelected ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
      }}
      className="flex items-center gap-1.5 py-0.5 px-2 cursor-pointer transition-colors text-xs group"
      onClick={(e) => { e.stopPropagation(); node.select(); node.activate() }}
      onContextMenu={handleContextMenu}
      onMouseEnter={e => { if (!isSelected) e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)' }}
      onMouseLeave={e => { if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent' }}
    >
      {getFileIcon(data.name)}
      <span className="flex-1 truncate">{data.name}</span>
    </div>
  )
}

// --- BlockNote editor ---

function MarkdownEditor({
  content,
  onChange,
  darkMode,
}: {
  content: string
  onChange: (md: string) => void
  darkMode: boolean
}) {
  const initializedRef = useRef(false)
  const editor = useCreateBlockNote({})

  useEffect(() => {
    if (!editor) return
    initializedRef.current = false
    async function load() {
      const blocks = await editor.tryParseMarkdownToBlocks(content)
      editor.replaceBlocks(editor.document, blocks)
      // Delay setting initialized to skip the onChange fired by replaceBlocks
      requestAnimationFrame(() => { initializedRef.current = true })
    }
    load()
  }, [editor, content])

  const handleChange = useCallback(() => {
    if (!editor || !initializedRef.current) return
    async function exportMd() {
      const md = await editor.blocksToMarkdownLossy(editor.document)
      onChange(md)
    }
    exportMd()
  }, [editor, onChange])

  return (
    <div className="blocknote-wrapper">
      <BlockNoteView
        editor={editor}
        editable={true}
        onChange={handleChange}
        theme={darkMode ? 'dark' : 'light'}
      />
    </div>
  )
}

// --- Main ---

function Documents() {
  const [docs, setDocs] = useState<Doc[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedDoc, setSelectedDoc] = useState<Doc | null>(null)
  const [dirty, setDirty] = useState(false)
  const [editContent, setEditContent] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [searchTerm, setSearchTerm] = useState('')
  const [contextMenu, setContextMenu] = useState<ContextMenuState>(null)
  const treeContainerRef = useRef<HTMLDivElement>(null)
  const [treeHeight, setTreeHeight] = useState(400)
  // For inline creation
  const [inlineNew, setInlineNew] = useState<{ parentDir: string; type: 'file' | 'folder' } | null>(null)
  const [inlineNewName, setInlineNewName] = useState('')
  // For rename
  const [renaming, setRenaming] = useState<string | null>(null)
  const [renameValue, setRenameValue] = useState('')

  const darkMode = document.documentElement.classList.contains('dark')

  // Listen for context menu events from tree nodes
  useEffect(() => {
    function handler(e: Event) {
      const detail = (e as CustomEvent).detail
      setContextMenu(detail)
    }
    window.addEventListener('doc-tree-context', handler)
    return () => window.removeEventListener('doc-tree-context', handler)
  }, [])

  const loadDocs = useCallback(async () => {
    setLoading(true)
    try {
      const all: Doc[] = []
      async function listDir(path: string) {
        const url = path ? `/api/documents/${path}` : '/api/documents'
        const entries = await api.get<Doc[]>(url)
        if (!entries) return
        for (const entry of entries) {
          all.push(entry)
          if (entry.is_dir) {
            await listDir(entry.path)
          }
        }
      }
      await listDir('')
      setDocs(all)
    } catch {
      setDocs([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadDocs() }, [loadDocs])

  useEffect(() => {
    const el = treeContainerRef.current
    if (!el) return
    const ro = new ResizeObserver(entries => {
      for (const entry of entries) setTreeHeight(entry.contentRect.height)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const treeData = useMemo(() => buildTreeData(docs), [docs])

  async function selectDoc(doc: Doc) {
    if (doc.is_dir) return
    // Auto-save current doc if dirty
    if (dirty && selectedDoc) {
      await saveCurrentDoc()
    }
    setError('')
    setDirty(false)
    try {
      const full = await api.get<Doc>(`/api/documents/${doc.path}`)
      setSelectedDoc(full)
      setEditContent(full.content || '')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load document')
    }
  }

  async function saveCurrentDoc() {
    if (!selectedDoc) return
    setSaving(true)
    setError('')
    try {
      await api.put(`/api/documents/${selectedDoc.path}`, { content: editContent })
      setSelectedDoc({ ...selectedDoc, content: editContent })
      setDirty(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  function handleEditorChange(md: string) {
    setEditContent(md)
    setDirty(true)
  }

  // Ctrl+S / Cmd+S to save
  useEffect(() => {
    function handler(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        if (dirty && selectedDoc) saveCurrentDoc()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  })

  async function deleteDoc(path: string) {
    setError('')
    try {
      await api.del(`/api/documents/${path}`)
      if (selectedDoc?.path === path) {
        setSelectedDoc(null)
        setDirty(false)
      }
      await loadDocs()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
    setContextMenu(null)
  }

  async function handleNewFile(parentDir: string) {
    setInlineNew({ parentDir, type: 'file' })
    setInlineNewName('')
  }

  async function handleNewFolder(parentDir: string) {
    setInlineNew({ parentDir, type: 'folder' })
    setInlineNewName('')
  }

  async function commitInlineNew() {
    if (!inlineNew || !inlineNewName.trim()) {
      setInlineNew(null)
      return
    }
    const name = inlineNewName.trim()
    const fullPath = inlineNew.parentDir ? `${inlineNew.parentDir}/${name}` : name
    setError('')
    try {
      if (inlineNew.type === 'folder') {
        // Create a folder by creating a placeholder file inside it
        await api.put(`/api/documents/${fullPath}/.gitkeep`, { content: '' })
      } else {
        await api.put(`/api/documents/${fullPath}`, { content: '' })
      }
      setInlineNew(null)
      setInlineNewName('')
      await loadDocs()
      // Auto-select newly created file
      if (inlineNew.type === 'file') {
        const full = await api.get<Doc>(`/api/documents/${fullPath}`)
        setSelectedDoc(full)
        setEditContent(full.content || '')
        setDirty(false)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    }
  }

  async function handleRename(path: string) {
    const name = path.split('/').pop() || ''
    setRenaming(path)
    setRenameValue(name)
  }

  async function commitRename() {
    if (!renaming || !renameValue.trim()) {
      setRenaming(null)
      return
    }
    const parts = renaming.split('/')
    parts[parts.length - 1] = renameValue.trim()
    const newPath = parts.join('/')
    if (newPath === renaming) {
      setRenaming(null)
      return
    }
    setError('')
    try {
      // Read, create new, delete old (no rename API on client)
      const doc = await api.get<Doc>(`/api/documents/${renaming}`)
      await api.put(`/api/documents/${newPath}`, { content: doc.content || '' })
      await api.del(`/api/documents/${renaming}`)
      if (selectedDoc?.path === renaming) {
        setSelectedDoc({ ...selectedDoc, path: newPath, content: doc.content || '' })
      }
      setRenaming(null)
      await loadDocs()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to rename')
    }
  }

  const isMarkdown = selectedDoc?.path.endsWith('.md')

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Documents"
        description="Notes, templates & knowledge"
        actions={
          <button
            onClick={() => handleNewFile('')}
            className="flex items-center gap-1.5 text-sm rounded-lg px-3 py-1.5 transition-colors"
            style={{ backgroundColor: 'var(--color-accent)', color: 'var(--color-accent-text)' }}
          >
            <Plus size={16} /> New
          </button>
        }
      />

      {error && <div className="px-6 py-2 text-sm" style={{ color: 'var(--color-error)' }}>{error}</div>}

      <div className="flex-1 flex min-h-0">
        {/* File tree sidebar */}
        <div className="w-64 flex flex-col shrink-0" style={{ borderRight: '1px solid var(--color-border-primary)' }}>
          {/* Search */}
          <div className="p-2" style={{ borderBottom: '1px solid var(--color-border-secondary)' }}>
            <div className="flex items-center gap-1.5 px-2 py-1 rounded-md" style={{ backgroundColor: 'var(--color-surface-tertiary)' }}>
              <Search size={13} style={{ color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
              <input
                type="text"
                placeholder="Search files..."
                value={searchTerm}
                onChange={e => setSearchTerm(e.target.value)}
                className="flex-1 bg-transparent text-xs outline-none"
                style={{ color: 'var(--color-text-primary)' }}
              />
            </div>
          </div>

          {/* Inline new input */}
          {inlineNew && (
            <div className="px-3 py-2" style={{ borderBottom: '1px solid var(--color-border-secondary)' }}>
              <div className="text-xs mb-1" style={{ color: 'var(--color-text-tertiary)' }}>
                New {inlineNew.type} {inlineNew.parentDir ? `in ${inlineNew.parentDir}/` : 'in root'}
              </div>
              <input
                type="text"
                autoFocus
                placeholder={inlineNew.type === 'file' ? 'filename.md' : 'folder-name'}
                value={inlineNewName}
                onChange={e => setInlineNewName(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') commitInlineNew()
                  if (e.key === 'Escape') setInlineNew(null)
                }}
                onBlur={commitInlineNew}
                className="w-full rounded px-2 py-1 text-xs outline-none"
                style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-focus)', color: 'var(--color-text-primary)' }}
              />
            </div>
          )}

          {/* Rename input */}
          {renaming && (
            <div className="px-3 py-2" style={{ borderBottom: '1px solid var(--color-border-secondary)' }}>
              <div className="text-xs mb-1" style={{ color: 'var(--color-text-tertiary)' }}>
                Rename {renaming}
              </div>
              <input
                type="text"
                autoFocus
                value={renameValue}
                onChange={e => setRenameValue(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') commitRename()
                  if (e.key === 'Escape') setRenaming(null)
                }}
                onBlur={commitRename}
                className="w-full rounded px-2 py-1 text-xs outline-none"
                style={{ backgroundColor: 'var(--color-surface-tertiary)', border: '1px solid var(--color-border-focus)', color: 'var(--color-text-primary)' }}
              />
            </div>
          )}

          {/* Tree */}
          <div
            ref={treeContainerRef}
            className="flex-1 overflow-hidden"
            onContextMenu={(e) => {
              e.preventDefault()
              setContextMenu({ x: e.clientX, y: e.clientY, path: '', isDir: true })
            }}
          >
            {loading ? (
              <div className="text-xs text-center py-4" style={{ color: 'var(--color-text-tertiary)' }}>Loading...</div>
            ) : treeData.length === 0 ? (
              <div className="text-xs text-center py-4" style={{ color: 'var(--color-text-tertiary)' }}>No documents</div>
            ) : (
              <Tree<TreeData>
                data={treeData}
                width="100%"
                height={treeHeight}
                rowHeight={28}
                indent={16}
                openByDefault={true}
                searchTerm={searchTerm}
                searchMatch={(node, term) =>
                  node.data.name.toLowerCase().includes(term.toLowerCase())
                }
                selection={selectedDoc?.path}
                disableDrag={true}
                disableDrop={true}
                disableEdit={true}
                onActivate={(node) => {
                  if (node.data.doc && !node.data.isDir) {
                    selectDoc(node.data.doc)
                  }
                }}
              >
                {FileNode}
              </Tree>
            )}
          </div>
        </div>

        {/* Content area */}
        <div className="flex-1 flex flex-col overflow-hidden">
          {selectedDoc ? (
            <>
              <div className="flex items-center justify-between px-4 py-2 shrink-0" style={{ borderBottom: '1px solid var(--color-border-secondary)' }}>
                <div className="flex items-center gap-2 min-w-0">
                  {getFileIcon(selectedDoc.path)}
                  <span className="text-sm font-medium truncate" style={{ color: 'var(--color-text-primary)' }}>{selectedDoc.path}</span>
                  {dirty && <span className="text-xs px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-surface-tertiary)', color: 'var(--color-text-tertiary)' }}>unsaved</span>}
                </div>
                <button
                  onClick={saveCurrentDoc}
                  disabled={saving || !dirty}
                  className="text-xs rounded-md px-3 py-1 transition-colors"
                  style={{
                    backgroundColor: dirty ? 'var(--color-accent)' : 'var(--color-surface-tertiary)',
                    color: dirty ? 'var(--color-accent-text)' : 'var(--color-text-tertiary)',
                    cursor: dirty ? 'pointer' : 'default',
                  }}
                >
                  {saving ? 'Saving...' : 'Save'}
                </button>
              </div>
              <div className="flex-1 overflow-y-auto">
                {isMarkdown ? (
                  <MarkdownEditor
                    key={selectedDoc.path}
                    content={selectedDoc.content || ''}
                    onChange={handleEditorChange}
                    darkMode={darkMode}
                  />
                ) : (
                  <textarea
                    value={editContent}
                    onChange={e => { setEditContent(e.target.value); setDirty(true) }}
                    className="w-full h-full p-6 text-sm font-mono resize-none outline-none"
                    style={{ backgroundColor: 'var(--color-surface-primary)', color: 'var(--color-text-primary)' }}
                    spellCheck={false}
                  />
                )}
              </div>
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center">
                <FileIconGeneric size={32} style={{ color: 'var(--color-text-quaternary)', margin: '0 auto 12px' }} />
                <div className="text-sm" style={{ color: 'var(--color-text-tertiary)' }}>Select a document to view</div>
                <div className="text-xs mt-1" style={{ color: 'var(--color-text-quaternary)' }}>Right-click the tree for options</div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Context menu */}
      {contextMenu && (
        <ContextMenu
          menu={contextMenu}
          onClose={() => setContextMenu(null)}
          onNewFile={handleNewFile}
          onNewFolder={handleNewFolder}
          onRename={handleRename}
          onDelete={deleteDoc}
        />
      )}
    </div>
  )
}

export default Documents
