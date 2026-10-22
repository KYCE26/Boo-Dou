<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import GhostMascot from '../components/GhostMascot.vue'
import MarkdownContent from '../components/MarkdownContent.vue'
import { useChatStore } from '../stores/chat'
import { useTheme } from '../composables/theme'

const chat = useChatStore()
const { messages, error, isStreaming, isEmpty, mascotState, status } = storeToRefs(chat)
const { theme, toggle: toggleTheme } = useTheme()

const draft = ref('')
const listEl = ref<HTMLElement | null>(null)
const inputEl = ref<HTMLTextAreaElement | null>(null)

const statusText: Record<string, string> = {
  thinking: 'Boo Dou sedang menyusun jawaban',
  error: 'Jawaban gagal dimuat',
  happy: 'Jawaban selesai',
  idle: 'Siap menerima pertanyaan',
}

async function submit() {
  const text = draft.value
  if (!text.trim() || isStreaming.value) return
  draft.value = ''
  // v-model writes the empty value on the next tick; measuring earlier would
  // keep the old height.
  await nextTick()
  resizeInput()
  await chat.send(text)
  inputEl.value?.focus()
}

function onKeydown(e: KeyboardEvent) {
  // Enter sends, Shift+Enter adds a line; IME composition must not send early.
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault()
    submit()
  }
}

// Escape stops a running answer from anywhere on the page, matching the
// Hentikan button for keyboard users.
function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isStreaming.value) {
    e.preventDefault()
    chat.stop()
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKeydown))

function resizeInput() {
  const el = inputEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 200)}px`
}

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
</script>

<template>
  <div class="page">
    <header class="topbar">
      <div class="brand">
        <GhostMascot :state="mascotState" :size="40" />
        <h1 class="brand-name">Boo Dou</h1>
      </div>
      <button
        type="button"
        class="icon-button"
        :aria-label="theme === 'dark' ? 'Ganti ke tema terang' : 'Ganti ke tema gelap'"
        :aria-pressed="theme === 'dark'"
        @click="toggleTheme"
      >
        <svg v-if="theme === 'dark'" viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
          <circle cx="12" cy="12" r="4.5" fill="none" stroke="currentColor" stroke-width="2" />
          <path
            d="M12 2v2.5M12 19.5V22M2 12h2.5M19.5 12H22M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M4.9 19.1l1.8-1.8M17.3 6.7l1.8-1.8"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
          />
        </svg>
        <svg v-else viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
          <path
            d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </header>

    <main ref="listEl" class="conversation">
      <div class="column">
        <section v-if="isEmpty" class="empty">
          <GhostMascot :state="mascotState" :size="140" />
          <h2>Halo, saya Boo Dou</h2>
          <p>Tulis pertanyaan di kolom bawah, lalu tekan Kirim atau Enter.</p>
        </section>

        <ol v-else class="messages" aria-label="Percakapan">
          <li
            v-for="msg in messages"
            :key="msg.id"
            class="message"
            :class="msg.role === 'user' ? 'from-user' : 'from-assistant'"
          >
            <span class="visually-hidden">{{ msg.role === 'user' ? 'Anda:' : 'Boo Dou:' }}</span>
            <p v-if="msg.role === 'user'" class="bubble">{{ msg.content }}</p>
            <div v-else-if="msg.content" class="bubble markdown-bubble">
              <MarkdownContent :source="msg.content" />
            </div>
            <p v-else class="bubble pending">
              <span class="dot" /><span class="dot" /><span class="dot" />
              <span class="visually-hidden">Menunggu jawaban</span>
            </p>
            <p v-if="msg.stopped" class="stopped-label">Dihentikan</p>
          </li>
        </ol>

        <div v-if="status === 'error'" class="error-card" role="alert">
          <GhostMascot state="error" :size="56" />
          <div class="error-body">
            <p class="error-title">Jawaban gagal dimuat</p>
            <p class="error-message">{{ error }}</p>
            <button type="button" class="button button-secondary" @click="chat.retry()">Kirim ulang pesan terakhir</button>
          </div>
        </div>
      </div>
    </main>

    <p class="visually-hidden" aria-live="polite">{{ statusText[mascotState] }}</p>

    <form class="composer" @submit.prevent="submit">
      <div class="column composer-row">
        <label for="chat-input" class="visually-hidden">Pesan untuk Boo Dou</label>
        <textarea
          id="chat-input"
          ref="inputEl"
          v-model="draft"
          rows="1"
          placeholder="Tulis pesan"
          autocomplete="off"
          @input="resizeInput"
          @keydown="onKeydown"
        />
        <button v-if="isStreaming" type="button" class="button button-secondary" aria-keyshortcuts="Escape" @click="chat.stop()">
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
            <rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" />
          </svg>
          Hentikan
        </button>
        <button v-else type="submit" class="button button-primary" :disabled="!draft.trim()">
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
            <path d="M4 12 20 4l-5 16-3-7-8-1Z" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" />
          </svg>
          Kirim
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.page {
  display: grid;
  grid-template-rows: auto 1fr auto;
  height: 100dvh;
}

.column {
  width: 100%;
  max-width: var(--content-max);
  margin-inline: auto;
  padding-inline: var(--space-4);
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-4);
  border-bottom: 1px solid var(--color-surface-muted);
  background: var(--color-surface);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}

.brand-name {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 700;
}

.icon-button {
  display: inline-grid;
  place-items: center;
  width: var(--tap-min);
  height: var(--tap-min);
  border: 1px solid var(--color-border);
  border-radius: 50%;
  background: transparent;
  color: var(--color-text);
}

.icon-button:hover {
  background: var(--color-surface-muted);
}

.conversation {
  overflow-y: auto;
  overflow-x: hidden;
  padding-block: var(--space-5);
}

.empty {
  display: grid;
  justify-items: center;
  gap: var(--space-2);
  padding-block: var(--space-6);
  text-align: center;
}

.empty h2 {
  margin: var(--space-3) 0 0;
  font-size: 1.5rem;
}

.empty p {
  margin: 0;
  max-width: 34ch;
  color: var(--color-text-muted);
}

.messages {
  display: grid;
  gap: var(--space-3);
  margin: 0;
  padding: 0;
  list-style: none;
}

.message {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
}

.from-user {
  align-items: flex-end;
}

.stopped-label {
  margin: var(--space-1) 0 0 var(--space-2);
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.bubble {
  max-width: min(85%, 60ch);
  margin: 0;
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-l);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.from-user .bubble {
  background: var(--color-accent);
  color: var(--color-on-accent);
  border-bottom-right-radius: var(--radius-s);
}

.from-assistant .bubble {
  background: var(--color-surface-muted);
  border-bottom-left-radius: var(--radius-s);
}

/* Markdown handles its own line breaks, and code or tables need the full
   width; min-width: 0 lets their inner scroll boxes shrink inside flex. */
.markdown-bubble {
  max-width: 100%;
  min-width: 0;
  white-space: normal;
}

.pending {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  min-height: 48px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-text-muted);
  animation: blink 1.2s infinite ease-in-out;
}

.dot:nth-child(2) {
  animation-delay: 0.15s;
}

.dot:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes blink {
  0%,
  80%,
  100% {
    opacity: 0.3;
  }
  40% {
    opacity: 1;
  }
}

.error-card {
  display: flex;
  gap: var(--space-3);
  align-items: flex-start;
  margin-top: var(--space-4);
  padding: var(--space-4);
  border: 1px solid var(--color-danger);
  border-radius: var(--radius-m);
  background: var(--color-danger-surface);
}

.error-body {
  display: grid;
  gap: var(--space-2);
  min-width: 0;
}

.error-title {
  margin: 0;
  font-weight: 700;
  color: var(--color-danger);
}

.error-message {
  margin: 0;
  overflow-wrap: anywhere;
}

.error-body .button {
  justify-self: start;
}

.composer {
  padding-block: var(--space-3);
  padding-bottom: max(var(--space-3), env(safe-area-inset-bottom));
  border-top: 1px solid var(--color-surface-muted);
  background: var(--color-surface);
}

.composer-row {
  display: flex;
  gap: var(--space-2);
  align-items: flex-end;
}

textarea {
  flex: 1;
  min-width: 0;
  min-height: var(--tap-min);
  max-height: 200px;
  padding: 10px var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-m);
  background: var(--color-bg);
  resize: none;
  line-height: 1.5;
}

textarea::placeholder {
  color: var(--color-text-muted);
  opacity: 1;
}

.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  min-width: var(--tap-min);
  min-height: var(--tap-min);
  padding: 0 var(--space-4);
  border-radius: var(--radius-m);
  font-weight: 600;
  white-space: nowrap;
}

.button-primary {
  border: 1px solid var(--color-accent);
  background: var(--color-accent);
  color: var(--color-on-accent);
}

.button-primary:hover:not(:disabled) {
  background: var(--color-accent-hover);
  border-color: var(--color-accent-hover);
}

.button-primary:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.button-secondary {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text);
}

.button-secondary:hover {
  background: var(--color-surface-muted);
}
</style>
