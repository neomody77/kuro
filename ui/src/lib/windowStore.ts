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

export function saveWindowLayout(windows: WindowState[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(windows))
  } catch { /* */ }
}

export function getNextZIndex() {
  return nextZIndex++
}

let counter = 0
export function createWindowId() {
  return `win_${Date.now()}_${counter++}`
}
