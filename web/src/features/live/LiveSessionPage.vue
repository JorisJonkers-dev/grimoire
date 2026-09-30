<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { endSessionMutation, getCampaignOptions, listMapsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { AmbientLight, TokenKind } from '@/infrastructure/api/types.gen'
import { useLiveSession } from '@/realtime/liveSession'
import type { Coord } from '@/shared/hex'
import HexGrid from '@/shared/map/HexGrid.vue'
import { GButton } from '@/shared/ui'
import { board } from './board'
import { key } from './geometry'
import MapBoard from './MapBoard.vue'

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
const cells = computed(() => board(state.value?.session?.gridRadius ?? 0, view.value?.tokens ?? [], selected.value))
const chosen = computed(() => view.value?.tokens.find((t) => t.id === selected.value) ?? null)

function tokenTool(c: Coord) {
  const there = view.value?.tokens.find((t) => t.q === c.q && t.r === c.r)
  if (there) {
    selected.value = there.id === selected.value ? null : there.id
  } else if (chosen.value) {
    live.value?.send({ kind: 'move_token', tokenId: chosen.value.id, q: c.q, r: c.r })
  } else if (label.value.trim()) {
    live.value?.send({ kind: 'place_token', label: label.value.trim(), tokenKind: kind.value, q: c.q, r: c.r, hidden: hidden.value, darkvisionFt: darkvision.value })
    label.value = ''
  }
}
function pick(c: Coord) {
  if (!isDM.value || !live.value) return
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
      <MapBoard v-if="view?.map" :map="view.map" :view="view" :dm="isDM" :selected="selected" :title="view.map.name" @select="pick" />
      <HexGrid v-else :cells="cells" :title="`Session ${String(state.session?.number ?? '')} map`" @select="pick" />
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
            <span>{{ { tokens: 'Place or move tokens', reveal: 'Reveal', conceal: 'Conceal', wall: 'Build walls', unwall: 'Clear walls', light: 'Place or remove light' }[t] }}</span>
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
          <label class="g-field"><span>Darkvision (ft)</span><input v-model.number="darkvision" type="number" min="0" max="300" data-testid="token-darkvision" /></label>
          <label class="check"><input v-model="hidden" type="checkbox" data-testid="token-hidden" /><span>Hidden</span></label>
        </div>
        <div v-if="chosen" class="row" data-testid="selected-token">
          <span>{{ chosen.label }}{{ chosen.hidden ? ' (hidden)' : '' }}</span>
          <GButton data-testid="toggle-hidden" @click="toggleHidden()">{{ chosen.hidden ? 'Reveal' : 'Hide' }}</GButton>
          <GButton variant="danger" data-testid="remove-token" @click="remove()">Remove</GButton>
        </div>
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
.tokens {
  font-size: 14px;
  color: var(--color-text-2);
}
</style>
