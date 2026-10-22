import { createRouter, createWebHistory } from 'vue-router'
import ChatView from '../views/ChatView.vue'

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    // Both paths share one ChatView instance; it reacts to the param instead
    // of remounting, which keeps a stream alive when "/" becomes "/c/:id".
    { path: '/', name: 'new', component: ChatView },
    { path: '/c/:id', name: 'conversation', component: ChatView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
