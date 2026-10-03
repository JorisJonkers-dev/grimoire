<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { calibrateMapMutation, getMapOptions, updateMapMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LiveMap, LocalMap, MapEdit } from '@/infrastructure/api/types.gen'
import type { Point } from '@/shared/hex'
import { GButton } from '@/shared/ui'
import { across, cellsBetween } from './geometry'
import MapBoard from './MapBoard.vue'

// A hex of a battle map is always this many feet across.
const LOCAL_HEX_FT = 5

const route = useRoute()
const path = { path: { campaignId: String(route.params.id), mapId: String(route.params.mapId) } }
const map = useQuery({ ...getMapOptions(path), retry: false })
const form = reactive<Required<MapEdit>>({ name: '', hexSizePx: 40, originX: 0, originY: 0, ambient: 'bright', gridStrength: 20, gridKind: 'hexes', scaleMiles: 6 })
const world = computed(() => map.data.value?.kind === 'world')
const a = ref<Point>({ x: 0, y: 0 })
const b = ref<Point>({ x: 0, y: 0 })
const bounds = ref<Point>({ x: 0, y: 0 })
const take = (m: LocalMap) => Object.assign(form, { name: m.name, hexSizePx: m.hexSizePx, originX: m.originX, originY: m.originY, ambient: m.ambient, gridStrength: m.gridStrength, gridKind: m.gridKind, scaleMiles: m.scaleMiles })
watch(
  () => map.data.value,
  (m) => {
    if (!m) return
    take(m)
    bounds.value = { x: m.width, y: m.height }
    a.value = { x: Math.round(m.width / 3), y: Math.round(m.height / 2) }
    b.value = { x: Math.round((2 * m.width) / 3), y: Math.round(m.height / 2) }
  },
  { immediate: true },
)
const preview = computed<LiveMap | null>(() =>
  map.data.value
    ? { ...map.data.value, hexSizePx: form.hexSizePx, originX: form.originX, originY: form.originY, gridKind: form.gridKind, gridStrength: form.gridStrength, imageVersion: 0 }
    : null,
)
const save = useMutation(updateMapMutation())
const edit = (): MapEdit => ({
  name: form.name, hexSizePx: form.hexSizePx, originX: form.originX, originY: form.originY, ambient: form.ambient, gridStrength: form.gridStrength,
  ...(world.value ? { gridKind: form.gridKind, scaleMiles: form.scaleMiles } : {}),
})

// Two points on the picture and the distance between them in the world size the grid.
const unit = computed(() => (world.value ? 'miles' : 'feet'))
const distance = ref<number | ''>('')
const measured = computed(() => {
  const cells = cellsBetween(form.hexSizePx, a.value, b.value)
  return `Now ${cells.toFixed(1)} cells, ${(cells * (world.value ? form.scaleMiles : LOCAL_HEX_FT)).toFixed(1)} ${unit.value} apart.`
})
const calibrate = useMutation(calibrateMapMutation())
const points = [
  { id: 'a', name: 'First', at: a },
  { id: 'b', name: 'Second', at: b },
]
const within = (p: Point): Point => ({
  x: Math.min(Math.max(Math.round(p.x), 0), bounds.value.x),
  y: Math.min(Math.max(Math.round(p.y), 0), bounds.value.y),
})
let drop = () => {}
function drag(event: PointerEvent, at: typeof a) {
  // The board is drawn at the picture's own size, so a pixel of the pointer is a pixel of the picture.
  const box = ((event.currentTarget as SVGGraphicsElement).ownerSVGElement as SVGSVGElement).getBoundingClientRect()
  const move = (e: Event) => {
    const { clientX, clientY } = e as MouseEvent
    at.value = within({ x: clientX - box.left, y: clientY - box.top })
  }
  drop()
  drop = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', drop)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', drop)
}
onBeforeUnmount(() => {
  drop()
})
const steps: Record<string, Point> = { ArrowLeft: { x: -1, y: 0 }, ArrowRight: { x: 1, y: 0 }, ArrowUp: { x: 0, y: -1 }, ArrowDown: { x: 0, y: 1 } }
function nudge(event: KeyboardEvent, at: typeof a) {
  const step = steps[event.key]
  if (!step) return
  event.preventDefault()
  const by = event.shiftKey ? 10 : 1
  at.value = within({ x: at.value.x + step.x * by, y: at.value.y + step.y * by })
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'maps', params: { id: path.path.campaignId } }" class="back">← Maps</RouterLink>
    <p v-if="map.isError.value" role="alert" class="g-alert" data-testid="map-missing">That map is not available.</p>
    <template v-else-if="preview">
      <h1>{{ form.name }}</h1>
      <p class="hint">Line the grid up with the picture: set the hex size and move the origin onto the centre of one hex, or drag the two points onto a distance you know.</p>
      <form class="g-card calibrate" data-testid="map-calibrate" @submit.prevent="save.mutate({ ...path, body: edit() })">
        <label class="g-field"><span>Name</span><input v-model="form.name" maxlength="80" /></label>
        <label class="g-field"><span>Hex size (px, centre to corner)</span><input v-model.number="form.hexSizePx" type="number" min="8" max="400" step="any" data-testid="hex-size" /></label>
        <label class="g-field"><span>Origin x (px)</span><input v-model.number="form.originX" type="number" step="any" data-testid="origin-x" /></label>
        <label class="g-field"><span>Origin y (px)</span><input v-model.number="form.originY" type="number" step="any" /></label>
        <label class="g-field">
          <span>Ambient light</span>
          <select v-model="form.ambient">
            <option value="bright">Bright</option>
            <option value="dim">Dim</option>
            <option value="dark">Dark</option>
          </select>
        </label>
        <template v-if="world">
          <label class="g-field">
            <span>Grid</span>
            <select v-model="form.gridKind" data-testid="grid-kind">
              <option value="hexes">Hexes</option>
              <option value="squares">Squares</option>
              <option value="off">Off</option>
            </select>
          </label>
          <label class="g-field"><span>Miles to a cell</span><input v-model.number="form.scaleMiles" type="number" min="0.1" max="1000" step="any" data-testid="scale-miles" /></label>
        </template>
        <label class="g-field">
          <span>Grid strength <output data-testid="grid-strength-value">{{ form.gridStrength }}%</output></span>
          <input v-model.number="form.gridStrength" type="range" min="0" max="100" data-testid="grid-strength" />
        </label>
        <GButton type="submit" variant="primary">Save calibration</GButton>
        <p v-if="!world" class="hint" data-testid="scale-fixed">A battle map keeps its hexes: each is 5 feet across.</p>
        <p v-if="save.isSuccess.value" role="status" data-testid="calibration-saved">Saved.</p>
        <p v-if="save.isError.value" role="alert" class="g-alert">The calibration could not be saved.</p>
      </form>
      <section class="g-card calibrate" aria-labelledby="two-points-title">
        <h2 id="two-points-title">Calibrate from two points</h2>
        <p class="hint" data-testid="calibrate-measured">{{ measured }}</p>
        <label class="g-field"><span>They are this many {{ unit }} apart</span><input v-model.number="distance" type="number" min="0.01" step="any" data-testid="calibrate-distance" /></label>
        <GButton :disabled="!(Number(distance) > 0)" data-testid="calibrate-go" @click="calibrate.mutate({ ...path, body: { ax: a.x, ay: a.y, bx: b.x, by: b.y, distance: Number(distance) } }, { onSuccess: take })">
          Calibrate
        </GButton>
        <p v-if="calibrate.isSuccess.value" role="status" data-testid="calibrated">Calibrated: a cell is {{ across(form.hexSizePx).toFixed(1) }} px across.</p>
        <p v-if="calibrate.isError.value" role="alert" class="g-alert" data-testid="calibrate-failed">
          Those points and that distance make cells too small or too large. Move the points or change the distance.
        </p>
      </section>
      <MapBoard :map="preview" :view="{ tokens: [], fog: false, visible: [], remembered: [] }" dm :title="`${form.name} with its grid`">
        <line :x1="a.x" :y1="a.y" :x2="b.x" :y2="b.y" class="rule" data-testid="calibrate-line" />
        <circle
          v-for="p in points"
          :key="p.id"
          :cx="p.at.value.x"
          :cy="p.at.value.y"
          r="9"
          class="point"
          role="button"
          tabindex="0"
          :aria-label="`${p.name} point, at ${String(p.at.value.x)}, ${String(p.at.value.y)}. Drag it, or move it with the arrow keys.`"
          :data-testid="`calibrate-${p.id}`"
          @pointerdown.prevent="drag($event, p.at)"
          @keydown="nudge($event, p.at)"
        />
      </MapBoard>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.calibrate {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  align-items: end;
}
.calibrate h2 {
  grid-column: 1 / -1;
  margin: 0;
}
.rule {
  stroke: var(--color-gold-high);
  stroke-width: 2;
  stroke-dasharray: 6 4;
  pointer-events: none;
}
.point {
  fill: var(--color-gold-high);
  stroke: #000;
  stroke-width: 2;
  cursor: grab;
  touch-action: none;
}
.point:focus-visible {
  outline: none;
  stroke: #fff;
  stroke-width: 3;
}
</style>
