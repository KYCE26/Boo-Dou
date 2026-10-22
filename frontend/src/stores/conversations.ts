import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '../api/conversations'
import type { Conversation } from '../api/conversations'

export type ListState = 'idle' | 'loading' | 'ready' | 'error'

export const useConversationsStore = defineStore('conversations', () => {
  const items = ref<Conversation[]>([])
  const state = ref<ListState>('idle')
  const error = ref<string | null>(null)

  async function load() {
    state.value = 'loading'
    error.value = null
    try {
      items.value = await api.listConversations()
      state.value = 'ready'
    } catch (err) {
      error.value = (err as Error).message
      state.value = 'error'
    }
  }

  // Quiet refresh after a message: titles and ordering change server-side,
  // but a failure here must not replace a working list with an error.
  async function refresh() {
    try {
      items.value = await api.listConversations()
      state.value = 'ready'
    } catch {
      // Keep showing the last known list.
    }
  }

  async function create() {
    const conv = await api.createConversation()
    items.value = [conv, ...items.value]
    return conv
  }

  async function rename(id: string, title: string) {
    const updated = await api.renameConversation(id, title)
    items.value = items.value.map((c) => (c.id === id ? updated : c))
  }

  async function remove(id: string) {
    await api.deleteConversation(id)
    items.value = items.value.filter((c) => c.id !== id)
  }

  return { items, state, error, load, refresh, create, rename, remove }
})
