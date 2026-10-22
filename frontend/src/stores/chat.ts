import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { ChatError, isAbort, streamConversationChat, type ChatRequest } from '../api/chat'
import { getConversation, type Attachment, type MessageStatus } from '../api/conversations'
import { ApiError } from '../api/http'
import { useConversationsStore } from './conversations'
import { copy, type ErrorKind } from '../copy'
import { errorKind } from '../lib/errors'

export type MascotState = 'idle' | 'thinking' | 'happy' | 'error'
export type ChatStatus = 'idle' | 'streaming' | 'error'
export type LoadState = 'ready' | 'loading' | 'error' | 'notfound'

export interface Message {
  localId: number
  id: string | null
  role: 'user' | 'assistant'
  content: string
  // 'streaming' exists only in the browser, while an answer is arriving.
  status: MessageStatus | 'streaming'
  attachments: Attachment[]
}

const HAPPY_MS = 1600

export const useChatStore = defineStore('chat', () => {
  const conversationId = ref<string | null>(null)
  const messages = ref<Message[]>([])
  const loadState = ref<LoadState>('ready')
  const loadError = ref<string | null>(null)
  const status = ref<ChatStatus>('idle')
  const error = ref<string | null>(null)
  const errorType = ref<ErrorKind>('unknown')
  const celebrating = ref(false)

  let nextId = 1
  let controller: AbortController | null = null
  let happyTimer: ReturnType<typeof setTimeout> | undefined
  let loadToken = 0

  const isStreaming = computed(() => status.value === 'streaming')
  const isEmpty = computed(() => messages.value.length === 0)

  const mascotState = computed<MascotState>(() => {
    if (status.value === 'streaming') return 'thinking'
    if (status.value === 'error' || loadState.value === 'error') return 'error'
    return celebrating.value ? 'happy' : 'idle'
  })

  function celebrate() {
    clearTimeout(happyTimer)
    celebrating.value = true
    happyTimer = setTimeout(() => (celebrating.value = false), HAPPY_MS)
  }

  function resetState() {
    controller?.abort()
    messages.value = []
    status.value = 'idle'
    error.value = null
    celebrating.value = false
  }

  // "/" is a conversation that does not exist yet; it is created on send.
  function startNew() {
    loadToken++
    resetState()
    conversationId.value = null
    loadState.value = 'ready'
  }

  async function open(id: string) {
    if (id === conversationId.value && loadState.value === 'ready') return
    const token = ++loadToken
    resetState()
    conversationId.value = id
    loadState.value = 'loading'
    loadError.value = null
    try {
      const detail = await getConversation(id)
      // A newer navigation started while this one was loading.
      if (token !== loadToken) return
      messages.value = detail.messages
        .filter((m) => !(m.role === 'assistant' && m.status === 'error' && !m.content))
        .map((m) => ({ localId: nextId++, id: m.id, role: m.role, content: m.content, status: m.status, attachments: m.attachments }))
      const last = detail.messages.at(-1)
      if (last && (last.role === 'user' || last.status === 'error')) {
        status.value = 'error'
        error.value = copy.errors.lastFailed
        errorType.value = 'unknown'
      }
      loadState.value = 'ready'
    } catch (err) {
      if (token !== loadToken) return
      loadState.value = err instanceof ApiError && err.status === 404 ? 'notfound' : 'error'
      loadError.value = (err as Error).message
    }
  }

  async function run(body: ChatRequest) {
    const id = conversationId.value
    if (!id) return
    controller = new AbortController()
    status.value = 'streaming'
    error.value = null
    celebrating.value = false

    messages.value.push({ localId: nextId++, id: null, role: 'assistant', content: '', status: 'streaming', attachments: [] })
    // Mutate through the reactive proxy, not the raw object, so the UI updates per token.
    const live = messages.value[messages.value.length - 1]!
    const conversations = useConversationsStore()
    let titleRefreshed = false

    try {
      const result = await streamConversationChat(id, body, {
        signal: controller.signal,
        onToken: (text) => {
          live.content += text
          // The server saves the title with the user message; refresh once so
          // the list stops showing an untitled conversation mid-answer.
          if (!titleRefreshed) {
            titleRefreshed = true
            void conversations.refresh()
          }
        },
      })
      live.id = result.messageId
      live.status = 'complete'
      status.value = 'idle'
      celebrate()
    } catch (err) {
      if (isAbort(err)) {
        // The server sees the disconnect and saves the partial text as stopped.
        status.value = 'idle'
        if (live.content) live.status = 'stopped'
        else removeMessage(live.localId)
        return
      }
      if (live.content) live.status = 'error'
      else removeMessage(live.localId)
      error.value = err instanceof ChatError ? err.message : 'Terjadi kesalahan yang tidak diketahui.'
      errorType.value = errorKind(err instanceof ChatError ? err.status : -1, navigator.onLine)
      status.value = 'error'
    } finally {
      controller = null
      void conversations.refresh()
    }
  }

  // Returns false when the conversation could not be created, so the caller
  // can keep the draft instead of losing what the user typed.
  async function send(text: string, attachments: Attachment[]): Promise<boolean> {
    const content = text.trim()
    if ((!content && attachments.length === 0) || isStreaming.value) return false

    if (!conversationId.value) {
      try {
        const conv = await useConversationsStore().create()
        conversationId.value = conv.id
        loadState.value = 'ready'
      } catch (err) {
        status.value = 'error'
        error.value = (err as Error).message
        errorType.value = errorKind((err as { status?: number }).status ?? 0, navigator.onLine)
        return false
      }
    }

    messages.value.push({ localId: nextId++, id: null, role: 'user', content, status: 'complete', attachments })
    void run({ text: content, attachment_ids: attachments.map((a) => a.id) })
    return true
  }

  function retry() {
    if (isStreaming.value || !conversationId.value) return
    while (messages.value.at(-1)?.role === 'assistant' && messages.value.at(-1)?.status === 'error') {
      messages.value.pop()
    }
    if (messages.value.at(-1)?.role !== 'user') return
    return run({ retry: true })
  }

  function stop() {
    controller?.abort()
  }

  function removeMessage(localId: number) {
    messages.value = messages.value.filter((m) => m.localId !== localId)
  }

  return {
    conversationId, messages, loadState, loadError, status, error, errorType,
    isStreaming, isEmpty, mascotState, startNew, open, send, retry, stop,
  }
})
