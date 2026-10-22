<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { renderMarkdown } from '../lib/markdown'

const props = defineProps<{ source: string }>()

const COPY_FEEDBACK_MS = 2000

// renderMarkdown always runs DOMPurify; nothing else may feed v-html here.
const html = ref(renderMarkdown(props.source))
const announcement = ref('')

// Tokens can arrive many times per frame; re-parsing more often than the
// screen can paint only burns CPU during long answers.
let frame = 0
watch(
  () => props.source,
  () => {
    if (frame) return
    frame = requestAnimationFrame(() => {
      frame = 0
      html.value = renderMarkdown(props.source)
    })
  },
)

const resetTimers = new WeakMap<HTMLButtonElement, ReturnType<typeof setTimeout>>()

async function onClick(event: MouseEvent) {
  const button = (event.target as HTMLElement).closest<HTMLButtonElement>('button[data-copy]')
  if (!button) return
  const code = button.closest('.code-block')?.querySelector('pre code')?.textContent ?? ''

  let copied = true
  try {
    // navigator.clipboard is undefined outside secure contexts, which the
    // try also covers.
    await navigator.clipboard.writeText(code)
  } catch {
    copied = false
  }

  button.textContent = copied ? 'Tersalin' : 'Gagal menyalin'
  announcement.value = copied ? 'Kode tersalin' : 'Gagal menyalin kode'
  clearTimeout(resetTimers.get(button))
  resetTimers.set(
    button,
    setTimeout(() => {
      button.textContent = 'Salin'
      announcement.value = ''
    }, COPY_FEEDBACK_MS),
  )
}

onBeforeUnmount(() => cancelAnimationFrame(frame))
</script>

<template>
  <div class="markdown-root">
    <div class="markdown" v-html="html" @click="onClick" />
    <span class="visually-hidden" aria-live="polite">{{ announcement }}</span>
  </div>
</template>

<style scoped>
.markdown-root {
  min-width: 0;
}

.markdown {
  overflow-wrap: anywhere;
}

.markdown :deep(> :first-child) {
  margin-top: 0;
}

.markdown :deep(> :last-child) {
  margin-bottom: 0;
}

.markdown :deep(p),
.markdown :deep(ul),
.markdown :deep(ol),
.markdown :deep(blockquote),
.markdown :deep(.code-block),
.markdown :deep(.table-wrap) {
  margin: 0 0 var(--space-3);
}

.markdown :deep(h1),
.markdown :deep(h2),
.markdown :deep(h3),
.markdown :deep(h4),
.markdown :deep(h5),
.markdown :deep(h6) {
  margin: var(--space-4) 0 var(--space-2);
  line-height: 1.3;
}

.markdown :deep(h1) {
  font-size: 1.3rem;
}

.markdown :deep(h2) {
  font-size: 1.175rem;
}

.markdown :deep(h3),
.markdown :deep(h4),
.markdown :deep(h5),
.markdown :deep(h6) {
  font-size: 1rem;
}

.markdown :deep(ul),
.markdown :deep(ol) {
  padding-left: 1.4em;
}

/* Explicit, because the message list is itself an <ol> and browsers would
   otherwise style these as nested lists (hollow circles). */
.markdown :deep(ul) {
  list-style: disc;
}

.markdown :deep(ol) {
  list-style: decimal;
}

.markdown :deep(li + li) {
  margin-top: var(--space-1);
}

.markdown :deep(blockquote) {
  padding-left: var(--space-3);
  border-left: 3px solid var(--color-green);
  color: var(--color-muted);
}

.markdown :deep(a) {
  color: var(--color-green);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.markdown :deep(hr) {
  border: 0;
  border-top: 1px solid var(--color-border);
  margin: var(--space-4) 0;
}

.markdown :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  /* Cascadia draws != as a single ≠ glyph; code must show what gets copied. */
  font-variant-ligatures: none;
  font-feature-settings: "liga" 0, "calt" 0;
}

.markdown :deep(:not(pre) > code) {
  padding: 0.1em 0.35em;
  border-radius: 4px;
  background: var(--color-tea);
}

.markdown :deep(.code-block) {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-s);
  background: var(--color-surface);
  overflow: hidden;
}

.markdown :deep(.code-head) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding-left: var(--space-3);
  border-bottom: 1px solid var(--color-border);
  background: var(--color-bg);
}

.markdown :deep(.code-lang) {
  color: var(--color-muted);
  font-size: 0.85rem;
}

.markdown :deep(.copy-btn) {
  min-width: var(--tap-min);
  min-height: var(--tap-min);
  padding: 0 var(--space-3);
  border: 0;
  border-left: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-ink);
  font-size: 0.875rem;
  font-weight: 600;
}

.markdown :deep(.copy-btn:hover),
.markdown :deep(.copy-btn:active) {
  background: var(--color-tea);
}

.markdown :deep(.copy-btn:focus-visible) {
  outline-offset: -3px;
}

/* Wide code scrolls inside its own box so the page never scrolls sideways. */
.markdown :deep(pre) {
  margin: 0;
  padding: var(--space-3);
  overflow-x: auto;
  overflow-wrap: normal;
  line-height: 1.5;
}

.markdown :deep(pre:focus-visible),
.markdown :deep(.table-wrap:focus-visible) {
  outline-offset: -3px;
}

.markdown :deep(pre code) {
  white-space: pre;
  /* Go is tab-indented; the default 8 wastes most of a phone screen. */
  tab-size: 4;
}

.markdown :deep(.table-wrap) {
  max-width: 100%;
  overflow-x: auto;
}

.markdown :deep(table) {
  border-collapse: collapse;
  min-width: 100%;
  font-size: 0.95rem;
  overflow-wrap: normal;
}

.markdown :deep(th),
.markdown :deep(td) {
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}

.markdown :deep(th) {
  background: var(--color-bg);
  font-weight: 700;
}
</style>
