import { useRef, useCallback } from 'react'
import { Rnd } from 'react-rnd'
import { Minus, Maximize2, X } from '../Icons'

export type WindowState = {
  id: string
  appId: string
  title: string
  x: number
  y: number
  width: number
  height: number
  minimized: boolean
  maximized: boolean
  zIndex: number
}

type Props = {
  state: WindowState
  onClose: (id: string) => void
  onMinimize: (id: string) => void
  onMaximize: (id: string) => void
  onFocus: (id: string) => void
  onMove: (id: string, x: number, y: number) => void
  onResize: (id: string, w: number, h: number, x: number, y: number) => void
  children: React.ReactNode
}

export default function Window({ state, onClose, onMinimize, onMaximize, onFocus, onMove, onResize, children }: Props) {
  const rndRef = useRef<Rnd>(null)

  const handleMaximize = useCallback(() => {
    onMaximize(state.id)
  }, [state.id, onMaximize])

  if (state.minimized) return null

  const pos = state.maximized
    ? { x: 0, y: 0 }
    : { x: state.x, y: state.y }
  const size = state.maximized
    ? { width: '100%', height: 'calc(100% - 48px)' }
    : { width: state.width, height: state.height }

  return (
    <Rnd
      ref={rndRef}
      position={pos}
      size={size as any}
      minWidth={320}
      minHeight={200}
      style={{ zIndex: state.zIndex }}
      disableDragging={state.maximized}
      enableResizing={!state.maximized}
      dragHandleClassName="window-drag-handle"
      onDragStop={(_e, d) => onMove(state.id, d.x, d.y)}
      onResizeStop={(_e, _dir, ref, _delta, position) => {
        onResize(state.id, ref.offsetWidth, ref.offsetHeight, position.x, position.y)
      }}
      onMouseDown={() => onFocus(state.id)}
      bounds="parent"
    >
      <div
        className="flex flex-col h-full rounded-lg overflow-hidden"
        style={{
          backgroundColor: 'var(--color-surface-primary)',
          border: '1px solid var(--color-border-primary)',
          boxShadow: '0 8px 32px rgba(0,0,0,0.12), 0 2px 8px rgba(0,0,0,0.08)',
          ...(state.maximized ? { borderRadius: 0 } : {}),
        }}
      >
        {/* Title bar */}
        <div
          className="window-drag-handle flex items-center justify-between h-9 shrink-0 px-3 select-none"
          style={{
            backgroundColor: 'var(--color-surface-secondary)',
            borderBottom: '1px solid var(--color-border-primary)',
          }}
          onDoubleClick={handleMaximize}
        >
          <span className="text-xs font-medium truncate" style={{ color: 'var(--color-text-primary)' }}>
            {state.title}
          </span>
          <div className="flex items-center gap-0.5 shrink-0">
            <button
              onClick={() => onMinimize(state.id)}
              className="p-1 rounded transition-colors"
              style={{ color: 'var(--color-text-tertiary)' }}
              onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
              onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
            >
              <Minus size={12} />
            </button>
            <button
              onClick={handleMaximize}
              className="p-1 rounded transition-colors"
              style={{ color: 'var(--color-text-tertiary)' }}
              onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-surface-hover)'; e.currentTarget.style.color = 'var(--color-text-primary)' }}
              onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
            >
              <Maximize2 size={12} />
            </button>
            <button
              onClick={() => onClose(state.id)}
              className="p-1 rounded transition-colors"
              style={{ color: 'var(--color-text-tertiary)' }}
              onMouseEnter={e => { e.currentTarget.style.backgroundColor = 'var(--color-error)'; e.currentTarget.style.color = '#fff' }}
              onMouseLeave={e => { e.currentTarget.style.backgroundColor = 'transparent'; e.currentTarget.style.color = 'var(--color-text-tertiary)' }}
            >
              <X size={12} />
            </button>
          </div>
        </div>
        {/* Content */}
        <div className="flex-1 overflow-auto">
          {children}
        </div>
      </div>
    </Rnd>
  )
}
