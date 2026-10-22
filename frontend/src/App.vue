<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import ConversationSidebar from './components/ConversationSidebar.vue'
import ModalDialog from './components/ModalDialog.vue'
import { useConversationsStore } from './stores/conversations'
import { useUiStore } from './stores/ui'

const ui = useUiStore()
const conversations = useConversationsStore()

onMounted(() => {
  void ui.loadHealth()
  void conversations.load()
})
</script>

<template>
  <div class="layout" :class="{ desktop: ui.desktop }">
    <!-- Exactly one sidebar instance exists at a time, so ids stay unique. -->
    <aside v-if="ui.desktop" class="sidebar">
      <ConversationSidebar />
    </aside>
    <ModalDialog
      v-else
      id="conversation-drawer"
      class="drawer"
      :open="ui.drawerOpen"
      labelledby="drawer-title"
      @close="ui.drawerOpen = false"
    >
      <div class="drawer-head">
        <h2 id="drawer-title" class="drawer-title">Percakapan</h2>
        <button type="button" class="icon-button" aria-label="Tutup daftar percakapan" @click="ui.drawerOpen = false">
          <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
            <path d="M6 6l12 12M18 6 6 18" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
          </svg>
        </button>
      </div>
      <ConversationSidebar @navigate="ui.drawerOpen = false" />
    </ModalDialog>

    <RouterView />
  </div>
</template>

<style scoped>
.layout.desktop {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
}

.sidebar {
  height: 100dvh;
  padding: var(--space-3);
  border-right: 1px solid var(--color-surface-muted);
  background: var(--color-surface);
  overflow: hidden;
}

/* The drawer is a modal <dialog> pinned to the left edge. */
.drawer {
  margin: 0;
  width: min(320px, 86vw);
  max-width: none;
  height: 100dvh;
  max-height: none;
  border: 0;
  border-right: 1px solid var(--color-border);
  border-radius: 0;
}

.drawer :deep(.modal-panel) {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: var(--space-3);
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
}

.drawer-title {
  margin: 0;
  font-size: 1.125rem;
}
</style>
