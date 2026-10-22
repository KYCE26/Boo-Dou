<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ open: boolean; labelledby: string; closeOnBackdrop?: boolean }>(), {
  closeOnBackdrop: true,
})
const emit = defineEmits<{ close: [] }>()

const el = ref<HTMLDialogElement | null>(null)

// showModal() makes the rest of the page inert, which is what traps focus and
// gives Escape-to-close for free, and the browser restores focus on close.
watch(
  () => props.open,
  async (open) => {
    await nextTick()
    const dialog = el.value
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    else if (!open && dialog.open) dialog.close()
  },
  { immediate: true },
)

function onBackdropClick(e: MouseEvent) {
  // The panel fills the dialog, so a click whose target is the dialog itself
  // landed on the backdrop.
  if (props.closeOnBackdrop && e.target === el.value) el.value?.close()
}

onBeforeUnmount(() => {
  if (el.value?.open) el.value.close()
})

defineExpose({ el })
</script>

<template>
  <dialog ref="el" class="modal" :aria-labelledby="labelledby" @close="emit('close')" @click="onBackdropClick">
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
</style>
