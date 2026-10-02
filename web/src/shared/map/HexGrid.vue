<script setup lang="ts">
import { computed } from 'vue'
import { type Coord, corners, type Layout, toPixel } from '@/shared/hex'
import { type GridCell, gridBox } from './grid'


const props = withDefaults(defineProps<{ cells: GridCell[]; size?: number; title: string }>(), { size: 24 })
const emit = defineEmits<{ select: [coord: Coord] }>()

const layout = computed<Layout>(() => ({ size: props.size, origin: { x: 0, y: 0 } }))
const shapes = computed(() =>
  props.cells.map((c) => ({
    ...c,
    points: corners(layout.value, c)
      .map((p) => `${p.x.toFixed(2)},${p.y.toFixed(2)}`)
      .join(' '),
    centre: toPixel(layout.value, c),
  })),
)
const box = computed(() => gridBox(props.cells, props.size))
</script>

<template>
  <div class="hex-scroll">
    <svg
      class="hex-grid"
      :viewBox="`${box.x.toFixed(1)} ${box.y.toFixed(1)} ${box.w.toFixed(1)} ${box.h.toFixed(1)}`"
      :width="Math.round(box.w)"
      :height="Math.round(box.h)"
      role="group"
      :aria-label="title"
    >
      <g
        v-for="s in shapes"
        :key="`${String(s.q)},${String(s.r)}`"
        :class="['hex', s.tone ? `hex--${s.tone}` : '']"
        role="button"
        tabindex="0"
        :aria-label="`Hex ${String(s.q)}, ${String(s.r)}${s.label ? `: ${s.label}` : ''}`"
        :data-hex="`${String(s.q)},${String(s.r)}`"
        @click="emit('select', { q: s.q, r: s.r })"
        @keydown.enter.prevent="emit('select', { q: s.q, r: s.r })"
      >
        <polygon :points="s.points" />
        <text v-if="s.mark" :x="s.centre.x" :y="s.centre.y + size * 0.18" text-anchor="middle" class="mark" aria-hidden="true">{{ s.mark }}</text>
      </g>
    </svg>
  </div>
</template>

<style scoped>
/* Drawn at its natural size so every hex stays a comfortable touch target; wide maps scroll. */
.hex-scroll {
  max-width: 100%;
  overflow-x: auto;
}
.hex-grid {
  display: block;
  touch-action: manipulation;
}
.hex polygon {
  fill: var(--color-felt);
  stroke: var(--color-line);
  stroke-width: 1;
  cursor: pointer;
}
.mark {
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 13px;
  fill: var(--color-text);
  pointer-events: none;
}
.hex--hidden polygon {
  fill: var(--color-enemy-fill);
  stroke: var(--color-enemy);
  stroke-dasharray: 4 3;
  opacity: 0.7;
}
.hex--npc polygon {
  fill: #2a2438;
  stroke: #b39ddb;
}
.hex--object polygon {
  fill: #3a3326;
  stroke: var(--color-bronze);
}
.hex--selected polygon {
  stroke: var(--color-gold-high);
  stroke-width: 3;
}
.hex:focus-visible polygon {
  stroke: var(--color-gold-high);
  stroke-width: 3;
}
.hex--watched polygon {
  fill: rgb(220 60 60 / 18%);
}
.hex--reach polygon {
  fill: #2c4a38;
}
.hex--path polygon {
  fill: var(--color-gold);
}
.hex--start polygon {
  fill: var(--color-party);
}
.hex--wall polygon {
  fill: #3a2e22;
}
.hex--mud polygon {
  fill: #4a3a22;
}
.hex--ally polygon {
  fill: var(--color-party-fill);
  stroke: var(--color-party);
}
.hex--enemy polygon {
  fill: var(--color-enemy-fill);
  stroke: var(--color-enemy);
}
.hex--zone polygon {
  stroke: var(--color-enemy);
  stroke-dasharray: 4 3;
}
.hex--area polygon {
  fill: rgb(212 120 40 / 45%);
  stroke: var(--color-gold-high);
}
.hex--surface-fire polygon {
  fill: #7a2a12;
}
.hex--surface-grease polygon {
  fill: #4b4226;
}
.hex--surface-water polygon {
  fill: #1f3f6b;
}
.hex--surface-ice polygon {
  fill: #7fb2d1;
}
.hex--surface-web polygon {
  fill: #5f5f5f;
}
.hex--surface-electrified polygon {
  fill: #4b4bb3;
}
.hex--seen polygon {
  stroke: var(--color-gold-high);
  stroke-width: 2;
}
</style>
