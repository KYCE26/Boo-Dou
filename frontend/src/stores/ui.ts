import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getHealth } from '../api/health'

const DESKTOP_QUERY = '(min-width: 900px)'

export const useUiStore = defineStore('ui', () => {
  // False until /api/health says otherwise, so image controls never flash
  // in for a text-only model.
  const vision = ref(false)
  const drawerOpen = ref(false)
  // Drives the offline notice; the service worker keeps the shell available.
  const online = ref(navigator.onLine)
  window.addEventListener('online', () => (online.value = true))
  window.addEventListener('offline', () => (online.value = false))

  const media = window.matchMedia(DESKTOP_QUERY)
  const desktop = ref(media.matches)
  media.addEventListener('change', (e) => {
    desktop.value = e.matches
    if (e.matches) drawerOpen.value = false
  })

  async function loadHealth() {
    try {
      vision.value = (await getHealth()).llm_vision === true
    } catch {
      vision.value = false
    }
  }

  return { vision, drawerOpen, desktop, online, loadHealth }
})
