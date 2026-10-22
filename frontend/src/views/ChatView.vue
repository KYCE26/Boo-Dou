<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import BrimEdge from '../components/BrimEdge.vue'
import ChatComposer from '../components/ChatComposer.vue'
import GhostMascot from '../components/GhostMascot.vue'
import ImageViewer from '../components/ImageViewer.vue'
import MarkdownContent from '../components/MarkdownContent.vue'
import { attachmentUrl, type Attachment } from '../api/conversations'
import { copy } from '../copy'
import { useChatStore } from '../stores/chat'
import { useConversationsStore } from '../stores/conversations'
import { useUiStore } from '../stores/ui'
import { useTheme } from '../composables/theme'

const chat = useChatStore()
const ui = useUiStore()
const conversations = useConversationsStore()
const route = useRoute()
const router = useRouter()
const { messages, error, errorType, isStreaming, isEmpty, mascotState, status, loadState, loadError } = storeToRefs(chat)
const { theme, toggle: toggleTheme } = useTheme()

const listEl = ref<HTMLElement | null>(null)
const composer = ref<InstanceType<typeof ChatComposer> | null>(null)
const viewing = ref<{ image: Attachment; label: string; origin: HTMLElement | null } | null>(null)
const dragging = ref(false)
let dragDepth = 0

const routeId = computed(() => (typeof route.params.id === 'string' ? route.params.id : null))
const currentTitle = computed(() => conversations.items.find((c) => c.id === chat.conversationId)?.title ?? '')

watch(
  routeId,
  (id) => {
    if (id) void chat.open(id)
    else chat.startNew()
  },
  { immediate: true },
)

// A conversation created by the first message moves the URL from "/" to
// /c/:id without reloading what is already on screen.
watch(
  () => chat.conversationId,
  (id) => {
    if (id && routeId.value !== id) void router.replace(`/c/${id}`)
  },
)

function sendMessage(text: string, attachments: Attachment[]) {
  return chat.send(text, attachments)
}

// Escape stops a running answer, matching the Hentikan button, unless a
// dialog or an inline editor has already claimed the key.
function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape' || !isStreaming.value || e.defaultPrevented) return
  if (document.querySelector('dialog[open]')) return
  e.preventDefault()
  chat.stop()
}

onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKeydown))

// Only follow new tokens when the reader is already near the bottom, so
// scrolling up to reread is not yanked back on every token.
watch(
  () => messages.value.map((m) => m.content.length).join(),
  async () => {
    const el = listEl.value
    if (!el) return
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 120
    await nextTick()
    if (nearBottom) el.scrollTop = el.scrollHeight
  },
)

watch(loadState, async (s) => {
  if (s !== 'ready') return
  await nextTick()
  if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight
})

const THUMB_HEIGHT = 96
const thumbWidth = (a: Attachment) => Math.min(160, Math.max(48, Math.round((a.width / a.height) * THUMB_HEIGHT)))

function openViewer(image: Attachment, index: number, total: number, origin: EventTarget | null) {
  viewing.value = { image, label: copy.conversation.imageLabel(index + 1, total), origin: origin as HTMLElement | null }
}

const hasFiles = (e: DragEvent) => Array.from(e.dataTransfer?.types ?? []).includes('Files')

function onDragEnter(e: DragEvent) {
  if (!ui.vision || !hasFiles(e)) return
  dragDepth++
  dragging.value = true
}

function onDragLeave() {
  if (!dragging.value) return
  dragDepth = Math.max(0, dragDepth - 1)
  if (dragDepth === 0) dragging.value = false
}

function onDrop(e: DragEvent) {
  dragDepth = 0
  dragging.value = false
  if (!ui.vision) return
  composer.value?.addFiles(Array.from(e.dataTransfer?.files ?? []))
}

const isLastStreaming = (i: number) => isStreaming.value && i === messages.value.length - 1
</script>

<template>
  <div
    class="page"
    @dragenter.prevent="onDragEnter"
    @dragover.prevent
    @dragleave="onDragLeave"
    @drop.prevent="onDrop"
  >
    <header class="topbar">
      <div class="topbar-row">
        <button
          v-if="!ui.desktop"
          type="button"
          class="button button-quiet menu-button"
          :aria-label="copy.header.openListLabel"
          aria-controls="conversation-drawer"
          :aria-expanded="ui.drawerOpen"
          @click="ui.drawerOpen = true"
        >
          <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
            <path d="M4 7h16M4 12h16M4 17h10" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
          </svg>
          <span class="menu-text">{{ copy.header.openList }}</span>
        </button>
        <div class="brand">
          <GhostMascot :state="mascotState" :size="34" />
          <span class="brand-name">{{ copy.brand }}</span>
          <span v-if="currentTitle && !isEmpty" class="conv-title">{{ currentTitle }}</span>
        </div>
        <button
          type="button"
          class="icon-button bare"
          :aria-label="theme === 'dark' ? copy.header.themeToLight : copy.header.themeToDark"
          :aria-pressed="theme === 'dark'"
          @click="toggleTheme"
        >
          <svg v-if="theme === 'dark'" viewBox="0 0 24 24" width="21" height="21" aria-hidden="true">
            <circle cx="12" cy="12" r="4.5" fill="none" stroke="currentColor" stroke-width="1.8" />
            <path
              d="M12 2v2.5M12 19.5V22M2 12h2.5M19.5 12H22M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M4.9 19.1l1.8-1.8M17.3 6.7l1.8-1.8"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
          <svg v-else viewBox="0 0 24 24" width="21" height="21" aria-hidden="true">
            <path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
          </svg>
        </button>
      </div>
      <BrimEdge class="topbar-edge" />
    </header>

    <main ref="listEl" class="conversation">
      <p v-if="!ui.online" class="offline-note" role="status">
        <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
          <path d="M3 3l18 18M8.5 8.6A6 6 0 0 0 6 13.5 4 4 0 0 0 7 21h10.5M12 6a6 6 0 0 1 5.7 4.1A4.5 4.5 0 0 1 21 17.5" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
        </svg>
        {{ copy.offline }}
      </p>

      <div class="column">
        <section v-if="loadState === 'loading'" class="state" role="status">
          <GhostMascot state="thinking" :size="88" />
          <p class="state-lead">{{ copy.conversation.loading }}</p>
        </section>

        <section v-else-if="loadState === 'notfound'" class="state" role="alert">
          <GhostMascot state="error" :size="88" />
          <h1 class="state-title">{{ copy.conversation.notFoundTitle }}</h1>
          <p class="state-lead">{{ copy.conversation.notFoundBody }}</p>
          <RouterLink to="/" class="button button-primary">{{ copy.conversation.notFoundAction }}</RouterLink>
        </section>

        <section v-else-if="loadState === 'error'" class="state" role="alert">
          <GhostMascot state="error" :size="88" />
          <h1 class="state-title">{{ copy.conversation.loadErrorTitle }}</h1>
          <p class="state-lead">{{ loadError }}</p>
          <button v-if="routeId" type="button" class="button button-quiet" @click="chat.open(routeId)">{{ copy.conversation.loadErrorAction }}</button>
        </section>

        <section v-else-if="isEmpty && status !== 'error'" class="welcome">
          <div class="welcome-mascot">
            <GhostMascot :state="mascotState" :size="168" />
          </div>
          <div class="welcome-text">
            <h1 class="welcome-title">{{ copy.empty.title }}</h1>
            <p class="welcome-lead">{{ copy.empty.lead }}</p>
            <h2 class="visually-hidden">{{ copy.empty.suggestionsLabel }}</h2>
            <ul class="suggestions">
              <li v-if="ui.vision">
                <button type="button" class="suggestion" @click="composer?.openPicker()">
                  <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
                    <rect x="3" y="5" width="18" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="1.8" />
                    <circle cx="9" cy="10" r="1.8" fill="currentColor" />
                    <path d="m4 18 5-5 4 4 3-3 4 4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
                  </svg>
                  <span class="suggestion-text">
                    <span class="suggestion-title">{{ copy.empty.photo.title }}</span>
                    <span class="suggestion-detail">{{ copy.empty.photo.detail }}</span>
                  </span>
                </button>
              </li>
              <li>
                <button type="button" class="suggestion" @click="composer?.setText(copy.empty.code.prompt)">
                  <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
                    <path d="M8 7l-5 5 5 5M16 7l5 5-5 5M14 4l-4 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                  <span class="suggestion-text">
                    <span class="suggestion-title">{{ copy.empty.code.title }}</span>
                    <span class="suggestion-detail">{{ copy.empty.code.detail }}</span>
                  </span>
                </button>
              </li>
              <li>
                <button type="button" class="suggestion" @click="composer?.setText(copy.empty.rewrite.prompt)">
                  <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
                    <path d="M4 20h4L19 9l-4-4L4 16v4ZM14 6l4 4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
                  </svg>
                  <span class="suggestion-text">
                    <span class="suggestion-title">{{ copy.empty.rewrite.title }}</span>
                    <span class="suggestion-detail">{{ copy.empty.rewrite.detail }}</span>
                  </span>
                </button>
              </li>
            </ul>
          </div>
        </section>

        <template v-else-if="!isEmpty">
        <h1 class="visually-hidden">{{ currentTitle || copy.brand }}</h1>
        <ol class="messages" :aria-label="copy.conversation.listLabel">
          <li
            v-for="(msg, i) in messages"
            :key="msg.localId"
            class="message"
            :class="[msg.role === 'user' ? 'from-user' : 'from-assistant', { fresh: msg.fresh }]"
          >
            <span class="visually-hidden">{{ msg.role === 'user' ? copy.conversation.you : copy.conversation.assistant }}</span>
            <div v-if="isLastStreaming(i)" class="writing" aria-hidden="true">
              <GhostMascot state="thinking" :size="30" />
            </div>
            <div v-if="msg.attachments.length" class="thumbs">
              <button
                v-for="(a, j) in msg.attachments"
                :key="a.id"
                type="button"
                class="thumb"
                :aria-label="copy.conversation.viewImage(j + 1)"
                @click="openViewer(a, j, msg.attachments.length, $event.currentTarget)"
              >
                <img :src="attachmentUrl(a.id)" :width="thumbWidth(a)" :height="THUMB_HEIGHT" alt="" loading="lazy" />
              </button>
            </div>
            <p v-if="msg.role === 'user' && msg.content" class="user-text">{{ msg.content }}</p>
            <div v-else-if="msg.role === 'assistant' && msg.content" class="assistant-text">
              <MarkdownContent :source="msg.content" />
            </div>
            <p v-else-if="msg.role === 'assistant'" class="pending">
              <span class="visually-hidden">{{ copy.conversation.waiting }}</span>
            </p>
            <p v-if="msg.status === 'stopped'" class="status-label">{{ copy.conversation.stopped }}</p>
            <p v-else-if="msg.role === 'assistant' && msg.status === 'error'" class="status-label">{{ copy.conversation.brokenOff }}</p>
          </li>
        </ol>
        </template>

        <div v-if="status === 'error'" class="notice" role="alert">
          <GhostMascot state="error" :size="52" />
          <div class="notice-body">
            <p class="notice-title">{{ copy.errors.titles[errorType] }}</p>
            <p class="notice-message">{{ error }}</p>
            <p class="notice-next">{{ copy.errors.nextStep[errorType] }}</p>
            <button v-if="chat.conversationId" type="button" class="button button-quiet" @click="chat.retry()">
              {{ copy.errors.retry }}
            </button>
          </div>
        </div>
      </div>
    </main>

    <p class="visually-hidden" aria-live="polite">{{ copy.conversation.status[mascotState] }}</p>

    <ChatComposer
      ref="composer"
      :key="routeId ?? 'new'"
      :vision="ui.vision"
      :streaming="isStreaming"
      :online="ui.online"
      :send="sendMessage"
      @stop="chat.stop()"
    />

    <div v-if="dragging" class="drop-overlay" aria-hidden="true">
      <p>{{ copy.conversation.dropHint }}</p>
    </div>

    <ImageViewer :image="viewing?.image ?? null" :label="viewing?.label ?? ''" :origin="viewing?.origin ?? null" @close="viewing = null" />
  </div>
</template>

<style scoped>
.page {
  position: relative;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  /* An explicit column stops wide content from widening the page past the viewport. */
  grid-template-columns: minmax(0, 1fr);
  height: 100vh;
  height: 100dvh;
  min-width: 0;
}

.column {
  width: 100%;
  max-width: var(--content-max);
  margin-inline: auto;
  padding-inline: var(--gutter);
}

.topbar {
  position: relative;
  z-index: 1;
  padding-top: env(safe-area-inset-top);
  background: var(--color-bg);
}

.topbar-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 60px;
  padding: var(--space-2) var(--gutter) var(--space-1);
}

/* Identity motif: the header ends in the brim's curve instead of a straight rule. */
.topbar-edge {
  position: absolute;
  left: 0;
  right: 0;
  top: 100%;
  margin-top: -1px;
}

.menu-button {
  padding: 0 var(--space-3);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex: 1;
  min-width: 0;
}

.brand-name {
  font-size: var(--text-l);
  font-weight: 800;
  letter-spacing: -0.01em;
  color: var(--color-green);
}

.conv-title {
  min-width: 0;
  padding-left: var(--space-2);
  border-left: 1px solid var(--color-border);
  color: var(--color-muted);
  font-size: var(--text-s);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conversation {
  /* Containing block for the visually-hidden labels inside messages; without
     it they escape this scroller and stretch the page to the list height. */
  position: relative;
  overflow-y: auto;
  overflow-x: hidden;
  padding-block: var(--space-6) var(--space-7);
}

.offline-note {
  display: flex;
  gap: var(--space-2);
  align-items: flex-start;
  max-width: var(--content-max);
  margin: 0 auto var(--space-5);
  padding: var(--space-3) var(--gutter);
  background: var(--color-tea);
  font-size: var(--text-s);
}

.offline-note svg {
  flex-shrink: 0;
  margin-top: 2px;
}

.state {
  display: grid;
  justify-items: start;
  gap: var(--space-3);
  padding-block: var(--space-6);
}

.state-title {
  margin: 0;
  font-size: var(--text-l);
  font-weight: 800;
  color: var(--color-green);
}

.state-lead {
  margin: 0;
  max-width: 40ch;
  color: var(--color-muted);
  overflow-wrap: anywhere;
}

/* First run is the one loud moment: big mascot, display type, real actions. */
.welcome {
  display: grid;
  gap: var(--space-5);
  padding-block: var(--space-4) var(--space-6);
}

.welcome-title {
  margin: 0;
  font-size: var(--text-display);
  font-weight: 800;
  line-height: 1.05;
  letter-spacing: -0.025em;
  color: var(--color-green);
}

.welcome-lead {
  margin: var(--space-3) 0 var(--space-5);
  max-width: 38ch;
  font-size: var(--text-read);
  color: var(--color-muted);
}

.suggestions {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;
  border-top: 1px solid var(--color-border);
}

.suggestions li {
  border-bottom: 1px solid var(--color-border);
}

.suggestion {
  display: flex;
  gap: var(--space-3);
  align-items: center;
  width: 100%;
  min-height: 60px;
  padding: var(--space-3) var(--space-2);
  border: 0;
  background: transparent;
  color: var(--color-ink);
  text-align: left;
}

.suggestion svg {
  flex-shrink: 0;
  color: var(--color-green);
}

.suggestion:hover,
.suggestion:active {
  background: var(--color-surface);
}

.suggestion-text {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.suggestion-title {
  font-weight: 700;
}

.suggestion-detail {
  font-size: var(--text-s);
  color: var(--color-muted);
}

@media (min-width: 600px) {
  .welcome {
    grid-template-columns: auto 1fr;
    align-items: start;
    gap: var(--space-6);
    padding-top: var(--space-6);
  }

  .welcome-mascot {
    padding-top: var(--space-2);
  }
}

.messages {
  display: grid;
  gap: var(--space-6);
  margin: 0;
  padding: 0;
  list-style: none;
}

.message {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-2);
  min-width: 0;
}

.from-user {
  align-items: flex-end;
}

.user-text {
  max-width: min(85%, 34rem);
  margin: 0;
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-l) var(--radius-l) var(--radius-s) var(--radius-l);
  background: var(--color-tea);
  font-weight: 550;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

/* Answers sit on the paper itself: wider, larger, and airier than the user's lines. */
.assistant-text {
  width: 100%;
  min-width: 0;
  font-size: var(--text-read);
  line-height: 1.7;
}

.writing {
  display: flex;
  align-items: center;
  height: 36px;
  animation: writing-in 220ms ease-out both;
}

/* New lines arrive from the side that sent them: the user from the right,
   Boo Dou rising from below, so the eye knows who just spoke. */
.message.fresh {
  animation: 280ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.from-user.fresh {
  animation-name: msg-from-right;
}

.from-assistant.fresh {
  animation-name: msg-rise;
}

@keyframes msg-from-right {
  from {
    opacity: 0;
    transform: translateX(16px);
  }
}

@keyframes msg-rise {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
}

@keyframes writing-in {
  from {
    opacity: 0;
    transform: scale(0.8);
  }
}

.pending {
  margin: 0;
  min-height: 1px;
}

.thumbs {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--space-2);
  max-width: 100%;
}

.thumb {
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-m);
  background: var(--color-surface);
  overflow: hidden;
  line-height: 0;
}

.thumb:hover,
.thumb:active {
  border-color: var(--color-green);
}

.thumb img {
  display: block;
  height: 96px;
  width: auto;
  max-width: 160px;
  object-fit: cover;
}

.status-label {
  margin: 0;
  color: var(--color-muted);
  font-size: var(--text-s);
}

.notice {
  display: flex;
  gap: var(--space-3);
  align-items: flex-start;
  margin-top: var(--space-5);
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-m);
  background: var(--color-surface);
}

.notice-body {
  display: grid;
  gap: var(--space-2);
  min-width: 0;
}

.notice-title {
  margin: 0;
  font-weight: 800;
}

.notice-message,
.notice-next {
  margin: 0;
  overflow-wrap: anywhere;
}

.notice-next {
  color: var(--color-muted);
  font-size: var(--text-s);
}

.notice .button {
  justify-self: start;
}

.drop-overlay {
  position: absolute;
  inset: var(--space-2);
  display: grid;
  place-items: center;
  border: 3px dashed var(--color-green);
  border-radius: var(--radius-l);
  background: color-mix(in srgb, var(--color-bg) 90%, transparent);
  pointer-events: none;
}

.drop-overlay p {
  margin: 0;
  font-size: var(--text-l);
  font-weight: 700;
}

@media (max-width: 599px) {
  .menu-text {
    font-size: var(--text-s);
  }

  .conv-title {
    display: none;
  }
}
</style>
