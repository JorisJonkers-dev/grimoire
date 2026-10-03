<script setup lang="ts">
import { computed } from 'vue'
import type { BuilderHex } from '@/infrastructure/api/types.gen'

// The area as tinted hexes around its origin, which is outlined.
const props = defineProps<{ hexes: BuilderHex[] }>()
const size = 12
const centre = (h: BuilderHex) => ({ x: size * Math.sqrt(3) * (h.q + h.r / 2), y: size * 1.5 * h.r })
const corners = (h: BuilderHex) => {
  const c = centre(h)
  return Array.from({ length: 6 }, (_, i) => {
    const a = (Math.PI / 180) * (60 * i - 30)
    return `${(c.x + size * Math.cos(a)).toFixed(1)},${(c.y + size * Math.sin(a)).toFixed(1)}`
  }).join(' ')
}
const all = computed(() => [{ q: 0, r: 0 }, ...props.hexes])
const box = computed(() => {
  const xs = all.value.map((h) => centre(h).x)
  const ys = all.value.map((h) => centre(h).y)
  const pad = size * 1.5
  return `${String(Math.min(...xs) - pad)} ${String(Math.min(...ys) - pad)} ${String(Math.max(...xs) - Math.min(...xs) + 2 * pad)} ${String(Math.max(...ys) - Math.min(...ys) + 2 * pad)}`
})
</script>

<template>
  <svg class="area" :viewBox="box" role="img" :aria-label="`The area covers ${String(hexes.length)} hexes`" data-testid="area-preview">
    <polygon v-for="h in hexes" :key="`${String(h.q)},${String(h.r)}`" :points="corners(h)" class="hit" data-testid="area-hex" />
    <polygon :points="corners({ q: 0, r: 0 })" class="origin" />
  </svg>
</template>

<style scoped>
.area {
  width: 100%;
  max-width: 320px;
  max-height: 240px;
}
.hit {
  fill: color-mix(in srgb, var(--color-gold-high) 35%, transparent);
  stroke: var(--color-line);
  stroke-width: 1;
}
.origin {
  fill: none;
  stroke: var(--color-text);
  stroke-width: 2;
}
</style>
