<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useLiveSession } from '@/realtime/liveSession'
import { type Coord, toPixel } from '@/shared/hex'
import { gridBox } from '@/shared/map/grid'
import HexGrid from '@/shared/map/HexGrid.vue'
import { board } from './board'
import CameraView from './CameraView.vue'
import { focus } from './camera'
import { layoutOf } from './geometry'
import InitiativeRail from './InitiativeRail.vue'
import MapBoard from './MapBoard.vue'
import WorldOverlay from './WorldOverlay.vue'

const HEX = 36
const route = useRoute()
const { view: state } = useLiveSession(String(route.params.id), String(route.params.sid), 'table')
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
// The world scene shows the party's travels when it is the map they travel; any other map shows bare.
const world = computed(() => (state.view?.world && state.view.world.map.id === table.value?.worldMap?.id ? state.view.world : null))
const worldBoard = computed(() => ({ tokens: [], fog: world.value !== null, visible: world.value?.revealed ?? [], remembered: [] }))
</script>

<template>
  <main class="table" data-testid="table-display">
    <h1 class="sr-only">Table display</h1>
    <p v-if="state.connection === 'ended'" class="ended" role="status">The session has ended.</p>
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
      <InitiativeRail v-if="state.view.combat" :combat="state.view.combat" />
      <CameraView :focus="centre" :zoom="(table?.zoomPct ?? 100) / 100" :ping="ping">
        <MapBoard v-if="state.view.map" :map="state.view.map" :view="state.view" :area="state.view.area?.hexes" title="The table" />
        <HexGrid v-else :cells="cells" :size="HEX" title="The table" />
      </CameraView>
    </div>
  </main>
</template>

<style scoped>
.table {
  display: grid;
  place-items: center;
  min-height: 100vh;
  background: var(--color-ground);
  overflow: hidden;
}
.stage {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  max-width: 100%;
  padding: 12px;
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
