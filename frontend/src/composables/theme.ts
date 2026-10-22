import { ref } from 'vue'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'boodou-theme'

// Storage can throw in private mode or when site data is blocked; the theme
// then simply falls back to the system preference.
function readStored(): Theme | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return v === 'light' || v === 'dark' ? v : null
  } catch {
    return null
  }
}

function writeStored(theme: Theme) {
  try {
    localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // Non-fatal: the choice still applies for this session.
  }
}

function systemTheme(): Theme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const theme = ref<Theme>(readStored() ?? systemTheme())

function apply(value: Theme) {
  document.documentElement.dataset.theme = value
  document.querySelector('meta[name="theme-color"]')?.setAttribute(
    'content',
    getComputedStyle(document.documentElement).getPropertyValue('--color-bg').trim(),
  )
}

apply(theme.value)

export function useTheme() {
  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    apply(theme.value)
    writeStored(theme.value)
  }
  return { theme, toggle }
}
