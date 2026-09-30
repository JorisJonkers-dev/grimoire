<script setup lang="ts">
import { computed } from 'vue'
import type { LiveMap, LiveView } from '@/infrastructure/api/types.gen'
import { type Coord, corners, toPixel } from '@/shared/hex'
import { describe, groundNotes, initials } from './board'
import { cellsFor, key, layoutOf } from './geometry'

const props = withDefaults(
  defineProps<{ map: LiveMap; view: LiveView; dm?: boolean; selected?: string | null; path?: Coord[]; area?: Coord[]; zone?: Coord[]; title: string }>(),
  { dm: false, selected: null, path: () => [], area: () => [], zone: () => [] },
)
const emit = defineEmits<{ select: [coord: Coord] }>()

const layout = computed(() => layoutOf(props.map))
const visible = computed(() => new Set(props.view.visible.map(key)))
const remembered = computed(() => new Set(props.view.remembered.map(key)))
const walls = computed(() => new Set((props.view.walls ?? []).map(key)))
const lights = computed(() => new Map((props.view.lights ?? []).map((l) => [key(l), l])))
const tokens = computed(() => new Map(props.view.tokens.map((t) => [key(t), t])))
const route = computed(() => new Set(props.path.map(key)))
const surfaces = computed(() => new Map((props.view.surfaces ?? []).map((s) => [key(s), s])))
const heights = computed(() => new Map((props.view.elevation ?? []).map((e) => [key(e), e.elevationFt])))
const area = computed(() => new Set(props.area.map(key)))
const zone = computed(() => new Set(props.zone.map(key)))
const points = (c: Coord) =>
  corners(layout.value, c)
    .map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`)
    .join(' ')
const cells = computed(() =>
  cellsFor(layout.value, props.map.width, props.map.height).map((c) => {
    const k = key(c)
    const fog = !props.view.fog || visible.value.has(k) ? 'lit' : remembered.value.has(k) ? 'remembered' : 'unseen'
    const t = tokens.value.get(k)
    const label = [
      `Hex ${k.replace(',', ', ')}`,
      fog === 'unseen' ? 'never seen' : fog === 'remembered' ? 'remembered' : '',
      t ? describe(t) : '',
      walls.value.has(k) ? 'wall' : '',
      lights.value.has(k) ? 'light' : '',
      route.value.has(k) ? 'on the path' : '',
      ...groundNotes(k, surfaces.value, area.value, zone.value),
      heights.value.has(k) ? `${String(heights.value.get(k))} ft high` : '',
    ]
      .filter(Boolean)
      .join(': ')
    return { ...c, k, fog, token: t, points: points(c), centre: toPixel(layout.value, c), label }
  }),
)
</script>

<template>
  <div class="board-scroll">
    <svg
      class="board"
      :viewBox="`0 0 ${String(map.width)} ${String(map.height)}`"
      :width="map.width"
      :height="map.height"
      role="group"
      :aria-label="title"
      data-testid="map-board"
    >
      <image :href="map.imageUrl" x="0" y="0" :width="map.width" :height="map.height" preserveAspectRatio="none" data-testid="map-image" />
      <g
        v-for="c in cells"
        :key="c.k"
        :class="['cell', `cell--${c.fog}`, { 'cell--wall': walls.has(c.k), 'cell--selected': c.token && c.token.id === selected, 'cell--dm': dm, 'cell--path': route.has(c.k), 'cell--area': area.has(c.k), 'cell--zone': zone.has(c.k) }, surfaces.has(c.k) ? `cell--surface-${surfaces.get(c.k)?.kind ?? ''}` : '']"
        role="button"
        tabindex="0"
        :aria-label="c.label"
        :data-hex="c.k"
        @click="emit('select', { q: c.q, r: c.r })"
        @keydown.enter.prevent="emit('select', { q: c.q, r: c.r })"
      >
        <polygon :points="c.points" />
        <circle v-if="lights.has(c.k)" :cx="c.centre.x" :cy="c.centre.y" :r="layout.size * 0.22" class="light" />
        <template v-if="c.token">
          <circle :cx="c.centre.x" :cy="c.centre.y" :r="layout.size * 0.62" :class="['token', `token--${c.token.kind}`, { 'token--hidden': c.token.hidden }]" />
          <text :x="c.centre.x" :y="c.centre.y + layout.size * 0.2" text-anchor="middle" class="mark" aria-hidden="true">{{ initials(c.token.label) }}</text>
        </template>
      </g>
      <slot :layout="layout" />
    </svg>
  </div>
</template>

<style scoped>
.board-scroll {
  max-width: 100%;
  overflow: auto;
}
.board {
  display: block;
  background: #000;
}
.cell polygon {
  fill: transparent;
  stroke: rgb(255 255 255 / 12%);
  stroke-width: 1;
  cursor: pointer;
}
.cell--remembered polygon {
  fill: rgb(0 0 0 / 55%);
}
.cell--unseen polygon {
  fill: #000;
}
.cell--unseen.cell--dm polygon {
  fill: rgb(0 0 0 / 35%);
  stroke: rgb(255 255 255 / 25%);
  stroke-dasharray: 3 3;
}
.cell--wall polygon {
  stroke: var(--color-enemy);
  stroke-width: 3;
}
.cell--zone polygon {
  stroke: var(--color-enemy);
  stroke-dasharray: 4 3;
}
.cell--area polygon {
  fill: rgb(212 120 40 / 40%);
}
.cell--surface-fire polygon {
  fill: rgb(200 60 20 / 45%);
}
.cell--surface-grease polygon {
  fill: rgb(90 80 40 / 55%);
}
.cell--surface-water polygon {
  fill: rgb(30 70 140 / 45%);
}
.cell--surface-ice polygon {
  fill: rgb(170 220 250 / 45%);
}
.cell--surface-web polygon {
  fill: rgb(200 200 200 / 35%);
}
.cell--surface-electrified polygon {
  fill: rgb(90 90 230 / 45%);
}
.cell--path polygon {
  fill: rgb(212 175 55 / 30%);
  stroke: var(--color-gold-high);
}
.cell--selected polygon {
  stroke: var(--color-gold-high);
  stroke-width: 3;
}
.cell:focus-visible polygon {
  stroke: var(--color-gold-high);
  stroke-width: 3;
}
.light {
  fill: #ffe38a;
  stroke: #8a6a10;
}
.token {
  stroke-width: 3;
}
.token--party {
  fill: var(--color-party-fill);
  stroke: var(--color-party);
}
.token--enemy {
  fill: var(--color-enemy-fill);
  stroke: var(--color-enemy);
}
.token--npc {
  fill: #2a2438;
  stroke: #b39ddb;
}
.token--object {
  fill: #3a3326;
  stroke: var(--color-bronze);
}
.token--hidden {
  stroke-dasharray: 5 4;
  opacity: 0.7;
}
.mark {
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 15px;
  fill: var(--color-text);
  pointer-events: none;
}
</style>
