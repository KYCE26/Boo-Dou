<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { attachmentUrl, type Attachment } from '../api/conversations'
import { copy } from '../copy'
import { prefersReducedMotion } from '../lib/motion'
import ModalDialog from './ModalDialog.vue'

const props = defineProps<{ image: Attachment | null; label: string; origin: HTMLElement | null }>()
const emit = defineEmits<{ close: [] }>()

// Kept apart from the prop so the image can fly back after the parent clears it.
const shown = ref<Attachment | null>(null)
const shownLabel = ref('')
const imgEl = ref<HTMLImageElement | null>(null)
let origin: HTMLElement | null = null

const EASE = 'cubic-bezier(0.2, 0.7, 0.2, 1)'

// FLIP: the full-size image starts where the thumbnail sits, so the eye can
// follow which picture opened.
function flipTransform(from: DOMRect, to: DOMRect) {
  const sx = from.width / to.width
  const sy = from.height / to.height
  return `translate(${from.left - to.left}px, ${from.top - to.top}px) scale(${sx}, ${sy})`
}

// The dialog opens a tick after the image mounts; wait until it has a real box.
async function laidOut(img: HTMLImageElement) {
  for (let i = 0; i < 6 && img.getBoundingClientRect().width === 0; i++) {
    await new Promise((r) => requestAnimationFrame(r))
  }
  return img.getBoundingClientRect()
}

async function flyIn() {
  await nextTick()
  const img = imgEl.value
  if (!img || !origin || prefersReducedMotion()) return
  const to = await laidOut(img)
  if (to.width === 0) return
  img.animate([{ transform: flipTransform(origin.getBoundingClientRect(), to) }, { transform: 'none' }], {
    duration: 300,
    easing: EASE,
  })
}

async function flyOut() {
  const img = imgEl.value
  if (!img || !origin?.isConnected || prefersReducedMotion()) return
  const to = img.getBoundingClientRect()
  await img
    .animate([{ transform: 'none' }, { transform: flipTransform(origin.getBoundingClientRect(), to) }], {
      duration: 220,
      easing: 'cubic-bezier(.4, 0, 1, 1)',
      fill: 'forwards',
    })
    .finished.catch(() => {})
}

watch(
  () => props.image,
  async (image) => {
    if (image) {
      origin = props.origin
      shown.value = image
      shownLabel.value = props.label
      await flyIn()
    } else if (shown.value) {
      await flyOut()
      shown.value = null
    }
  },
)
</script>

<template>
  <ModalDialog :open="shown !== null" labelledby="viewer-title" class="viewer" motion="fade" @close="emit('close')">
    <div class="viewer-head">
      <h2 id="viewer-title" class="viewer-title">{{ shownLabel }}</h2>
      <button type="button" class="icon-button bare" :aria-label="copy.viewer.close" @click="emit('close')">
        <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
          <path d="M6 6l12 12M18 6 6 18" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
        </svg>
      </button>
    </div>
    <img
      v-if="shown"
      ref="imgEl"
      :src="attachmentUrl(shown.id)"
      :width="shown.width"
      :height="shown.height"
      :alt="shownLabel"
      class="viewer-img"
    />
  </ModalDialog>
</template>

<style scoped>
.viewer {
  max-width: min(60rem, calc(100vw - 2 * var(--gutter)));
}

.viewer :deep(.modal-panel) {
  padding: var(--space-3);
}

.viewer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
  padding-left: var(--space-2);
}

.viewer-title {
  margin: 0;
  font-size: var(--text-m);
  font-weight: 700;
}

/* The box matches the picture exactly, so the FLIP scale does not stretch it. */
.viewer-img {
  display: block;
  width: auto;
  height: auto;
  max-width: 100%;
  max-height: 78dvh;
  margin-inline: auto;
  border-radius: var(--radius-s);
  background: var(--color-bg);
  transform-origin: top left;
}
</style>
