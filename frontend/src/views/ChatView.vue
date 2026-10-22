<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import ChatComposer from '../components/ChatComposer.vue'
import GhostMascot from '../components/GhostMascot.vue'
import ImageViewer from '../components/ImageViewer.vue'
import MarkdownContent from '../components/MarkdownContent.vue'
import { attachmentUrl, type Attachment } from '../api/conversations'
import { useChatStore } from '../stores/chat'
import { useUiStore } from '../stores/ui'
import { useTheme } from '../composables/theme'

const chat = useChatStore()
const ui = useUiStore()
const route = useRoute()
const router = useRouter()
const { messages, error, isStreaming, isEmpty, mascotState, status, loadState, loadError } = storeToRefs(chat)
const { theme, toggle: toggleTheme } = useTheme()

const listEl = ref<HTMLElement | null>(null)
const composer = ref<InstanceType<typeof ChatComposer> | null>(null)
const viewing = ref<{ image: Attachment; label: string } | null>(null)
const dragging = ref(false)
let dragDepth = 0

const statusText: Record<string, string> = {
  thinking: 'Boo Dou sedang menyusun jawaban',
  error: 'Jawaban gagal dimuat',
  happy: 'Jawaban selesai',
  idle: 'Siap menerima pertanyaan',
}

const routeId = computed(() => (typeof route.params.id === 'string' ? route.params.id : null))

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

function openViewer(image: Attachment, index: number, total: number) {
  viewing.value = { image, label: total > 1 ? `Gambar ${index + 1} dari ${total}` : 'Gambar lampiran' }
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
      <div class="brand">
        <button
          v-if="!ui.desktop"
          type="button"
          class="icon-button plain"
          aria-label="Buka daftar percakapan"
          aria-controls="conversation-drawer"
          :aria-expanded="ui.drawerOpen"
          @click="ui.drawerOpen = true"
        >
          <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
            <path d="M4 7h16M4 12h16M4 17h16" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
          </svg>
        </button>
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
          <path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" />
        </svg>
      </button>
    </header>

    <main ref="listEl" class="conversation">
      <div class="column">
        <section v-if="loadState === 'loading'" class="state">
          <GhostMascot state="thinking" :size="96" />
          <p>Memuat percakapan</p>
        </section>

        <section v-else-if="loadState === 'notfound'" class="state" role="alert">
          <GhostMascot state="error" :size="96" />
          <h2>Percakapan tidak ditemukan</h2>
          <p>Mungkin sudah dihapus.</p>
          <RouterLink to="/" class="button button-primary">Mulai percakapan baru</RouterLink>
        </section>

        <section v-else-if="loadState === 'error'" class="state" role="alert">
          <GhostMascot state="error" :size="96" />
          <h2>Percakapan gagal dimuat</h2>
          <p>{{ loadError }}</p>
          <button v-if="routeId" type="button" class="button button-secondary" @click="chat.open(routeId)">Muat ulang percakapan</button>
        </section>

        <section v-else-if="isEmpty" class="state">
          <GhostMascot :state="mascotState" :size="140" />
          <h2>Halo, saya Boo Dou</h2>
          <p v-if="ui.vision">Tulis pertanyaan atau lampirkan gambar, lalu tekan Kirim atau Enter.</p>
          <p v-else>Tulis pertanyaan di kolom bawah, lalu tekan Kirim atau Enter.</p>
        </section>

        <ol v-else class="messages" aria-label="Percakapan">
          <li
            v-for="msg in messages"
            :key="msg.localId"
            class="message"
            :class="msg.role === 'user' ? 'from-user' : 'from-assistant'"
          >
            <span class="visually-hidden">{{ msg.role === 'user' ? 'Anda:' : 'Boo Dou:' }}</span>
            <div v-if="msg.attachments.length" class="thumbs">
              <button
                v-for="(a, i) in msg.attachments"
                :key="a.id"
                type="button"
                class="thumb"
                :aria-label="`Lihat gambar ${i + 1} ukuran penuh`"
                @click="openViewer(a, i, msg.attachments.length)"
              >
                <img :src="attachmentUrl(a.id)" :width="thumbWidth(a)" :height="THUMB_HEIGHT" alt="" loading="lazy" />
              </button>
            </div>
            <p v-if="msg.role === 'user' && msg.content" class="bubble">{{ msg.content }}</p>
            <div v-else-if="msg.role === 'assistant' && msg.content" class="bubble markdown-bubble">
              <MarkdownContent :source="msg.content" />
            </div>
            <p v-else-if="msg.role === 'assistant'" class="bubble pending">
              <span class="dot" /><span class="dot" /><span class="dot" />
              <span class="visually-hidden">Menunggu jawaban</span>
            </p>
            <p v-if="msg.status === 'stopped'" class="status-label">Dihentikan</p>
            <p v-else-if="msg.role === 'assistant' && msg.status === 'error'" class="status-label">Terputus karena error</p>
          </li>
        </ol>

        <div v-if="status === 'error'" class="error-card" role="alert">
          <GhostMascot state="error" :size="56" />
          <div class="error-body">
            <p class="error-title">Jawaban gagal dimuat</p>
            <p class="error-message">{{ error }}</p>
            <button v-if="chat.conversationId" type="button" class="button button-secondary" @click="chat.retry()">
              Kirim ulang pesan terakhir
            </button>
          </div>
        </div>
      </div>
    </main>

    <p class="visually-hidden" aria-live="polite">{{ statusText[mascotState] }}</p>

    <ChatComposer
      ref="composer"
      :vision="ui.vision"
      :streaming="isStreaming"
      :send="sendMessage"
      :key="routeId ?? 'new'"
      @stop="chat.stop()"
    />

    <div v-if="dragging" class="drop-overlay" aria-hidden="true">
      <p>Lepaskan gambar untuk melampirkan</p>
    </div>

    <ImageViewer :image="viewing?.image ?? null" :label="viewing?.label ?? ''" @close="viewing = null" />
  </div>
</template>

<style scoped>
.page {
  position: relative;
  display: grid;
  grid-template-rows: auto 1fr auto;
  height: 100dvh;
  min-width: 0;
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

.conversation {
  overflow-y: auto;
  overflow-x: hidden;
  padding-block: var(--space-5);
}

.state {
  display: grid;
  justify-items: center;
  gap: var(--space-2);
  padding-block: var(--space-6);
  text-align: center;
}

.state h2 {
  margin: var(--space-3) 0 0;
  font-size: 1.5rem;
}

.state p {
  margin: 0;
  max-width: 36ch;
  color: var(--color-text-muted);
  overflow-wrap: anywhere;
}

.state .button {
  margin-top: var(--space-2);
  text-decoration: none;
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
  gap: var(--space-1);
  min-width: 0;
}

.from-user {
  align-items: flex-end;
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
  background: var(--color-surface-muted);
  overflow: hidden;
  line-height: 0;
}

.thumb img {
  display: block;
  height: 96px;
  width: auto;
  max-width: 160px;
  object-fit: cover;
}

.status-label {
  margin: 0 0 0 var(--space-2);
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
  background: var(--color-tea);
  color: var(--color-ink);
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
  white-space: normal;
}

.drop-overlay {
  position: absolute;
  inset: var(--space-2);
  display: grid;
  place-items: center;
  border: 3px dashed var(--color-green);
  border-radius: var(--radius-l);
  background: color-mix(in srgb, var(--color-bg) 88%, transparent);
  pointer-events: none;
}

.drop-overlay p {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 700;
}
</style>
