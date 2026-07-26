export type ThemeMode = 'light' | 'dark'

const STORAGE_KEY = 'kyc_theme'

export function getStoredTheme(): ThemeMode {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'dark' || v === 'light') return v
  } catch {
    /* ignore */
  }
  try {
    if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      if (window.matchMedia('(prefers-color-scheme: dark)').matches) return 'dark'
    }
  } catch {
    /* ignore */
  }
  return 'light'
}

export function applyTheme(mode: ThemeMode) {
  document.documentElement.dataset.theme = mode
  // swagger-ui-react ≥5.31 gates its dark stylesheet on html.dark-mode
  document.documentElement.classList.toggle('dark-mode', mode === 'dark')
}

export function setTheme(mode: ThemeMode) {
  try {
    localStorage.setItem(STORAGE_KEY, mode)
  } catch {
    /* ignore */
  }
  applyTheme(mode)
}

export function toggleTheme(): ThemeMode {
  const next: ThemeMode = getStoredTheme() === 'dark' ? 'light' : 'dark'
  setTheme(next)
  return next
}

export function initTheme() {
  applyTheme(getStoredTheme())
}
