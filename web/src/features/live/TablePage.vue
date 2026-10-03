<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useLiveSession } from '@/realtime/liveSession'
import { type Coord, toPixel } from '@/shared/hex'
import { gridBox } from '@/shared/map/grid'
import DiceHost from '@/features/dice/DiceHost.vue'
import { throwDice } from '@/features/dice/stage'
import HexGrid from '@/shared/map/HexGrid.vue'
import { board } from './board'
import CameraView from './CameraView.vue'
import { checkLine } from './checks'
import LiveRoll from './LiveRoll.vue'
import { focus } from './camera'
import { layoutOf } from './geometry'
import { BANNER_MS } from './motion'
import RosterStrip from './RosterStrip.vue'
import MapBoard from './MapBoard.vue'
import WorldOverlay from './WorldOverlay.vue'

const HEX = 36
const route = useRoute()
const campaignId = String(route.params.id)
const { view: state } = useLiveSession(campaignId, String(route.params.sid), 'table')
const table = computed(() => state.view?.table)
const cells = computed(() =>
  state.session && state.view
    ? board(state.session.gridRadius, state.view.tokens, null, [], { surfaces: state.view.surfaces, area: state.view.area?.hexes })
    : [],
)
// Where a hex's centre sits inside the drawn board, so the camera can look at it.
const px = (c: Coord) => {
  if (state.view?.map) return toPixel(layoutOf(state.view.map), c)
  const box = gridBox(cells.value, HEX)
  const p = toPixel({ size: HEX, origin: { x: 0, y: 0 } }, c)
  return { x: p.x - box.x, y: p.y - box.y }
}
const centre = computed(() => (state.view ? px(focus(state.view)) : { x: 0, y: 0 }))
const ping = computed(() => (state.ping ? { ...px(state.ping), n: state.ping.n } : null))
// The latest Encounter Check: an open one shows its roll here while it waits.
const latest = computed(() => state.view?.checks?.at(-1))
// The world scene shows the party's travels when it is the map they travel; any other map shows bare.
const world = computed(() => (state.view?.world && state.view.world.map.id === table.value?.worldMap?.id ? state.view.world : null))
// Whose turn it is shows as each turn starts, then leaves; it names only creatures the table sees.
const turnOf = ref('')
let turnTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => state.turn?.n,
  () => {
    const names = (state.turn?.tokenIds ?? []).flatMap((id) => state.view?.tokens.find((t) => t.id === id)?.label ?? []).sort((a, b) => a.localeCompare(b))
    if (names.length === 0) return
    clearTimeout(turnTimer)
    turnOf.value = `${names.join(' and ')}'s turn`
    turnTimer = setTimeout(() => { turnOf.value = '' }, BANNER_MS)
  },
)
onBeforeUnmount(() => { clearTimeout(turnTimer) })
// A player's roll is thrown on the table's dice stage as it resolves.
watch(
  () => state.rolls,
  () => {
    if (state.roll) throwDice(state.roll, state.roll.id)
  },
)
const signed = (n: number) => `${n < 0 ? '−' : '+'} ${String(Math.abs(n))}`
const worldBoard = computed(() => ({ tokens: [], fog: world.value !== null, visible: world.value?.revealed ?? [], remembered: [] }))
</script>

<template>
  <main class="table" data-testid="table-display">
    <h1 class="sr-only">Table display</h1>
    <aside v-if="latest && !table?.blackout" class="check" aria-label="Encounter check" data-testid="table-check">
      <LiveRoll v-if="latest.status === 'pending' && latest.rollId" :key="latest.rollId" :campaign-id="campaignId" :roll-id="latest.rollId" />
      <p v-else>{{ checkLine(latest) }}</p>
    </aside>
    <template v-if="state.connection !== 'ended' && state.view && !table?.blackout">
      <p v-if="turnOf" class="turn" role="status" data-testid="table-turn">{{ turnOf }}</p>
      <aside v-if="state.roll" class="roll" aria-label="The last roll" data-testid="table-roll">
        <span class="who">{{ state.roll.roller }} · {{ state.roll.purpose }}</span>
        <span
          v-for="(d, i) in state.roll.dice"
          :key="i"
          :class="['die', { 'die--dropped': !d.kept }]"
          role="img"
          :aria-label="`d${String(d.faces)}: ${String(d.value)}${d.kept ? '' : ', dropped'}`"
        >{{ d.value }}</span>
        <span v-if="state.roll.modifier" class="mod">{{ signed(state.roll.modifier) }}</span>
        <strong class="total">= {{ state.roll.total }}</strong>
      </aside>
      <p v-if="table?.caption" class="caption" data-testid="table-caption">{{ table.caption }}</p>
    </template>
    <p v-if="state.connection === 'ended'" class="ended" role="status" data-testid="session-ended">The session has ended.</p>
    <p v-else-if="!state.session || !state.view" class="ended" role="status">Waiting for the table…</p>
    <div v-else-if="table?.blackout" class="blackout" role="img" aria-label="The table is dark" data-testid="blackout"></div>
    <section v-else-if="table?.scene === 'title' || table?.scene === 'handout'" :class="['card', table.scene]" :data-testid="`scene-${table.scene}`">
      <h2>{{ table.title }}</h2>
      <p>{{ table.body }}</p>
    </section>
    <div v-else-if="table?.scene === 'world' && table.worldMap" class="stage" data-testid="scene-world">
      <MapBoard :map="world?.map ?? table.worldMap" :view="worldBoard" :title="table.worldMap.name">
        <template v-if="world" #default="{ layout }">
          <WorldOverlay :world="world" :layout="layout" :from="null" />
        </template>
      </MapBoard>
    </div>
    <div v-else class="stage">
      <RosterStrip
        v-if="(state.view.roster?.length ?? 0) > 0 || state.view.combat"
        class="table-roster"
        :roster="state.view.roster ?? []"
        :combat="state.view.combat"
        :tokens="state.view.tokens"
        :reveal="state.reveal"
      />
      <CameraView :focus="centre" :zoom="(table?.zoomPct ?? 100) / 100" :ping="ping">
        <MapBoard v-if="state.view.map" :map="state.view.map" :view="state.view" :area="state.view.area?.hexes" title="The table" />
        <HexGrid v-else :cells="cells" :size="HEX" title="The table" />
      </CameraView>
    </div>
    <DiceHost v-if="!table?.blackout" />
  </main>
</template>

<style scoped>
.turn,
.roll,
.caption {
  position: fixed;
  z-index: 3;
  margin: 0;
  border-radius: 14px;
  color: var(--color-text);
  background: color-mix(in srgb, var(--color-surface) 90%, transparent);
  box-shadow: 0 8px 28px rgb(0 0 0 / 45%);
}
.turn {
  top: 22%;
  left: 50%;
  padding: 14px 40px;
  border: 2px solid var(--color-gold-high);
  font-family: var(--font-display);
  font-size: clamp(28px, 4vw, 64px);
  color: var(--color-gold-high);
  transform: translateX(-50%);
}
/* The roll keeps to the top corner and the caption to the bottom, so neither covers the other. */
.roll {
  top: 24px;
  right: 24px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  max-width: 26vw;
  padding: 14px 20px;
  font-size: clamp(18px, 1.6vw, 30px);
}
.who {
  flex: 1 0 100%;
  color: var(--color-text-2);
}
.die {
  display: grid;
  place-items: center;
  min-width: 2.2em;
  min-height: 2.2em;
  border: 2px solid var(--color-gold-high);
  border-radius: 10px;
  font-family: var(--font-display);
  font-weight: 700;
}
.die--dropped {
  border-color: var(--color-line);
  color: var(--color-text-2);
  text-decoration: line-through;
}
.total {
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.caption {
  bottom: 24px;
  left: 50%;
  max-width: 70vw;
  padding: 14px 28px;
  font-size: clamp(20px, 2vw, 40px);
  text-align: center;
  transform: translateX(-50%);
}
.table {
  display: grid;
  place-items: center;
  min-height: 100vh;
  background: var(--color-ground);
  overflow: hidden;
}
.stage {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  max-width: 100%;
  padding: 12px;
}
.table-roster {
  position: absolute;
  top: 12px;
  left: 50%;
  z-index: 2;
  max-width: calc(100% - 24px);
  transform: translateX(-50%);
}
.check {
  position: fixed;
  top: 12px;
  right: 12px;
  z-index: 2;
  max-width: 420px;
  font-size: 20px;
}
.blackout {
  position: fixed;
  inset: 0;
  background: #000;
}
.card {
  max-width: 900px;
  padding: 32px;
  text-align: center;
}
.card h2 {
  margin: 0 0 16px;
  font-family: var(--font-display);
  font-size: 56px;
  color: var(--color-gold-high);
}
.card p {
  font-size: 28px;
  white-space: pre-line;
}
.handout h2 {
  font-size: 40px;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.ended {
  font-family: var(--font-display);
  font-size: 28px;
  color: var(--color-text-2);
}
</style>
