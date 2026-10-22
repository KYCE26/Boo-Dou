import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { ChatError, isAbort, streamChat, type ChatMessage } from '../api/chat'

export type MascotState = 'idle' | 'thinking' | 'happy' | 'error'
export type ChatStatus = 'idle' | 'streaming' | 'error'

export interface Message extends ChatMessage {
  id: number
  // Set when the user stopped the stream, so the UI can mark the answer as unfinished.
  stopped?: boolean
}

const HAPPY_MS = 1600

export const useChatStore = defineStore('chat', () => {
  const messages = ref<Message[]>([])
  const status = ref<ChatStatus>('idle')
  const error = ref<string | null>(null)
  const celebrating = ref(false)

  let nextId = 1
  let controller: AbortController | null = null
  let happyTimer: ReturnType<typeof setTimeout> | undefined

  const isStreaming = computed(() => status.value === 'streaming')
  const isEmpty = computed(() => messages.value.length === 0)

  const mascotState = computed<MascotState>(() => {
    if (status.value === 'streaming') return 'thinking'
    if (status.value === 'error') return 'error'
    return celebrating.value ? 'happy' : 'idle'
  })

  function celebrate() {
    clearTimeout(happyTimer)
    celebrating.value = true
    happyTimer = setTimeout(() => (celebrating.value = false), HAPPY_MS)
  }

  async function run() {
    controller = new AbortController()
    status.value = 'streaming'
    error.value = null
    celebrating.value = false

    const history: ChatMessage[] = messages.value.map(({ role, content }) => ({ role, content }))
    const reply: Message = { id: nextId++, role: 'assistant', content: '' }
    messages.value.push(reply)
    // Mutate through the reactive proxy, not the raw object, so the UI updates per token.
    const live = messages.value[messages.value.length - 1]!

    try {
      await streamChat(history, {
        signal: controller.signal,
        onToken: (text) => (live.content += text),
      })
      status.value = 'idle'
      celebrate()
    } catch (err) {
      if (isAbort(err)) {
        status.value = 'idle'
        if (live.content) live.stopped = true
        else removeMessage(live.id)
        return
      }
      // A half-written answer would be resent as context on retry, so drop it.
      removeMessage(live.id)
      error.value = err instanceof ChatError ? err.message : 'Terjadi kesalahan yang tidak diketahui.'
      status.value = 'error'
    } finally {
      controller = null
    }
  }

  function send(text: string) {
    const content = text.trim()
    if (!content || isStreaming.value) return
    messages.value.push({ id: nextId++, role: 'user', content })
    return run()
  }

  function retry() {
    if (isStreaming.value || messages.value.at(-1)?.role !== 'user') return
    return run()
  }

  function stop() {
    controller?.abort()
  }

  function removeMessage(id: number) {
    messages.value = messages.value.filter((m) => m.id !== id)
  }

  return { messages, status, error, isStreaming, isEmpty, mascotState, send, retry, stop }
})
