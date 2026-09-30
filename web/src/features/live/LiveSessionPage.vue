<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { endSessionMutation, getCampaignOptions, listMapsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { rollRest } from '@/infrastructure/api/sdk.gen'
import type { AmbientLight, LiveCombatant, LiveCombatantSetup, TokenKind } from '@/infrastructure/api/types.gen'
import { useLiveSession } from '@/realtime/liveSession'
import type { Coord } from '@/shared/hex'
import HexGrid from '@/shared/map/HexGrid.vue'
import { GButton } from '@/shared/ui'
import { board } from './board'
import { key } from './geometry'
import InitiativeRail from './InitiativeRail.vue'
import InitiativeRoll from './InitiativeRoll.vue'
import MapBoard from './MapBoard.vue'
import StartCombat from './StartCombat.vue'
import TurnPanel from './TurnPanel.vue'

type Tool = 'tokens' | 'reveal' | 'conceal' | 'wall' | 'unwall' | 'light'

const route = useRoute()
const router = useRouter()
const campaignId = String(route.params.id)
const sessionId = String(route.params.sid)
const campaign = useQuery({ ...getCampaignOptions({ path: { campaignId } }), retry: false })
const isDM = computed(() => campaign.data.value?.myRole === 'dm')
const maps = useQuery(computed(() => ({ ...listMapsOptions({ path: { campaignId } }), enabled: isDM.value })))
const live = shallowRef<ReturnType<typeof useLiveSession> | null>(null)
const state = computed(() => live.value?.view)
const view = computed(() => state.value?.view ?? null)

watch(
  () => campaign.data.value,
  (c) => {
    if (c && !live.value) live.value = useLiveSession(campaignId, sessionId, c.myRole === 'dm' ? 'dm' : 'party')
  },
  { immediate: true },
)

const tool = ref<Tool>('tokens')
const selected = ref<string | null>(null)
const label = ref('')
const kind = ref<TokenKind>('enemy')
const hidden = ref(false)
const darkvision = ref(0)
const brightFt = ref(20)
const dimFt = ref(40)
const mapChoice = ref('')
const controller = ref('')
const players = computed(() => campaign.data.value?.members.filter((m) => m.role === 'player') ?? [])
const walkPath = computed(() => state.value?.path?.hexes ?? [])
const cells = computed(() => board(state.value?.session?.gridRadius ?? 0, view.value?.tokens ?? [], selected.value, walkPath.value))
const chosen = computed(() => view.value?.tokens.find((t) => t.id === selected.value) ?? null)
const mine = computed(() => view.value?.tokens.filter((t) => t.controllerId && t.controllerId === campaign.data.value?.me.id) ?? [])
const walker = computed(() => (isDM.value ? chosen.value : (mine.value.find((t) => t.id === selected.value) ?? mine.value[0] ?? null)))
const combat = computed(() => view.value?.combat ?? null)
const playable = (c: LiveCombatant) => isDM.value || (c.controllerId !== undefined && c.controllerId === campaign.data.value?.me.id)
const turns = computed(() => combat.value?.combatants.filter((c) => c.acting && playable(c)) ?? [])
const toRoll = computed(() =>
  combat.value?.status === 'rolling'
    ? combat.value.combatants.filter((c) => c.initiative === undefined && (isDM.value ? !c.controllerId : playable(c)))
    : [],
)
const choosing = ref(false)
function startCombat(combatants: LiveCombatantSetup[]) {
  live.value?.send({ kind: 'start_combat', combatants })
  choosing.value = false
}
async function rollAll() {
  for (const c of toRoll.value) await rollRest({ path: { campaignId, rollId: c.rollId } }).catch(() => undefined)
}
const tokenAt = (c: Coord) => view.value?.tokens.find((t) => t.q === c.q && t.r === c.r)

// The first tap on a hex previews the walk there; a second tap on the same hex walks it.
function walkTo(c: Coord) {
  const t = walker.value
  if (!t) return
  const p = state.value?.path
  const end = p?.hexes.at(-1)
  const confirmed = p?.tokenId === t.id && end?.q === c.q && end.r === c.r
  live.value?.send({ kind: confirmed ? 'walk' : 'plan_walk', tokenId: t.id, q: c.q, r: c.r })
}
function tokenTool(c: Coord) {
  const there = tokenAt(c)
  if (there) {
    selected.value = there.id === selected.value ? null : there.id
  } else if (chosen.value) {
    walkTo(c)
  } else if (label.value.trim()) {
    live.value?.send({
      kind: 'place_token', label: label.value.trim(), tokenKind: kind.value, q: c.q, r: c.r, hidden: hidden.value, darkvisionFt: darkvision.value,
      ...(controller.value ? { controllerId: controller.value } : {}),
    })
    label.value = ''
  }
}
function explore(c: Coord) {
  const there = tokenAt(c)
  if (there && mine.value.some((t) => t.id === there.id)) selected.value = there.id
  else walkTo(c)
}
function pick(c: Coord) {
  if (!live.value) return
  if (!isDM.value) {
    explore(c)
    return
  }
  switch (tool.value) {
    case 'tokens':
      tokenTool(c)
      return
    case 'reveal':
    case 'conceal':
      live.value.send({ kind: 'reveal_hexes', hexes: [c], on: tool.value === 'reveal' })
      return
    case 'wall':
    case 'unwall':
      live.value.send({ kind: 'set_walls', hexes: [c], on: tool.value === 'wall' })
      return
    default: {
      const lit = view.value?.lights?.find((l) => key(l) === key(c))
      if (lit) live.value.send({ kind: 'remove_light', lightId: lit.id })
      else live.value.send({ kind: 'place_light', q: c.q, r: c.r, brightFt: brightFt.value, dimFt: dimFt.value })
    }
  }
}
function useMap() {
  live.value?.send(mapChoice.value ? { kind: 'set_map', mapId: mapChoice.value } : { kind: 'set_map' })
}
function setAmbient(a: AmbientLight) {
  live.value?.send({ kind: 'set_ambient', ambient: a })
}
function toggleHidden() {
  if (chosen.value) live.value?.send({ kind: 'set_token_hidden', tokenId: chosen.value.id, hidden: !chosen.value.hidden })
}
function remove() {
  if (chosen.value) live.value?.send({ kind: 'remove_token', tokenId: chosen.value.id })
  selected.value = null
}
const end = useMutation(endSessionMutation())
function endSession() {
  end.mutate({ path: { campaignId, sessionId } }, { onSuccess: () => void router.push({ name: 'campaign', params: { id: campaignId } }) })
}
const status = computed(() => ({ connecting: 'Connecting…', open: 'Live', reconnecting: 'Reconnecting…', ended: 'Session ended' })[state.value?.connection ?? 'connecting'])
</script>

<template>
  <main class="g-page live">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <p v-if="campaign.isError.value" role="alert" class="g-alert" data-testid="live-missing">This session is not available to you.</p>
    <template v-else-if="state">
      <header class="head">
        <h1>Session {{ state.session?.number ?? '' }}</h1>
        <span class="g-tag" :class="`conn--${state.connection}`" data-testid="connection" role="status">{{ status }}</span>
        <RouterLink :to="{ name: 'table', params: { id: campaignId, sid: sessionId } }" class="table-link">Table display</RouterLink>
      </header>
      <p v-if="state.rejection" role="alert" class="g-alert" data-testid="rejection">{{ state.rejection }}</p>
      <InitiativeRail v-if="combat" :combat="combat" />
      <p v-if="!isDM && turns.length > 0" role="status" class="banner" data-testid="your-turn">Your turn</p>
      <section v-if="toRoll.length > 0" class="rolls" aria-label="Initiative to roll">
        <GButton v-if="isDM && toRoll.length > 1" data-testid="roll-all" @click="rollAll()">Roll every initiative for me</GButton>
        <InitiativeRoll v-for="c in toRoll" :key="c.rollId" :campaign-id="campaignId" :roll-id="c.rollId" />
      </section>
      <TurnPanel
        v-for="c in turns"
        :key="c.id"
        :combatant="c"
        @spend="(r) => live?.send({ kind: 'spend', combatantId: c.id, resource: r })"
        @end="live?.send({ kind: 'end_turn', combatantId: c.id })"
      />
      <MapBoard v-if="view?.map" :map="view.map" :view="view" :dm="isDM" :selected="selected" :path="walkPath" :title="view.map.name" @select="pick" />
      <HexGrid v-else :cells="cells" :title="`Session ${String(state.session?.number ?? '')} map`" @select="pick" />
      <p v-if="state.path" role="status" class="walk" data-testid="walk-preview">
        Walk {{ state.path.costFt }} ft. Tap the same hex again to go.
      </p>
      <p v-else-if="!isDM && walker" class="walk" data-testid="walker">Tap a hex to walk {{ walker.label }} there.</p>
      <section v-if="isDM" class="g-card controls" data-testid="dm-controls">
        <div class="row">
          <label class="g-field grow">
            <span>Map</span>
            <select v-model="mapChoice" data-testid="map-choice">
              <option value="">No map (open grid)</option>
              <option v-for="m in maps.data.value ?? []" :key="m.id" :value="m.id">{{ m.name }}</option>
            </select>
          </label>
          <GButton data-testid="use-map" @click="useMap()">Use map</GButton>
          <RouterLink :to="{ name: 'maps', params: { id: campaignId } }" class="manage">Manage maps</RouterLink>
        </div>
        <fieldset class="tools">
          <legend>Tap the map to</legend>
          <label v-for="t in (['tokens', 'reveal', 'conceal', 'wall', 'unwall', 'light'] as const)" :key="t" class="tool">
            <input v-model="tool" type="radio" :value="t" :data-testid="`tool-${t}`" />
            <span>{{ { tokens: 'Place or walk tokens', reveal: 'Reveal', conceal: 'Conceal', wall: 'Build walls', unwall: 'Clear walls', light: 'Place or remove light' }[t] }}</span>
          </label>
        </fieldset>
        <div v-if="view?.map" class="row">
          <span>Ambient light</span>
          <GButton v-for="a in (['bright', 'dim', 'dark'] as const)" :key="a" :data-testid="`ambient-${a}`" @click="setAmbient(a)">
            {{ a }}{{ view.ambient === a ? ' ✓' : '' }}
          </GButton>
        </div>
        <div v-if="tool === 'light'" class="row">
          <label class="g-field"><span>Bright (ft)</span><input v-model.number="brightFt" type="number" min="0" max="600" /></label>
          <label class="g-field"><span>Dim (ft)</span><input v-model.number="dimFt" type="number" min="0" max="600" /></label>
        </div>
        <div v-if="tool === 'tokens'" class="row">
          <label class="g-field grow"><span>Name</span><input v-model="label" maxlength="40" data-testid="token-label" /></label>
          <label class="g-field">
            <span>Kind</span>
            <select v-model="kind" data-testid="token-kind">
              <option value="party">Party</option>
              <option value="enemy">Enemy</option>
              <option value="npc">NPC</option>
              <option value="object">Object</option>
            </select>
          </label>
          <label class="g-field">
            <span>Controlled by</span>
            <select v-model="controller" data-testid="token-controller">
              <option value="">Only the DM</option>
              <option v-for="m in players" :key="m.id" :value="m.id">{{ m.displayName }}</option>
            </select>
          </label>
          <label class="g-field"><span>Darkvision (ft)</span><input v-model.number="darkvision" type="number" min="0" max="300" data-testid="token-darkvision" /></label>
          <label class="check"><input v-model="hidden" type="checkbox" data-testid="token-hidden" /><span>Hidden</span></label>
        </div>
        <div v-if="chosen" class="row" data-testid="selected-token">
          <span>{{ chosen.label }}{{ chosen.hidden ? ' (hidden)' : '' }}</span>
          <GButton data-testid="toggle-hidden" @click="toggleHidden()">{{ chosen.hidden ? 'Reveal' : 'Hide' }}</GButton>
          <GButton variant="danger" data-testid="remove-token" @click="remove()">Remove</GButton>
        </div>
        <div class="row">
          <GButton v-if="combat" variant="danger" data-testid="end-combat" @click="live?.send({ kind: 'end_combat' })">End combat</GButton>
          <GButton v-else-if="!choosing" data-testid="choose-combatants" :disabled="!view?.tokens.length" @click="choosing = true">
            Start combat…
          </GButton>
        </div>
        <StartCombat v-if="choosing && !combat" :tokens="view?.tokens ?? []" @start="startCombat" />
        <GButton variant="danger" data-testid="end-session" @click="endSession()">End session</GButton>
      </section>
      <ul class="g-list tokens" aria-label="Tokens in view">
        <li v-for="t in view?.tokens ?? []" :key="t.id">{{ t.label }} · {{ t.kind }}{{ t.hidden ? ' · hidden' : '' }}</li>
      </ul>
    </template>
    <p v-else>Opening the session…</p>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.conn--open {
  border-color: var(--color-success);
  color: var(--color-success);
}
.conn--reconnecting,
.conn--ended {
  border-color: var(--color-enemy);
  color: var(--color-enemy-soft);
}
.table-link {
  margin-left: auto;
  color: var(--color-gold-high);
}
.controls {
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
.grow {
  flex: 1 1 160px;
}
.manage {
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  color: var(--color-gold-high);
}
.tools {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
  margin: 0;
  padding: 0;
  border: 0;
}
.tools legend {
  margin-bottom: 4px;
}
.tool,
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
.banner {
  margin: 0;
  font-family: var(--font-display);
  font-size: 22px;
  color: var(--color-gold-high);
}
.rolls {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.walk {
  margin: 0;
  color: var(--color-gold-high);
}
.tokens {
  font-size: 14px;
  color: var(--color-text-2);
}
</style>
