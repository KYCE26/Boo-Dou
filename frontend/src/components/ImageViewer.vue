<script setup lang="ts">
import { attachmentUrl, type Attachment } from '../api/conversations'
import ModalDialog from './ModalDialog.vue'

defineProps<{ image: Attachment | null; label: string }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <ModalDialog :open="image !== null" labelledby="viewer-title" class="viewer" @close="emit('close')">
    <div class="viewer-head">
      <h2 id="viewer-title" class="viewer-title">{{ label }}</h2>
      <button type="button" class="icon-button" aria-label="Tutup gambar" @click="emit('close')">
        <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
          <path d="M6 6l12 12M18 6 6 18" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
        </svg>
      </button>
    </div>
    <img v-if="image" :src="attachmentUrl(image.id)" :width="image.width" :height="image.height" :alt="label" class="viewer-img" />
  </ModalDialog>
</template>

<style scoped>
.viewer :deep(.modal-panel) {
  padding: var(--space-3);
}

.viewer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin-bottom: var(--space-2);
}

.viewer-title {
  margin: 0;
  font-size: 1rem;
}

.viewer-img {
  display: block;
  width: 100%;
  height: auto;
  max-height: 75dvh;
  object-fit: contain;
  border-radius: var(--radius-s);
}
</style>
