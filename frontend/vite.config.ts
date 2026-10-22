import { readFileSync } from 'node:fs'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

// The manifest cannot read CSS variables at runtime, so pull the light-theme
// values from tokens.css at build time to keep a single source of truth.
const tokens = readFileSync(new URL('./src/styles/tokens.css', import.meta.url), 'utf8')
function token(name: string): string {
  const match = tokens.match(new RegExp(`--${name}:\\s*(#[0-9a-fA-F]{3,8})`))
  if (!match) throw new Error(`Token --${name} tidak ditemukan di src/styles/tokens.css`)
  return match[1]!
}

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.ico', 'mascot.svg', 'apple-touch-icon-180x180.png'],
      manifest: {
        name: 'Boo Dou',
        short_name: 'Boo Dou',
        description: 'Asisten AI berbasis web',
        lang: 'id',
        start_url: '/',
        display: 'standalone',
        theme_color: token('color-brand'),
        background_color: token('color-bg'),
        icons: [
          { src: 'pwa-64x64.png', sizes: '64x64', type: 'image/png' },
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: 'maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // Chat responses are live streams; the service worker must never
        // answer /api with a cached app shell.
        navigateFallbackDenylist: [/^\/api\//],
      },
    }),
  ],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
