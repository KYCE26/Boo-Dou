<script setup lang="ts">
// The curve is the mascot's own front brim edge (GhostMascot.vue, hat path
// from -47,-2 down to 0,13 and back up to 47,-2), normalised to 0..94 x 0..15.
const CURVE = 'M0 0C1 6 11 10 23 12 35 14 41 15 47 15 53 15 59 14 71 12 83 10 93 6 94 0'

withDefaults(defineProps<{ variant?: 'line' | 'edge' }>(), { variant: 'line' })
</script>

<template>
  <svg class="brim" :class="variant" viewBox="0 0 94 15" preserveAspectRatio="none" aria-hidden="true" focusable="false">
    <path v-if="variant === 'edge'" class="fill" :d="`${CURVE}V15H0Z`" />
    <path class="stroke" :d="CURVE" vector-effect="non-scaling-stroke" />
  </svg>
</template>

<style scoped>
.brim {
  display: block;
  width: 100%;
  overflow: visible;
  pointer-events: none;
}

.line {
  height: 10px;
}

.edge {
  height: 14px;
}

.stroke {
  fill: none;
  stroke: var(--color-border);
  stroke-width: 1px;
}

.fill {
  fill: var(--color-surface);
}
</style>
