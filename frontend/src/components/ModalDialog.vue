<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { prefersReducedMotion } from '../lib/motion'

type Motion = 'pop' | 'drawer' | 'fade'

const props = withDefaults(
  defineProps<{ open: boolean; labelledby: string; closeOnBackdrop?: boolean; motion?: Motion }>(),
  { closeOnBackdrop: true, motion: 'pop' },
)
const emit = defineEmits<{ close: [] }>()

const el = ref<HTMLDialogElement | null>(null)

const OUT: Record<Motion, Keyframe[]> = {
  pop: [
    { opacity: 1, transform: 'scale(1)' },
    { opacity: 0, transform: 'scale(0.97)' },
  ],
  drawer: [{ transform: 'translateX(0)' }, { transform: 'translateX(-100%)' }],
  fade: [{ opacity: 1 }, { opacity: 0 }],
}

// Play the exit before close(): a native dialog disappears instantly otherwise.
async function animateClose(dialog: HTMLDialogElement) {
  if (!prefersReducedMotion()) {
    const timing = { duration: 160, easing: 'cubic-bezier(.4, 0, 1, 1)', fill: 'forwards' as const }
    const backdrop = dialog.animate([{ opacity: 1 }, { opacity: 0 }], { ...timing, pseudoElement: '::backdrop' })
    await dialog.animate(OUT[props.motion], timing).finished.catch(() => {})
    backdrop.cancel()
  }
  dialog.close()
  dialog.getAnimations().forEach((a) => a.cancel())
}

// showModal() makes the rest of the page inert, which is what traps focus and
// gives Escape-to-close, and the browser restores focus on close.
watch(
  () => props.open,
  async (open) => {
    await nextTick()
    const dialog = el.value
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    else if (!open && dialog.open) await animateClose(dialog)
  },
  { immediate: true },
)

// Route Escape through the parent so the exit animation runs; if the browser
// refuses to cancel, the dialog simply closes and 'close' still fires.
function onCancel(e: Event) {
  e.preventDefault()
  emit('close')
}

function onBackdropClick(e: MouseEvent) {
  // The panel fills the dialog, so a click whose target is the dialog itself
  // landed on the backdrop.
  if (props.closeOnBackdrop && e.target === el.value) emit('close')
}

function onClose() {
  if (props.open) emit('close')
}

onBeforeUnmount(() => {
  if (el.value?.open) el.value.close()
})

defineExpose({ el })
</script>

<template>
  <dialog
    ref="el"
    class="modal"
    :class="`motion-${motion}`"
    :aria-labelledby="labelledby"
    @cancel="onCancel"
    @close="onClose"
    @click="onBackdropClick"
  >
    <div class="modal-panel">
      <slot />
    </div>
  </dialog>
</template>

<style scoped>
.modal {
  max-width: min(30rem, calc(100vw - 2 * var(--gutter)));
  width: 100%;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-m);
  background: var(--color-surface);
  color: var(--color-ink);
  /* The only shadow in the app: a dialog sits above the page and must read as lifted. */
  box-shadow: 0 18px 48px rgb(0 0 0 / 0.22);
}

.modal::backdrop {
  background: var(--color-scrim);
}

.modal-panel {
  padding: var(--space-5);
}

/* Entrances: a dialog settles into place, the drawer slides from the side it
   lives on, and the viewer fades so its image can fly in from the thumbnail. */
.modal[open] {
  animation: 200ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.modal[open]::backdrop {
  animation: modal-fade 200ms ease-out both;
}

.motion-pop[open] {
  animation-name: modal-pop;
}

.motion-drawer[open] {
  animation-name: modal-drawer;
  animation-duration: 240ms;
}

.motion-fade[open] {
  animation-name: modal-fade;
}

@keyframes modal-pop {
  from {
    opacity: 0;
    transform: scale(0.97) translateY(6px);
  }
}

@keyframes modal-drawer {
  from {
    transform: translateX(-100%);
  }
}

@keyframes modal-fade {
  from {
    opacity: 0;
  }
}
</style>
