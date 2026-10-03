<script setup lang="ts">
import { computed, ref } from 'vue'
import type { LiveMarcher, LiveMeasure, LiveTravelLeg, LiveWorld, LocalMap, TravelPace, Vehicle } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import type { Coord } from '@/shared/hex'
import { GButton } from '@/shared/ui'
import { cellsFor, key, layoutOf } from './geometry'
import MapBoard from './MapBoard.vue'
import PartyClock from './PartyClock.vue'
import { duration, journey, MAX_WAYPOINTS, measured } from './travel'
import WorldOverlay from './WorldOverlay.vue'

type Tool = 'node' | 'route' | 'party' | 'remove'

const props = defineProps<{ world?: LiveWorld; dm: boolean; maps: LocalMap[]; localMaps: LocalMap[]; measure: LiveMeasure | null; gameDay: number; gameMinute: number; marchingOrder: LiveMarcher[]; vehicles: Vehicle[] }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()
const choice = ref('')
const tool = ref<Tool>('node')
const name = ref('')
const from = ref<string | null>(null)
const distance = ref(12)
const pace = ref<TravelPace>('normal')
// aboard is the vehicle the party travels in; a vehicle that cannot move is not offered.
const aboard = ref('')
const afloat = computed(() => props.vehicles.filter((v) => v.speed > 0))
const travel = (routeId: string) => { emit('send', aboard.value ? { kind: 'travel', routeId, vehicleId: aboard.value } : { kind: 'travel', routeId, pace: pace.value }) }
const how = (l: LiveTravelLeg) => (l.vehicle ? `aboard ${l.vehicle}` : `at a ${l.pace} pace`)

const nodes = computed(() => new Map((props.world?.nodes ?? []).map((n) => [n.id, n])))
const here = computed(() => (props.world?.partyNodeId ? nodes.value.get(props.world.partyNodeId) : undefined))
const nameOf = (id: string) => nodes.value.get(id)?.name ?? 'somewhere unseen'
const roads = computed(() =>
  (props.world?.routes ?? [])
    .filter((r) => here.value && (r.fromNodeId === here.value.id || r.toNodeId === here.value.id))
    .map((r) => {
      const plan = r.plans.find((p) => p.pace === pace.value)
      return { route: r, to: nameOf(r.fromNodeId === here.value?.id ? r.toNodeId : r.fromNodeId), time: plan ? duration(plan.minutes, plan.days) : '' }
    }),
)
// Fog follows what the party has found: with the world map, land it has not been to is dimmed; without
// it, dark. The DM's own map is neither.
const board = computed(() => {
  const w = props.world
  const visible = w?.revealed ?? []
  if (!w?.found || props.dm) return { tokens: [], fog: true, visible, remembered: [] }
  const been = new Set(visible.map(key))
  return { tokens: [], fog: true, visible, remembered: cellsFor(layoutOf(w.map), w.map.width, w.map.height).filter((c) => !been.has(key(c))) }
})
// What the DM says the party has found: the world map itself, and the local Maps at its locations.
const secret = ref(false)
const localMap = ref('')
const placed = computed(() => new Set((props.world?.nodes ?? []).flatMap((n) => (n.mapId ? [n.mapId] : []))))
const free = computed(() => props.localMaps.filter((m) => !placed.value.has(m.id)))
function findable(w: LiveWorld) {
  const locals = w.nodes.flatMap((n) => (n.mapId ? [{ id: n.mapId, label: `${props.localMaps.find((m) => m.id === n.mapId)?.name ?? 'A local map'}, at ${n.name}`, found: n.found === true }] : []))
  return [{ id: w.map.id, label: `${w.map.name}, the world map`, found: w.found }, ...locals]
}
const tools: { value: Tool; label: string }[] = [
  { value: 'node', label: 'Add a location' },
  { value: 'route', label: 'Join two locations' },
  { value: 'party', label: 'Put the party there' },
  { value: 'remove', label: 'Remove a location' },
]

// Anyone measures a route: each tap adds a waypoint, and the server answers with its length.
const measuring = ref(false)
const waypoints = ref<Coord[]>([])
const measurePace = ref<TravelPace>('normal')
const full = computed(() => waypoints.value.length >= MAX_WAYPOINTS)
const result = computed(() => (waypoints.value.length > 1 && props.measure ? measured(props.measure, measurePace.value) : ''))
const hint = computed(() => {
  if (full.value) return `A route has at most ${String(MAX_WAYPOINTS)} points.`
  return waypoints.value.length === 0 ? 'Tap the map to mark the route. Each tap adds a point.' : 'Tap where the route goes next.'
})
function route(points: Coord[]) {
  waypoints.value = points
  if (points.length > 1) emit('send', { kind: 'measure_route', hexes: points })
}
function toggleMeasure() {
  measuring.value = !measuring.value
  waypoints.value = []
}

function tap(c: Coord) {
  if (measuring.value) {
    if (!full.value) route([...waypoints.value, { q: c.q, r: c.r }])
    return
  }
  if (!props.dm) return
  const n = props.world?.nodes.find((x) => x.q === c.q && x.r === c.r)
  if (tool.value === 'node') {
    if (!n && name.value.trim()) emit('send', { kind: 'add_node', label: name.value.trim(), q: c.q, r: c.r, ...(secret.value ? { secret: true } : {}), ...(localMap.value ? { mapId: localMap.value } : {}) })
    return
  }
  if (!n) return
  if (tool.value === 'party') emit('send', { kind: 'place_party', nodeId: n.id })
  else if (tool.value === 'remove') emit('send', { kind: 'remove_node', nodeId: n.id })
  else if (!from.value || from.value === n.id) from.value = n.id
  else {
    emit('send', { kind: 'add_route', nodeId: from.value, toNodeId: n.id, distanceMi: distance.value })
    from.value = null
  }
}
</script>

<template>
  <section class="world" aria-label="World map" data-testid="world">
    <div v-if="dm" class="row">
      <label class="g-field grow">
        <span>World map</span>
        <select v-model="choice" data-testid="world-choice">
          <option value="">None</option>
          <option v-for="m in maps" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select>
      </label>
      <GButton data-testid="use-world" @click="emit('send', choice ? { kind: 'set_world', mapId: choice } : { kind: 'set_world' })">Use world map</GButton>
    </div>
    <PartyClock :game-day="gameDay" :game-minute="gameMinute" :marching-order="marchingOrder" :dm="dm" @send="(cmd) => emit('send', cmd)" />
    <p v-if="!world" class="hint" data-testid="no-world">{{ dm ? 'Choose a world map for the party to travel.' : 'The DM has not opened a world map yet.' }}</p>
    <template v-else>
      <MapBoard :map="world.map" :view="board" :dm="dm" :title="world.map.name" :path="waypoints" @select="tap">
        <template #default="{ layout }">
          <WorldOverlay :world="world" :layout="layout" :from="from" :measured="waypoints" />
        </template>
      </MapBoard>
      <p v-if="!dm" class="hint" data-testid="world-fog">
        {{ world.found ? 'The party has this map: it is dimmed where the party has not been.' : 'The party has no map of these lands: only where it has been shows.' }}
      </p>
      <p role="status" data-testid="party-at">{{ here ? `The party is at ${here.name}.` : 'The party is not on this map yet.' }}</p>
      <div class="row">
        <GButton :aria-pressed="measuring" data-testid="measure-toggle" @click="toggleMeasure()">Measure a route</GButton>
      </div>
      <section v-if="measuring" class="g-card" aria-label="Measured route" data-testid="measure">
        <p v-if="result" role="status" class="measured" data-testid="measure-result">{{ result }}</p>
        <p class="hint" data-testid="measure-hint">{{ hint }}</p>
        <div class="row">
          <label class="g-field">
            <span>Pace</span>
            <select v-model="measurePace" data-testid="measure-pace">
              <option value="slow">Slow (2 mph)</option>
              <option value="normal">Normal (3 mph)</option>
              <option value="fast">Fast (4 mph)</option>
            </select>
          </label>
          <GButton :disabled="waypoints.length === 0" data-testid="measure-undo" @click="route(waypoints.slice(0, -1))">Take back a point</GButton>
          <GButton :disabled="waypoints.length === 0" data-testid="measure-clear" @click="route([])">Clear</GButton>
        </div>
      </section>
      <section v-if="roads.length > 0" class="g-card" aria-label="Routes from here" data-testid="roads">
        <h2>Routes from here</h2>
        <label v-if="afloat.length > 0" class="g-field">
          <span>Travelling</span>
          <select v-model="aboard" data-testid="aboard">
            <option value="">On foot</option>
            <option v-for="v in afloat" :key="v.id" :value="v.id">{{ v.name }} ({{ v.speed }} miles a day)</option>
          </select>
        </label>
        <label v-if="dm && !aboard" class="g-field">
          <span>Pace</span>
          <select v-model="pace" data-testid="pace">
            <option value="slow">Slow (2 mph)</option>
            <option value="normal">Normal (3 mph)</option>
            <option value="fast">Fast (4 mph)</option>
          </select>
        </label>
        <ul class="g-list">
          <li v-for="r in roads" :key="r.route.id" class="row">
            <span>{{ r.to }} · {{ r.route.distanceMi }} mi<template v-if="!aboard"> · {{ r.time }}</template></span>
            <GButton v-if="dm" :aria-label="`Travel to ${r.to}`" @click="travel(r.route.id)">Travel</GButton>
          </li>
        </ul>
      </section>
      <section v-if="world.legs.length > 0" class="g-card" aria-label="Journey this session" data-testid="legs">
        <h2>Journey this session</h2>
        <ol class="g-list">
          <li v-for="(l, i) in world.legs" :key="i">{{ l.from }} → {{ l.to }} · {{ l.distanceMi }} mi {{ how(l) }} · {{ duration(l.minutes, l.days) }}</li>
        </ol>
        <p data-testid="journey">In all: {{ journey(world.legs) }}</p>
      </section>
      <section v-if="dm" class="g-card" aria-label="Maps the party has found" data-testid="found-maps">
        <h2>Found maps</h2>
        <p class="hint">Without the world map the party sees only where it has been; a local map it finds shows at its place.</p>
        <ul class="g-list">
          <li v-for="m in findable(world)" :key="m.id" class="row">
            <span>{{ m.label }}: {{ m.found ? 'found' : 'not found' }}</span>
            <GButton :data-testid="`find-${m.id}`" @click="emit('send', { kind: 'find_map', mapId: m.id, on: !m.found })">{{ m.found ? 'The party lost it' : 'The party found it' }}</GButton>
          </li>
        </ul>
      </section>
      <section v-if="dm" class="g-card" aria-label="World map tools">
        <fieldset class="row tools">
          <legend>Tap the world map to</legend>
          <label v-for="t in tools" :key="t.value" class="check">
            <input v-model="tool" type="radio" :value="t.value" :data-testid="`world-tool-${t.value}`" @change="from = null" />
            <span>{{ t.label }}</span>
          </label>
        </fieldset>
        <template v-if="tool === 'node'">
          <label class="g-field"><span>Location name</span><input v-model="name" maxlength="40" data-testid="world-node-name" /></label>
          <label class="check"><input v-model="secret" type="checkbox" data-testid="world-node-secret" /><span>A secret place: only you see it</span></label>
          <label class="g-field">
            <span>Local map that lies there</span>
            <select v-model="localMap" data-testid="world-node-map">
              <option value="">No local map</option>
              <option v-for="m in free" :key="m.id" :value="m.id">{{ m.name }}</option>
            </select>
          </label>
        </template>
        <template v-if="tool === 'route'">
          <label class="g-field"><span>Distance (miles)</span><input v-model.number="distance" type="number" min="1" max="2000" data-testid="world-distance" /></label>
          <p class="hint" role="status">{{ from ? `From ${nameOf(from)}: now tap where the route goes.` : 'Tap the location the route starts from.' }}</p>
        </template>
        <ul class="g-list" aria-label="Routes">
          <li v-for="r in world.routes" :key="r.id" class="row">
            <span>{{ nameOf(r.fromNodeId) }} – {{ nameOf(r.toNodeId) }} · {{ r.distanceMi }} mi</span>
            <GButton variant="danger" :data-testid="`remove-route-${r.id}`" @click="emit('send', { kind: 'remove_route', routeId: r.id })">Remove</GButton>
          </li>
        </ul>
      </section>
    </template>
  </section>
</template>

<style scoped>
.world {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.measured {
  margin: 0;
  font-weight: 700;
}
.tools {
  margin: 0;
  padding: 0;
  border: 0;
}
.grow {
  flex: 1 1 200px;
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
.hint {
  color: var(--color-text-2);
}
h2 {
  margin: 0 0 8px;
  font-size: 18px;
}
</style>
