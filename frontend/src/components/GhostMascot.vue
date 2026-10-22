<script setup lang="ts">
import { ref, useId, watch } from 'vue'

export type GhostState = 'idle' | 'thinking' | 'happy' | 'error'

const props = withDefaults(defineProps<{ state: GhostState; size?: number }>(), {
  size: 120,
})

// SVG ids are document-global; prefixing them per instance keeps clip paths
// and the title from resolving to another mascot on the same page.
const uid = useId()
const underHatId = `${uid}-ghost-under-hat`
const bodyClipId = `${uid}-ghost-body-clip`
const bodyShapeId = `${uid}-ghost-body-shape`
const titleId = `${uid}-ghost-title`

// Remounting .rig is the only reliable way to restart a CSS animation, and it
// covers both a state change and replay() with the same state.
const playId = ref(0)
watch(
  () => props.state,
  () => playId.value++,
)

function replay() {
  playId.value++
}

defineExpose({ replay })
</script>

<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 120 140"
    role="img"
    :aria-labelledby="titleId"
    class="ghost"
    :data-state="state"
    :width="size"
    :height="(size * 140) / 120"
  >
    <title :id="titleId">Maskot hantu bertopi fedora</title>
    <defs>
      <!-- Kepala dipotong di garis tengah brim, jadi tidak ada bagian kepala yang nongol di samping atau atas topi -->
      <clipPath :id="underHatId" clipPathUnits="userSpaceOnUse">
        <path transform="translate(60 46) rotate(14)" d="M-130 0H130V160H-130Z" />
      </clipPath>
      <clipPath :id="bodyClipId" clipPathUnits="userSpaceOnUse">
        <path
          :id="bodyShapeId"
          d="M60 24C80 24 92 40 93 62 94 86 88 104 72 116 60 125 40 130 14 122 26 118 32 110 31 98 28 86 27 74 27 62 28 40 40 24 60 24Z"
        />
      </clipPath>
    </defs>

    <g :key="playId" class="rig">
      <ellipse class="body ln" cx="28.5" cy="95" rx="5.5" ry="4" transform="rotate(35 28.5 95)" />
      <ellipse class="body ln" cx="92.5" cy="92" rx="5.5" ry="4" transform="rotate(-35 92.5 92)" />

      <g :clip-path="`url(#${underHatId})`">
        <use :href="`#${bodyShapeId}`" class="body ln" />
        <g :clip-path="`url(#${bodyClipId})`">
          <!-- Satu tone bayangan di sisi kiri dan buntut; cahaya datang dari kanan atas -->
          <path fill="#000" opacity=".07" d="M27 36C25 52 27 74 31 98 32 110 26 118 14 122 34 126 48 122 56 116 40 104 32 84 33 36Z" />
          <!-- Bayangan brim jatuh di dahi, ikut kemiringan topi -->
          <path fill="#000" opacity=".13" transform="translate(60 46) rotate(14)" d="M-46 2C-36 12-16 18 0 18 16 18 36 12 46 2Z" />
        </g>
      </g>

      <g class="face f-idle">
        <ellipse class="ink" cx="50" cy="78" rx="4" ry="6" />
        <ellipse class="ink" cx="71" cy="78" rx="4" ry="6" />
        <circle class="glint" cx="51.4" cy="75.6" r="1.4" />
        <circle class="glint" cx="72.4" cy="75.6" r="1.4" />
        <path class="ln" fill="none" d="M57 90q3.5 3 7 0" />
      </g>

      <g class="face f-thinking">
        <ellipse class="ink" cx="52" cy="76" rx="3.6" ry="5.4" />
        <ellipse class="ink" cx="73" cy="76" rx="3.6" ry="5.4" />
        <circle class="glint" cx="53.6" cy="73.4" r="1.3" />
        <circle class="glint" cx="74.6" cy="73.4" r="1.3" />
        <path class="ln" fill="none" d="M59 91h6" />
      </g>

      <g class="face f-happy">
        <path class="ln" fill="none" d="M45 79q5-7 10 0M66 79q5-7 10 0" />
        <path class="ink ln" d="M55 87h12q0 8-6 8t-6-8Z" />
      </g>

      <g class="face f-error">
        <path class="ln" fill="none" d="M46 74l8 8M54 74l-8 8M67 74l8 8M75 74l-8 8" />
        <path class="ln" fill="none" d="M53 92q2-3 4 0t4 0 4 0 4 0" />
      </g>

      <g transform="translate(60 46) rotate(14)">
        <!-- Brim snap: tepi belakang naik, tepi depan turun di tengah, ujung kiri-kanan sedikit melengkung ke atas -->
        <path class="hat ln" d="M-47-2C-40-11 40-11 47-2 46 4 36 8 24 10 12 12 6 13 0 13-6 13-12 12-24 10-36 8-46 4-47-2Z" />
        <!-- Tebal brim depan, flat tone supaya brim terasa punya volume -->
        <path fill="#fff" opacity=".09" d="M44 1C38 6 24 8.6 0 10.4-24 8.6-38 6-44 1-36 7-14 10.6 0 10.6 14 10.6 36 7 44 1Z" />
        <!-- Crown fedora: puncak satu lengkung halus -->
        <path class="hat ln" d="M-29 3C-29.5-8-28-20-24-27-20-34 20-34 24-27 28-20 29.5-8 29 3 18 7-18 7-29 3Z" />
        <path fill="#fff" opacity=".11" d="M10-32C17-32 21-30 24-27 28-20 29.5-10 29-4 26-3 23-2.5 20-2.4 21-14 19-25 10-32Z" />
        <path class="band ln" d="M-28.7-6C-18-2 18-2 28.7-6L29 3C18 7-18 7-29 3Z" />
        <path class="star" d="M-17-3q.8 3.1 3.6 3.6-2.8.5-3.6 3.6-.8-3.1-3.6-3.6 2.8-.5 3.6-3.6Z" />
      </g>
    </g>
  </svg>
</template>

<style scoped>
.ghost {
  --_body: var(--ghost-body, #F6F4EF);
  --_line: var(--ghost-outline, #26222B);
  --_hat: var(--ghost-hat, #1B191E);
  --_accent: var(--ghost-accent, #2E7A50);
  --_eye: var(--ghost-eye, var(--_line));
  overflow: visible;
  flex-shrink: 0;
}
.ln { stroke: var(--_line); stroke-width: 2.4; stroke-linejoin: round; stroke-linecap: round; }
.body { fill: var(--_body); }
.hat { fill: var(--_hat); }
.band { fill: var(--_accent); }
.ink { fill: var(--_eye); }
.face .ln { stroke: var(--_eye); }
.star { fill: #F6F4EF; }
.glint { fill: var(--_body); }
.face { display: none; }
.ghost[data-state="idle"] .f-idle,
.ghost[data-state="thinking"] .f-thinking,
.ghost[data-state="happy"] .f-happy,
.ghost[data-state="error"] .f-error { display: inline; }
.rig { transform-box: view-box; transform-origin: 60px 124px; }

/* Gerak hanya saat state dipicu aplikasi; bob thinking berhenti begitu jawaban selesai */
.ghost[data-state="thinking"] .rig { animation: g-bob 1.4s ease-in-out infinite; }
.ghost[data-state="happy"] .rig { animation: g-hop .45s ease-out 1; }
.ghost[data-state="error"] .rig { animation: g-shake .4s ease-in-out 1; }
@keyframes g-bob { 50% { transform: translateY(-3px); } }
@keyframes g-hop { 40% { transform: translateY(-6px) scaleY(1.03); } }
@keyframes g-shake { 25% { transform: translateX(-2.5px); } 75% { transform: translateX(2.5px); } }
@media (prefers-reduced-motion: reduce) { .ghost .rig { animation: none !important; } }
</style>
