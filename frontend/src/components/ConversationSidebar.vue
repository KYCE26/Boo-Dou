<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import type { Conversation } from '../api/conversations'
import { copy } from '../copy'
import { useChatStore } from '../stores/chat'
import { useConversationsStore } from '../stores/conversations'
import ModalDialog from './ModalDialog.vue'

// The drawer has its own heading, so the list heading is kept for screen readers only there.
const props = withDefaults(defineProps<{ inDrawer?: boolean }>(), { inDrawer: false })
const emit = defineEmits<{ navigate: [] }>()

const conversations = useConversationsStore()
const chat = useChatStore()
const router = useRouter()

const label = (c: Conversation) => c.title || copy.sidebar.untitled

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
  <nav class="sidebar-nav" :aria-label="copy.sidebar.nav">
    <button type="button" class="button button-primary new-button" @click="newConversation">
      <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
        <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" />
      </svg>
      {{ copy.sidebar.newChat }}
    </button>

    <h2 class="list-title" :class="{ 'visually-hidden': props.inDrawer }">{{ copy.sidebar.title }}</h2>

    <p v-if="conversations.state === 'loading' || conversations.state === 'idle'" class="list-note" role="status">
      {{ copy.sidebar.loading }}
    </p>
    <div v-else-if="conversations.state === 'error'" class="list-note list-error" role="alert">
      <p>{{ conversations.error }}</p>
      <button type="button" class="button button-quiet" @click="conversations.load()">{{ copy.sidebar.reload }}</button>
    </div>
    <p v-else-if="conversations.items.length === 0" class="list-note">{{ copy.sidebar.empty }}</p>

    <ul v-else ref="listEl" class="conv-list">
      <li v-for="c in conversations.items" :key="c.id" class="conv-item" :class="{ active: c.id === activeId }">
        <template v-if="editingId === c.id">
          <label :for="`rename-${c.id}`" class="visually-hidden">{{ copy.sidebar.renameLabel }}</label>
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
            class="icon-button bare"
            :data-rename="c.id"
            :aria-label="copy.sidebar.rename(label(c))"
            @click="startRename(c)"
          >
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path d="M4 20h4L19 9l-4-4L4 16v4ZM14 6l4 4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" />
            </svg>
          </button>
          <button type="button" class="icon-button bare" :aria-label="copy.sidebar.remove(label(c))" @click="askDelete(c)">
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path
                d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linejoin="round"
                stroke-linecap="round"
              />
            </svg>
          </button>
        </template>
      </li>
    </ul>

    <ModalDialog :open="pendingDelete !== null" labelledby="delete-title" @close="pendingDelete = null">
      <h2 id="delete-title" class="dialog-title">{{ copy.deleteDialog.title }}</h2>
      <p class="dialog-text">{{ pendingDelete ? copy.deleteDialog.body(label(pendingDelete)) : '' }}</p>
      <p v-if="deleteError" class="dialog-error" role="alert">{{ deleteError }}</p>
      <div class="dialog-actions">
        <!-- Cancel takes initial focus so a stray Enter never deletes. -->
        <button type="button" class="button button-quiet" autofocus @click="pendingDelete = null">{{ copy.deleteDialog.cancel }}</button>
        <button type="button" class="button button-ink" :disabled="deleting" @click="confirmDelete">{{ copy.deleteDialog.confirm }}</button>
      </div>
    </ModalDialog>
  </nav>
</template>

<style scoped>
.sidebar-nav {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}

.new-button {
  justify-content: flex-start;
  width: 100%;
}

.list-title {
  margin: var(--space-5) 0 var(--space-2);
  padding-left: var(--space-2);
  font-size: var(--text-s);
  font-weight: 700;
  color: var(--color-muted);
}

.list-note {
  margin: 0;
  padding: var(--space-2);
  color: var(--color-muted);
}

.list-error {
  display: grid;
  gap: var(--space-2);
  color: var(--color-ink);
}

.list-error p {
  margin: 0;
}

.conv-list {
  display: grid;
  /* Long nowrap titles would otherwise size the track and push the actions out. */
  grid-template-columns: minmax(0, 1fr);
  align-content: start;
  gap: 2px;
  margin: 0;
  padding: 0 0 var(--space-4);
  list-style: none;
  overflow-y: auto;
  min-height: 0;
}

.conv-item {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  border-radius: var(--radius-m);
}

.conv-item:hover,
.conv-item:active {
  background: var(--color-bg);
}

.conv-item.active {
  background: var(--color-tea);
}

.conv-link {
  flex: 1;
  min-width: 0;
  min-height: var(--tap-min);
  display: flex;
  align-items: center;
  padding: 0 var(--space-2);
  border-radius: var(--radius-m);
  color: var(--color-ink);
  text-decoration: none;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-item.active .conv-link {
  font-weight: 650;
}

.conv-item .icon-button {
  color: var(--color-muted);
}

.conv-item .icon-button:hover,
.conv-item .icon-button:active {
  color: var(--color-ink);
  background: transparent;
}

/* Green, not --color-border: the border only reaches 2.74:1 on the tea tint. */
.rename-input {
  flex: 1;
  min-width: 0;
  min-height: var(--tap-min);
  padding: 0 var(--space-2);
  border: 1px solid var(--color-green);
  border-radius: var(--radius-s);
  background: var(--color-surface);
}

.rename-error {
  flex-basis: 100%;
  margin: 0;
  padding: var(--space-1) var(--space-2);
  font-size: var(--text-s);
}

.dialog-title {
  margin: 0 0 var(--space-2);
  font-size: var(--text-l);
  font-weight: 800;
}

.dialog-text {
  margin: 0 0 var(--space-5);
  overflow-wrap: anywhere;
}

.dialog-error {
  margin: 0 0 var(--space-3);
  font-weight: 650;
}

.dialog-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--space-3);
}
</style>
