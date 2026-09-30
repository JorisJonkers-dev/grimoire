<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { endSessionMutation, getCampaignOptions, listCharactersOptions, listMapsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { rollRest } from '@/infrastructure/api/sdk.gen'
import type { AmbientLight, LiveCombatant, LiveCombatantSetup, LiveToken, TokenKind } from '@/infrastructure/api/types.gen'
import { useLiveSession } from '@/realtime/liveSession'
import type { Coord } from '@/shared/hex'
import HexGrid from '@/shared/map/HexGrid.vue'
import { GButton } from '@/shared/ui'
import { board, describe } from './board'
import { key } from './geometry'
import AttackPreview from './AttackPreview.vue'
import Hotbar from './Hotbar.vue'
import InitiativeRail from './InitiativeRail.vue'
import LiveRoll from './LiveRoll.vue'
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
const characters = useQuery(computed(() => ({ ...listCharactersOptions({ path: { campaignId } }), enabled: isDM.value })))
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
const monster = ref('')
const character = ref('')
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
const tokenById = (id: string) => view.value?.tokens.find((t) => t.id === id)
const pending = computed(() => combat.value?.attack ?? null)
const aiming = ref<{ tokenId: string; attackNo: number } | null>(null)
const bars = computed(() =>
  turns.value.flatMap((c) => {
    const token = tokenById(c.tokenId)
    return token?.attacks?.length ? [{ c, token }] : []
  }),
)
const blockedFor = (c: LiveCombatant) => (pending.value ? 'An attack is waiting on its roll.' : c.action ? '' : 'The action is used this turn.')
function arm(token: LiveToken, attackNo: number) {
  const same = aiming.value?.tokenId === token.id && aiming.value.attackNo === attackNo
  aiming.value = same ? null : { tokenId: token.id, attackNo }
}
const preview = computed(() => {
  const p = state.value?.preview
  return p && p.tokenId === aiming.value?.tokenId && p.attackNo === aiming.value.attackNo ? p : null
})
function confirmAttack(p: { tokenId: string; attackNo: number; targetId: string }) {
  live.value?.send({ kind: 'attack', tokenId: p.tokenId, attackNo: p.attackNo, targetId: p.targetId })
  aiming.value = null
}
// Whoever throws the attacker's dice sees the attack's Roll Card: its Controller, or the DM.
const attackRoll = computed(() => {
  const a = pending.value
  const attacker = a ? tokenById(a.attackerId) : undefined
  if (!a || !attacker) return null
  return (isDM.value ? !attacker.controllerId : attacker.controllerId === campaign.data.value?.me.id) ? a.rollId : null
})
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
  } else if (label.value.trim() || monster.value.trim() || character.value) {
    live.value?.send({
      kind: 'place_token', label: label.value.trim(), tokenKind: kind.value, q: c.q, r: c.r, hidden: hidden.value, darkvisionFt: darkvision.value,
      ...(controller.value ? { controllerId: controller.value } : {}),
      ...(character.value ? { characterId: character.value } : monster.value.trim() ? { monsterSlug: monster.value.trim() } : {}),
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
  const target = tokenAt(c)
  if (aiming.value && target) {
    live.value.send({ kind: 'preview_attack', ...aiming.value, targetId: target.id })
    return
  }
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
        <LiveRoll v-for="c in toRoll" :key="c.rollId" :campaign-id="campaignId" :roll-id="c.rollId" />
      </section>
      <TurnPanel
        v-for="c in turns"
        :key="c.id"
        :combatant="c"
        @spend="(r) => live?.send({ kind: 'spend', combatantId: c.id, resource: r })"
        @end="live?.send({ kind: 'end_turn', combatantId: c.id })"
      />
      <Hotbar
        v-for="b in bars"
        :key="b.token.id"
        :token="b.token"
        :armed="aiming?.tokenId === b.token.id ? aiming.attackNo : null"
        :blocked="blockedFor(b.c)"
        @arm="(n) => arm(b.token, n)"
      />
      <AttackPreview
        v-if="preview"
        :preview="preview"
        :target="tokenById(preview.targetId)?.label ?? 'the target'"
        @confirm="confirmAttack(preview)"
        @cancel="aiming = null"
      />
      <p v-if="pending" role="status" class="walk" data-testid="pending-attack">
        {{ pending.name }}{{ pending.critical ? ' (critical)' : '' }}: waiting for the {{ pending.stage === 'to_hit' ? 'attack' : 'damage' }} roll.
      </p>
      <LiveRoll v-if="attackRoll" :key="attackRoll" :campaign-id="campaignId" :roll-id="attackRoll" />
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
          <label class="g-field">
            <span>Monster</span>
            <input v-model="monster" placeholder="goblin" maxlength="80" data-testid="token-monster" />
          </label>
          <label class="g-field">
            <span>Character</span>
            <select v-model="character" data-testid="token-character">
              <option value="">None</option>
              <option v-for="ch in characters.data.value ?? []" :key="ch.id" :value="ch.id">{{ ch.name }}</option>
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
          <GButton data-testid="undo-damage" @click="live?.send({ kind: 'undo_damage' })">Undo last damage</GButton>
          <GButton v-if="combat" variant="danger" data-testid="end-combat" @click="live?.send({ kind: 'end_combat' })">End combat</GButton>
          <GButton v-else-if="!choosing" data-testid="choose-combatants" :disabled="!view?.tokens.length" @click="choosing = true">
            Start combat…
          </GButton>
        </div>
        <StartCombat v-if="choosing && !combat" :tokens="view?.tokens ?? []" @start="startCombat" />
        <GButton variant="danger" data-testid="end-session" @click="endSession()">End session</GButton>
      </section>
      <ul class="g-list tokens" aria-label="Tokens in view">
        <li v-for="t in view?.tokens ?? []" :key="t.id">{{ describe(t) }} · {{ t.kind }}</li>
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
