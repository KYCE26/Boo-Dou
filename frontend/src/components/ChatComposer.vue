<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { uploadAttachment } from '../api/attachments'
import type { Attachment } from '../api/conversations'
import { copy } from '../copy'
import { ACCEPTED_TYPES, compressImage, MAX_IMAGES, MAX_UPLOAD_BYTES, selectImages } from '../lib/image'
import BrimEdge from './BrimEdge.vue'

const props = defineProps<{
  vision: boolean
  streaming: boolean
  online: boolean
  send: (text: string, attachments: Attachment[]) => Promise<boolean>
}>()
const emit = defineEmits<{ stop: [] }>()

interface Draft {
  key: number
  name: string
  state: 'compressing' | 'uploading' | 'done'
  progress: number
  previewUrl: string | null
  attachment: Attachment | null
  abort: (() => void) | null
}

const text = ref('')
const drafts = ref<Draft[]>([])
const errors = ref<string[]>([])
const inputEl = ref<HTMLTextAreaElement | null>(null)
const fileEl = ref<HTMLInputElement | null>(null)
let nextKey = 1

const pending = computed(() => drafts.value.some((d) => d.state !== 'done'))
const ready = computed(() => drafts.value.filter((d) => d.state === 'done' && d.attachment).map((d) => d.attachment!))
const canSend = computed(
  () => props.online && !props.streaming && !pending.value && (text.value.trim() !== '' || ready.value.length > 0),
)

function draftByKey(key: number) {
  return drafts.value.find((d) => d.key === key)
}

function dropDraft(key: number) {
  const d = draftByKey(key)
  if (!d) return
  d.abort?.()
  if (d.previewUrl) URL.revokeObjectURL(d.previewUrl)
  drafts.value = drafts.value.filter((x) => x.key !== key)
}

async function process(file: File) {
  const key = nextKey++
  drafts.value.push({ key, name: file.name || 'Gambar tempelan', state: 'compressing', progress: 0, previewUrl: null, attachment: null, abort: null })
  const name = draftByKey(key)!.name

  let blob: Blob
  try {
    blob = await compressImage(file)
  } catch {
    errors.value.push(copy.composer.compressFailed(name))
    dropDraft(key)
    return
  }
  // The user may have removed it while it was compressing.
  const draft = draftByKey(key)
  if (!draft) return
  if (blob.size > MAX_UPLOAD_BYTES) {
    errors.value.push(copy.composer.tooBigAfter(name))
    dropDraft(key)
    return
  }

  draft.previewUrl = URL.createObjectURL(blob)
  draft.state = 'uploading'
  const upload = uploadAttachment(blob, (p) => {
    const d = draftByKey(key)
    if (d) d.progress = p
  })
  draft.abort = upload.abort
  try {
    const attachment = await upload.promise
    const d = draftByKey(key)
    if (!d) return
    d.attachment = attachment
    d.state = 'done'
    d.abort = null
  } catch (err) {
    if ((err as Error).name === 'AbortError') return
    errors.value.push(copy.composer.uploadFailed(name, (err as Error).message))
    dropDraft(key)
  }
}

// Entry point for the button, paste, and drag-drop alike, so every path
// gets the same checks before anything is sent to the server.
function addFiles(files: File[]) {
  if (!props.vision || files.length === 0) return
  errors.value = []
  const { accepted, rejected } = selectImages(files, drafts.value.length)
  for (const r of rejected) errors.value.push(copy.composer.rejected(r.name, r.reason))
  for (const f of accepted) void process(f)
}

function openPicker() {
  fileEl.value?.click()
}

function onPicked(e: Event) {
  const input = e.target as HTMLInputElement
  addFiles(Array.from(input.files ?? []))
  input.value = ''
}

function onPaste(e: ClipboardEvent) {
  if (!props.vision) return
  const files = Array.from(e.clipboardData?.files ?? [])
  if (files.length === 0) return
  e.preventDefault()
  addFiles(files)
}

async function submit() {
  if (!canSend.value) return
  const message = text.value
  const attachments = ready.value
  errors.value = []
  const ok = await props.send(message, attachments)
  if (!ok) return
  text.value = ''
  for (const d of drafts.value) if (d.previewUrl) URL.revokeObjectURL(d.previewUrl)
  drafts.value = []
  await nextTick()
  resizeInput()
  inputEl.value?.focus()
}

function onKeydown(e: KeyboardEvent) {
  // Enter sends, Shift+Enter adds a line; IME composition must not send early.
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
    e.preventDefault()
    void submit()
  }
}

function resizeInput() {
  const el = inputEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 200)}px`
}

// On phones the on-screen keyboard can cover the field; bring it back into view.
function onFocus() {
  setTimeout(() => inputEl.value?.scrollIntoView({ block: 'nearest' }), 250)
}

async function removeDraft(index: number) {
  const d = drafts.value[index]
  if (!d) return
  dropDraft(d.key)
  // Keep keyboard focus inside the preview row instead of dropping to <body>.
  await nextTick()
  const buttons = document.querySelectorAll<HTMLButtonElement>('.preview-remove')
  ;(buttons[Math.min(index, buttons.length - 1)] ?? inputEl.value)?.focus()
}

async function setText(value: string) {
  text.value = value
  await nextTick()
  resizeInput()
  const el = inputEl.value
  el?.focus()
  el?.setSelectionRange(value.length, value.length)
}

onBeforeUnmount(() => {
  for (const d of drafts.value) {
    d.abort?.()
    if (d.previewUrl) URL.revokeObjectURL(d.previewUrl)
  }
})

defineExpose({ addFiles, openPicker, setText, focus: () => inputEl.value?.focus() })
</script>

<template>
  <form class="composer" @submit.prevent="submit">
    <BrimEdge variant="edge" class="composer-edge" />
    <div class="column">
      <div v-if="errors.length" class="composer-errors" role="alert">
        <p v-for="(msg, i) in errors" :key="i">{{ msg }}</p>
      </div>

      <ul v-if="drafts.length" class="previews" :aria-label="copy.composer.previewsLabel">
        <li v-for="(d, i) in drafts" :key="d.key" class="preview">
          <img v-if="d.previewUrl" :src="d.previewUrl" :alt="copy.composer.previewAlt(i + 1)" class="preview-img" />
          <span v-else class="preview-img preview-wait">{{ copy.composer.compressing }}</span>
          <progress
            v-if="d.state === 'uploading'"
            class="preview-progress"
            :value="d.progress"
            max="1"
            :aria-label="copy.composer.uploadLabel(i + 1)"
          />
          <button type="button" class="preview-remove" :aria-label="copy.composer.removeImage(i + 1)" @click="removeDraft(i)">
            <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
              <path d="M6 6l12 12M18 6 6 18" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" />
            </svg>
          </button>
        </li>
      </ul>
      <p v-if="pending" class="visually-hidden" aria-live="polite">{{ copy.composer.processing }}</p>

      <div class="composer-row">
        <template v-if="vision">
          <input
            ref="fileEl"
            type="file"
            class="visually-hidden"
            tabindex="-1"
            aria-hidden="true"
            :accept="ACCEPTED_TYPES.join(',')"
            multiple
            @change="onPicked"
          />
          <button
            type="button"
            class="icon-button"
            :aria-label="copy.composer.attach"
            :aria-describedby="drafts.length >= MAX_IMAGES ? 'attach-limit' : undefined"
            :disabled="drafts.length >= MAX_IMAGES"
            @click="openPicker"
          >
            <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
              <rect x="3" y="5" width="18" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="1.8" />
              <circle cx="9" cy="10" r="1.8" fill="currentColor" />
              <path d="m4 18 5-5 4 4 3-3 4 4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
            </svg>
          </button>
          <span v-if="drafts.length >= MAX_IMAGES" id="attach-limit" class="visually-hidden">{{ copy.composer.attachLimit(MAX_IMAGES) }}</span>
        </template>

        <label for="chat-input" class="visually-hidden">{{ copy.composer.label }}</label>
        <textarea
          id="chat-input"
          ref="inputEl"
          v-model="text"
          rows="1"
          :placeholder="copy.composer.placeholder"
          autocomplete="off"
          enterkeyhint="send"
          @input="resizeInput"
          @keydown="onKeydown"
          @paste="onPaste"
          @focus="onFocus"
        />
        <button v-if="streaming" type="button" class="button button-quiet" aria-keyshortcuts="Escape" @click="emit('stop')">
          <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
            <rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" />
          </svg>
          {{ copy.composer.stop }}
        </button>
        <button v-else type="submit" class="button button-send" :disabled="!canSend">
          <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
            <path d="M4 12 20 4l-5 16-3-7-8-1Z" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" />
          </svg>
          {{ copy.composer.send }}
        </button>
      </div>
    </div>
  </form>
</template>

<style scoped>
.composer {
  position: relative;
  padding-bottom: max(var(--space-3), env(safe-area-inset-bottom));
  background: var(--color-surface);
}

/* Identity motif: the bar's top edge follows the mascot's brim, hanging from the page. */
.composer-edge {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 100%;
  margin-bottom: -1px;
}

.column {
  width: 100%;
  max-width: var(--content-max);
  margin-inline: auto;
  padding: var(--space-2) var(--gutter) 0;
}

.composer-errors {
  margin-bottom: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-s);
  background: var(--color-bg);
  font-size: var(--text-s);
  overflow-wrap: anywhere;
}

.composer-errors p {
  margin: 0;
}

.previews {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin: 0 0 var(--space-3);
  padding: var(--space-2) 0 0;
  list-style: none;
}

.preview {
  position: relative;
  width: 64px;
  height: 64px;
}

.preview-img {
  width: 64px;
  height: 64px;
  object-fit: cover;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-s);
}

.preview-wait {
  display: grid;
  place-items: center;
  padding: var(--space-1);
  background: var(--color-bg);
  color: var(--color-muted);
  font-size: 0.7rem;
  text-align: center;
}

.preview-progress {
  position: absolute;
  left: 4px;
  right: 4px;
  bottom: 4px;
  width: calc(100% - 8px);
  height: 5px;
  accent-color: var(--color-green);
}

/* 44px hit area around a small badge on the corner. */
.preview-remove {
  position: absolute;
  top: -14px;
  right: -14px;
  display: grid;
  place-items: center;
  width: var(--tap-min);
  height: var(--tap-min);
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--color-ink);
}

.preview-remove svg {
  box-sizing: content-box;
  padding: 4px;
  border: 1px solid var(--color-border);
  border-radius: 50%;
  background: var(--color-surface);
}

.preview-remove:hover svg,
.preview-remove:active svg {
  background: var(--color-tea);
}

.preview-remove:focus-visible {
  outline-offset: -8px;
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
  color: var(--color-muted);
  opacity: 1;
}

@media (max-width: 380px) {
  .button-send {
    padding: 0 var(--space-3);
  }
}
</style>
