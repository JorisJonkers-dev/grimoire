<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { createVehicleMutation, damageVehicleMutation, deleteVehicleMutation, listVehiclesOptions, postVehicleCrewMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Vehicle, VehicleComponent, VehicleKind, VehicleStation } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

// A Campaign's vehicles and ships: hull, components, crew stations and how fast each goes as it
// stands. The DM builds them, damages and repairs them and posts their crew; every Member sees them.
const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const list = useQuery({ ...listVehiclesOptions(path), retry: false })
const dm = computed(() => list.data.value?.dm ?? false)
const vehicles = computed(() => list.data.value?.vehicles ?? [])

function stands(v: Vehicle): string {
  const travels = `Travels by ${v.kind}, ${v.kind === 'land' ? 'eight hours a day' : 'round the clock'}.`
  if (v.hull === 0) return `${travels} A wreck: it goes nowhere.`
  if (v.speed === 0) return `${travels} Nothing is left to drive it: it goes nowhere.`
  return `${travels} Makes ${String(v.speed)} of its ${String(v.milesPerDay)} miles a day${v.shortHanded ? ': short of crew, it goes at half speed.' : '.'}`
}
const hullOf = (v: Vehicle) => `Hull: ${String(v.hull)} of ${String(v.hullMax)} hit points.${v.threshold > 0 ? ` Blows under ${String(v.threshold)} damage do nothing.` : ''}`
const partOf = (p: VehicleComponent) => `${p.name}${p.drives ? ', which drives it' : ''}: ${p.hp === 0 ? 'broken' : `${String(p.hp)} of ${String(p.hpMax)} hit points`}`
const postOf = (s: VehicleStation) => `${s.name}: ${String(s.posted)} of ${String(s.crew)} crew`

const problem = ref('')
const failed = () => { problem.value = 'That could not be done. Check what you entered and try again.' }
const done = () => {
  problem.value = ''
  void client.invalidateQueries()
}
const outcome = { onSuccess: done, onError: failed }

// How many hit points one press takes or gives back.
const by = reactive<Record<string, number>>({})
const blow = useMutation(damageVehicleMutation())
function strike(v: Vehicle, repair: boolean, part?: VehicleComponent) {
  const body = { amount: by[v.id] ?? 10, ...(part ? { componentId: part.id } : {}), ...(repair ? { repair } : {}) }
  blow.mutate({ path: { campaignId, vehicleId: v.id }, body }, outcome)
}
const crew = useMutation(postVehicleCrewMutation())
const post = (v: Vehicle, s: VehicleStation, posted: number) => { crew.mutate({ path: { campaignId, vehicleId: v.id, stationId: s.id }, body: { posted } }, outcome) }
const drop = useMutation(deleteVehicleMutation())
const remove = (v: Vehicle) => { drop.mutate({ path: { campaignId, vehicleId: v.id } }, outcome) }

const fresh = () => ({
  name: '', kind: 'land' as VehicleKind, hullMax: 50, threshold: 0, milesPerDay: 24,
  components: [] as { name: string; hpMax: number; drives: boolean }[], stations: [] as { name: string; crew: number }[],
})
const draft = reactive(fresh())
const create = useMutation(createVehicleMutation())
function add() {
  const body = {
    name: draft.name.trim(), kind: draft.kind, hullMax: draft.hullMax, threshold: draft.threshold, milesPerDay: draft.milesPerDay,
    components: draft.components.map((c) => ({ name: c.name.trim(), hpMax: c.hpMax, drives: c.drives })),
    stations: draft.stations.map((s) => ({ name: s.name.trim(), crew: s.crew })),
  }
  create.mutate({ ...path, body }, {
    onSuccess: () => {
      Object.assign(draft, fresh())
      done()
    },
    onError: failed,
  })
}
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Campaign</span>
      <h1>Vehicles</h1>
    </header>
    <p v-if="list.isError.value" role="alert" class="g-alert" data-testid="vehicles-missing">That Campaign is not available.</p>
    <template v-else-if="list.isSuccess.value">
      <p class="hint">Wagons, ships and airships the party travels the world map aboard.</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="vehicles-problem">{{ problem }}</p>
      <p v-if="vehicles.length === 0" class="hint" data-testid="no-vehicles">No vehicles in this Campaign yet.</p>
      <section v-for="v in vehicles" :key="v.id" class="g-card vehicle" :aria-labelledby="`vehicle-title-${v.id}`" :data-testid="`vehicle-${v.id}`">
        <h2 :id="`vehicle-title-${v.id}`">{{ v.name }}</h2>
        <p class="hint" data-testid="vehicle-stands">{{ stands(v) }}</p>
        <label v-if="dm" class="g-field small"><span>Hit points at a time</span><input v-model.number="by[v.id]" type="number" min="1" max="10000" placeholder="10" data-testid="amount" /></label>
        <div class="row">
          <span data-testid="hull">{{ hullOf(v) }}</span>
          <template v-if="dm">
            <GButton :aria-label="`Damage the hull of ${v.name}`" data-testid="hull-damage" @click="strike(v, false)">Damage</GButton>
            <GButton :aria-label="`Repair the hull of ${v.name}`" data-testid="hull-repair" @click="strike(v, true)">Repair</GButton>
          </template>
        </div>
        <ul v-if="v.components.length > 0" class="g-list" :aria-label="`Components of ${v.name}`">
          <li v-for="p in v.components" :key="p.id" class="row" data-testid="component">
            <span>{{ partOf(p) }}</span>
            <template v-if="dm">
              <GButton :aria-label="`Damage ${p.name} of ${v.name}`" data-testid="part-damage" @click="strike(v, false, p)">Damage</GButton>
              <GButton :aria-label="`Repair ${p.name} of ${v.name}`" data-testid="part-repair" @click="strike(v, true, p)">Repair</GButton>
            </template>
          </li>
        </ul>
        <ul v-if="v.stations.length > 0" class="g-list" :aria-label="`Crew stations of ${v.name}`">
          <li v-for="s in v.stations" :key="s.id" class="row" data-testid="station">
            <span>{{ postOf(s) }}</span>
            <template v-if="dm">
              <GButton :aria-label="`One fewer at ${s.name} of ${v.name}`" :disabled="s.posted === 0" data-testid="crew-fewer" @click="post(v, s, s.posted - 1)">−</GButton>
              <GButton :aria-label="`One more at ${s.name} of ${v.name}`" :disabled="s.posted >= s.crew" data-testid="crew-more" @click="post(v, s, s.posted + 1)">+</GButton>
            </template>
          </li>
        </ul>
        <GButton v-if="dm" variant="danger" :aria-label="`Remove ${v.name}`" data-testid="vehicle-remove" @click="remove(v)">Remove</GButton>
      </section>

      <form v-if="dm" class="g-card vehicle" aria-label="Build a vehicle" data-testid="vehicle-add" @submit.prevent="add()">
        <h2>Build a vehicle</h2>
        <label class="g-field"><span>Name</span><input v-model="draft.name" maxlength="80" data-testid="vehicle-name" /></label>
        <div class="row">
          <label class="g-field">
            <span>Travels by</span>
            <select v-model="draft.kind" data-testid="vehicle-kind">
              <option value="land">Land</option>
              <option value="water">Water</option>
              <option value="air">Air</option>
            </select>
          </label>
          <label class="g-field small"><span>Hull hit points</span><input v-model.number="draft.hullMax" type="number" min="1" max="10000" data-testid="vehicle-hull" /></label>
          <label class="g-field small"><span>Damage threshold</span><input v-model.number="draft.threshold" type="number" min="0" max="100" data-testid="vehicle-threshold" /></label>
          <label class="g-field small"><span>Miles a day</span><input v-model.number="draft.milesPerDay" type="number" min="1" max="1000" data-testid="vehicle-speed" /></label>
        </div>
        <h3>Components</h3>
        <ol class="rows">
          <li v-for="(c, i) in draft.components" :key="i" class="row">
            <label class="g-field grow"><span>Name</span><input v-model="c.name" maxlength="80" :data-testid="`component-${String(i)}-name`" /></label>
            <label class="g-field small"><span>Hit points</span><input v-model.number="c.hpMax" type="number" min="1" max="10000" :data-testid="`component-${String(i)}-hp`" /></label>
            <label class="check"><input v-model="c.drives" type="checkbox" :data-testid="`component-${String(i)}-drives`" /><span>Drives it</span></label>
            <GButton :aria-label="`Remove component ${String(i + 1)}`" :data-testid="`component-${String(i)}-remove`" @click="draft.components.splice(i, 1)">×</GButton>
          </li>
        </ol>
        <GButton data-testid="component-add" @click="draft.components.push({ name: '', hpMax: 10, drives: false })">+ Component</GButton>
        <h3>Crew stations</h3>
        <ol class="rows">
          <li v-for="(s, i) in draft.stations" :key="i" class="row">
            <label class="g-field grow"><span>Name</span><input v-model="s.name" maxlength="80" :data-testid="`station-${String(i)}-name`" /></label>
            <label class="g-field small"><span>Crew it takes</span><input v-model.number="s.crew" type="number" min="0" max="200" :data-testid="`station-${String(i)}-crew`" /></label>
            <GButton :aria-label="`Remove crew station ${String(i + 1)}`" :data-testid="`station-${String(i)}-remove`" @click="draft.stations.splice(i, 1)">×</GButton>
          </li>
        </ol>
        <GButton data-testid="station-add" @click="draft.stations.push({ name: '', crew: 1 })">+ Crew station</GButton>
        <GButton type="submit" variant="primary" :disabled="draft.name.trim() === ''">Build vehicle</GButton>
      </form>
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
.vehicle {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.vehicle h2,
.vehicle h3 {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
}
.small {
  max-width: 140px;
}
.grow {
  flex: 1;
  min-width: 160px;
}
</style>
