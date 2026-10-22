<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import type { Conversation } from '../api/conversations'
import { useChatStore } from '../stores/chat'
import { useConversationsStore } from '../stores/conversations'
import ModalDialog from './ModalDialog.vue'

const emit = defineEmits<{ navigate: [] }>()

const conversations = useConversationsStore()
const chat = useChatStore()
const router = useRouter()

const UNTITLED = 'Percakapan tanpa judul'
const label = (c: Conversation) => c.title || UNTITLED

const editingId = ref<string | null>(null)
const editTitle = ref('')
const renameError = ref<string | null>(null)
const renameInput = ref<HTMLInputElement[]>([])
const listEl = ref<HTMLElement | null>(null)
let settling = false

const pendingDelete = ref<Conversation | null>(null)
const deleteError = ref<string | null>(null)
const deleting = ref(false)

const activeId = computed(() => chat.conversationId)

function newConversation() {
  emit('navigate')
  void router.push('/')
}

async function startRename(c: Conversation) {
  editingId.value = c.id
  editTitle.value = c.title
  renameError.value = null
  await nextTick()
  renameInput.value[0]?.focus()
  renameInput.value[0]?.select()
}

async function finishRename(save: boolean) {
  // Escape removes the input, which can fire blur; settling stops that blur
  // from saving what the user just cancelled.
  if (settling || !editingId.value) return
  settling = true
  const id = editingId.value
  const title = editTitle.value.trim()
  const original = conversations.items.find((c) => c.id === id)?.title ?? ''
  try {
    if (save && title && title !== original) {
      await conversations.rename(id, title)
    }
    editingId.value = null
    await nextTick()
    // Return focus to where the keyboard user started.
    listEl.value?.querySelector<HTMLButtonElement>(`[data-rename="${id}"]`)?.focus()
  } catch (err) {
    renameError.value = (err as Error).message
  } finally {
    settling = false
  }
}

function askDelete(c: Conversation) {
  deleteError.value = null
  pendingDelete.value = c
}

async function confirmDelete() {
  const target = pendingDelete.value
  if (!target) return
  deleting.value = true
  deleteError.value = null
  try {
    await conversations.remove(target.id)
    pendingDelete.value = null
    if (target.id === chat.conversationId) {
      emit('navigate')
      await router.push('/')
    }
  } catch (err) {
    deleteError.value = (err as Error).message
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <nav class="sidebar-nav" aria-label="Daftar percakapan">
    <button type="button" class="button button-primary new-button" @click="newConversation">
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
        <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
      </svg>
      Percakapan baru
    </button>

    <p v-if="conversations.state === 'loading' || conversations.state === 'idle'" class="list-note">Memuat percakapan</p>
    <div v-else-if="conversations.state === 'error'" class="list-note list-error" role="alert">
      <p>{{ conversations.error }}</p>
      <button type="button" class="button button-secondary" @click="conversations.load()">Muat ulang daftar</button>
    </div>
    <p v-else-if="conversations.items.length === 0" class="list-note">Belum ada percakapan. Pesan pertamamu akan tersimpan di sini.</p>

    <ul v-else ref="listEl" class="conv-list">
      <li v-for="c in conversations.items" :key="c.id" class="conv-item" :class="{ active: c.id === activeId }">
        <template v-if="editingId === c.id">
          <label :for="`rename-${c.id}`" class="visually-hidden">Judul percakapan</label>
          <input
            :id="`rename-${c.id}`"
            ref="renameInput"
            v-model="editTitle"
            class="rename-input"
            maxlength="120"
            :aria-describedby="renameError ? `rename-error-${c.id}` : undefined"
            @keydown.enter.prevent="finishRename(true)"
            @keydown.esc.prevent.stop="finishRename(false)"
            @blur="finishRename(true)"
          />
          <p v-if="renameError" :id="`rename-error-${c.id}`" class="rename-error" role="alert">{{ renameError }}</p>
        </template>
        <template v-else>
          <RouterLink
            :to="`/c/${c.id}`"
            class="conv-link"
            :aria-current="c.id === activeId ? 'page' : undefined"
            @click="emit('navigate')"
          >
            {{ label(c) }}
          </RouterLink>
          <button
            type="button"
            class="icon-button plain"
            :data-rename="c.id"
            :aria-label="`Ganti nama: ${label(c)}`"
            @click="startRename(c)"
          >
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path d="M4 20h4L19 9l-4-4L4 16v4ZM14 6l4 4" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" />
            </svg>
          </button>
          <button type="button" class="icon-button plain" :aria-label="`Hapus: ${label(c)}`" @click="askDelete(c)">
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path
                d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linejoin="round"
                stroke-linecap="round"
              />
            </svg>
          </button>
        </template>
      </li>
    </ul>

    <ModalDialog :open="pendingDelete !== null" labelledby="delete-title" @close="pendingDelete = null">
      <h2 id="delete-title" class="dialog-title">Hapus percakapan?</h2>
      <p class="dialog-text">
        "{{ pendingDelete ? label(pendingDelete) : '' }}" beserta semua pesan dan gambarnya akan dihapus permanen.
      </p>
      <p v-if="deleteError" class="dialog-error" role="alert">{{ deleteError }}</p>
      <div class="dialog-actions">
        <!-- Cancel takes initial focus so a stray Enter never deletes. -->
        <button type="button" class="button button-secondary" autofocus @click="pendingDelete = null">Batal</button>
        <button type="button" class="button button-danger" :disabled="deleting" @click="confirmDelete">Hapus percakapan</button>
      </div>
    </ModalDialog>
  </nav>
</template>

<style scoped>
.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-height: 0;
  height: 100%;
}

.new-button {
  justify-content: flex-start;
}

.list-note {
  margin: 0;
  padding: var(--space-2);
  color: var(--color-text-muted);
}

.list-error {
  display: grid;
  gap: var(--space-2);
  color: var(--color-text);
}

.list-error p {
  margin: 0;
}

.conv-list {
  display: grid;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-y: auto;
  min-height: 0;
}

.conv-item {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px;
  border-radius: var(--radius-s);
}

.conv-item.active {
  background: var(--color-surface-muted);
}

.conv-link {
  flex: 1;
  min-width: 0;
  min-height: var(--tap-min);
  display: flex;
  align-items: center;
  padding: 0 var(--space-2);
  border-radius: var(--radius-s);
  color: var(--color-text);
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-link:hover {
  text-decoration: underline;
}

.conv-item.active .conv-link {
  font-weight: 600;
}

.rename-input {
  flex: 1;
  min-width: 0;
  min-height: var(--tap-min);
  padding: 0 var(--space-2);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-s);
  background: var(--color-bg);
}

.rename-error {
  flex-basis: 100%;
  margin: 0;
  padding: 0 var(--space-2);
  color: var(--color-danger);
  font-size: 0.875rem;
}

.dialog-title {
  margin: 0 0 var(--space-2);
  font-size: 1.2rem;
}

.dialog-text {
  margin: 0 0 var(--space-4);
  overflow-wrap: anywhere;
}

.dialog-error {
  margin: 0 0 var(--space-3);
  color: var(--color-danger);
}

.dialog-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--space-2);
}
</style>
