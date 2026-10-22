import { defineConfig, minimal2023Preset } from '@vite-pwa/assets-generator/config'

// Maskable and Apple icons are shown on an opaque tile, so give them the app
// background instead of black; transparent icons keep the ghost's own shape.
const tile = { background: '#f3efe4', fit: 'contain' as const }

export default defineConfig({
  headLinkOptions: { preset: '2023' },
  preset: {
    ...minimal2023Preset,
    maskable: { ...minimal2023Preset.maskable, resizeOptions: tile },
    apple: { ...minimal2023Preset.apple, resizeOptions: tile },
  },
  // Rendered from the flattened copy made by scripts/build-icons.mjs.
  images: ['.icon-build/mascot.svg'],
})
