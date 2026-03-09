import type { WindowState } from '../components/desktop/Window'

const STORAGE_KEY = 'kuro:windowLayout'
let nextZIndex = 100

export function loadWindowLayout(): WindowState[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) {
        // Restore zIndex counter
        for (const w of parsed) {
          if (w.zIndex >= nextZIndex) nextZIndex = w.zIndex + 1
        }
        return parsed
      }
    }
  } catch { /* */ }
  return []
}

/**
 * Load layout from server, falling back to localStorage.
 * Call this once on app init to hydrate from the server-side layout.
 */
export async function loadWindowLayoutFromServer(): Promise<WindowState[]> {
  try {
    const res = await fetch('/api/settings/layout')
    if (res.ok) {
      const parsed = await res.json()
      if (Array.isArray(parsed) && parsed.length > 0) {
        // Restore zIndex counter
        for (const w of parsed) {
          if (w.zIndex >= nextZIndex) nextZIndex = w.zIndex + 1
        }
        // Also update localStorage as cache
        try {
          localStorage.setItem(STORAGE_KEY, JSON.stringify(parsed))
        } catch { /* */ }
        return parsed
      }
    }
  } catch { /* server unreachable, fall back */ }

  // Fall back to localStorage
  return loadWindowLayout()
}

export function saveWindowLayout(windows: WindowState[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(windows))
  } catch { /* */ }

  // Fire-and-forget save to server
  saveWindowLayoutToServer(windows)
}

/** Debounced save to server to avoid flooding on rapid window moves/resizes. */
let saveTimer: ReturnType<typeof setTimeout> | null = null

function saveWindowLayoutToServer(windows: WindowState[]) {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    fetch('/api/settings/layout', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(windows),
    }).catch(() => { /* ignore network errors */ })
  }, 500)
}

export function getNextZIndex() {
  return nextZIndex++
}

let counter = 0
export function createWindowId() {
  return `win_${Date.now()}_${counter++}`
}
