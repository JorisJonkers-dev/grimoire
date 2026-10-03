<script setup lang="ts">
import { computed } from 'vue'
import type { LiveMap, LiveView } from '@/infrastructure/api/types.gen'
import { type Coord, corners, toPixel } from '@/shared/hex'
import { type Captions, DANGER_NOTE, describe, groundNotes, initials } from './board'
import { cellsFor, key, layoutOf, squareLines } from './geometry'

const props = withDefaults(
  defineProps<{ map: LiveMap; view: LiveView; dm?: boolean; selected?: string | null; path?: Coord[]; danger?: Coord[]; area?: Coord[]; zone?: Coord[]; reach?: Coord[]; captions?: Captions; title: string }>(),
  { dm: false, selected: null, path: () => [], danger: () => [], captions: () => ({}), area: () => [], zone: () => [], reach: () => [] },
)
const emit = defineEmits<{ select: [coord: Coord] }>()

const layout = computed(() => layoutOf(props.map))
// The grid lies see-through over the picture: as hexes, as squares the width of a hex, or not drawn.
const grid = computed(() => {
  const strength = String(props.map.gridStrength / 100)
  return { '--grid': props.map.gridKind === 'hexes' ? strength : '0', '--grid-squares': props.map.gridKind === 'squares' ? strength : '0' }
})
const squares = computed(() => (props.map.gridKind === 'squares' ? squareLines(layout.value, props.map.width, props.map.height) : ''))
const visible = computed(() => new Set(props.view.visible.map(key)))
const remembered = computed(() => new Set(props.view.remembered.map(key)))
const walls = computed(() => new Set((props.view.walls ?? []).map(key)))
const lights = computed(() => new Map((props.view.lights ?? []).map((l) => [key(l), l])))
const tokens = computed(() => new Map(props.view.tokens.map((t) => [key(t), t])))
const route = computed(() => new Set(props.path.map(key)))
const danger = computed(() => new Set(props.danger.map(key)))
const surfaces = computed(() => new Map((props.view.surfaces ?? []).map((s) => [key(s), s])))
const heights = computed(() => new Map((props.view.elevation ?? []).map((e) => [key(e), e.elevationFt])))
// shade darkens sunken ground and lightens raised ground, more the further from level it is.
const shade = (ft: number) => Math.min(0.6, 0.12 + Math.abs(ft) / 50).toFixed(2)
const area = computed(() => new Set(props.area.map(key)))
const zone = computed(() => new Set(props.zone.map(key)))
const reach = computed(() => new Set(props.reach.map(key)))
const points = (c: Coord) =>
  corners(layout.value, c)
    .map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`)
    .join(' ')
const objects = computed(() => new Map((props.view.objects ?? []).map((o) => [key({ q: o.q, r: o.r }), o])))
const objectNote = (k: string) => {
  const o = objects.value.get(k)
  if (!o) return ''
  return `${o.name} (${o.broken ? 'broken' : o.kind === 'lever' ? (o.open ? 'pulled' : 'up') : o.open ? 'open' : 'closed'})`
}
const cells = computed(() =>
  cellsFor(layout.value, props.map.width, props.map.height).map((c) => {
    const k = key(c)
    const fog = !props.view.fog || visible.value.has(k) ? 'lit' : remembered.value.has(k) ? 'remembered' : 'unseen'
    const t = tokens.value.get(k)
    const label = [
      `Hex ${k.replace(',', ', ')}`,
      fog === 'unseen' ? 'never seen' : fog === 'remembered' ? 'remembered' : '',
      t ? describe(t) : '',
      t ? (props.captions[t.id]?.note ?? '') : '',
      walls.value.has(k) ? 'wall' : '',
      lights.value.has(k) ? 'light' : '',
      objectNote(k),
      route.value.has(k) ? 'on the path' : '',
      danger.value.has(k) ? DANGER_NOTE : '',
      ...groundNotes(k, surfaces.value, area.value, zone.value, reach.value),
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
      :data-grid="map.gridKind"
      :style="grid"
    >
      <image :href="map.imageUrl" x="0" y="0" :width="map.width" :height="map.height" preserveAspectRatio="none" data-testid="map-image" />
      <g
        v-for="c in cells"
        :key="c.k"
        :class="['cell', `cell--${c.fog}`, { 'cell--wall': walls.has(c.k), 'cell--selected': c.token && c.token.id === selected, 'cell--dm': dm, 'cell--path': route.has(c.k), 'cell--danger': danger.has(c.k), 'cell--area': area.has(c.k), 'cell--zone': zone.has(c.k), 'cell--watched': reach.has(c.k) }, surfaces.has(c.k) ? `cell--surface-${surfaces.get(c.k)?.kind ?? ''}` : '']"
        role="button"
        tabindex="0"
        :aria-label="c.label"
        :data-hex="c.k"
        @click="emit('select', { q: c.q, r: c.r })"
        @keydown.enter.prevent="emit('select', { q: c.q, r: c.r })"
      >
        <polygon :points="c.points" />
        <polygon v-if="heights.has(c.k)" :points="c.points" :class="['height', (heights.get(c.k) ?? 0) > 0 ? 'height--up' : 'height--down']" :style="{ opacity: shade(heights.get(c.k) ?? 0) }" :data-height="heights.get(c.k)" />
        <circle v-if="lights.has(c.k)" :cx="c.centre.x" :cy="c.centre.y" :r="layout.size * 0.22" class="light" />
        <template v-if="c.token">
          <circle :cx="c.centre.x" :cy="c.centre.y" :r="layout.size * 0.62" :class="['token', `token--${c.token.kind}`, { 'token--hidden': c.token.hidden }]" />
          <text :x="c.centre.x" :y="c.centre.y + layout.size * 0.2" text-anchor="middle" class="mark" aria-hidden="true">{{ initials(c.token.label) }}</text>
          <text
            v-if="captions[c.token.id]"
            :x="c.centre.x"
            :y="c.centre.y + layout.size * 0.95"
            text-anchor="middle"
            class="caption"
            :style="{ fontSize: `${String(Math.round(layout.size * 0.3))}px` }"
            aria-hidden="true"
            :data-testid="`caption-${c.token.id}`"
          >
            {{ captions[c.token.id]?.text }}
          </text>
        </template>
      </g>
      <path v-if="squares" :d="squares" class="squares" data-testid="grid-squares" />
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
  stroke: rgb(255 255 255 / var(--grid));
  stroke-width: 1;
  cursor: pointer;
}
.squares {
  fill: none;
  stroke: rgb(255 255 255 / var(--grid-squares));
  stroke-width: 1;
  pointer-events: none;
}
.cell .height {
  pointer-events: none;
  stroke: none;
}
.cell .height--up {
  fill: #fff;
}
.cell .height--down {
  fill: #000;
}
.cell--watched polygon {
  fill: rgb(220 60 60 / 18%);
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
.caption {
  fill: var(--color-gold-high);
  paint-order: stroke;
  stroke: #000;
  stroke-width: 3px;
  pointer-events: none;
}
.cell--danger polygon {
  fill: rgb(200 60 60 / 35%);
  stroke: var(--color-enemy);
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
