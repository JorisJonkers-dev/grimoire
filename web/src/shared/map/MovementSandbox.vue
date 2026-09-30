<script setup lang="ts">
import { useMutation } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { previewReachMutation, previewSightMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Coord } from '@/shared/hex'
import type { GridCell } from './grid'
import HexGrid from './HexGrid.vue'
import { key, sandbox } from './sandbox'

const field = sandbox()
const start = ref<Coord>({ q: 0, r: 2 })
const target = ref<Coord | null>(null)
const mode = ref<'move' | 'look'>('move')
const reach = useMutation(previewReachMutation())
const sight = useMutation(previewSightMutation())

function preview() {
  reach.mutate({ body: { ...field, from: start.value, speedFt: 30, ...(target.value ? { to: target.value } : {}) } })
  if (target.value) sight.mutate({ body: { ...field, from: start.value, to: target.value } })
}
function select(c: Coord) {
  if (mode.value === 'move') {
    start.value = c
    target.value = null
  } else {
    target.value = c
  }
  preview()
}

const cells = computed<GridCell[]>(() => {
  const reached = new Set((reach.data.value?.hexes ?? []).filter((h) => h.canEnd).map(key))
  const path = new Set((reach.data.value?.path ?? []).map(key))
  const occupied = new Map(field.occupants.map((o) => [key(o), o.side]))
  return field.cells.map((c) => {
    const k = key(c)
    const tone =
      k === key(start.value)
        ? 'start'
        : (occupied.get(k) ??
          (c.blocked ? 'wall' : path.has(k) ? 'path' : reached.has(k) ? 'reach' : c.difficult ? 'mud' : target.value && k === key(target.value) ? 'seen' : undefined))
    return { q: c.q, r: c.r, ...(tone ? { tone } : {}) }
  })
})
</script>

<template>
  <div class="sandbox" data-testid="movement-sandbox">
    <fieldset>
      <legend>Click a hex to</legend>
      <label><input v-model="mode" type="radio" value="move" /> place the mover (30 ft)</label>
      <label><input v-model="mode" type="radio" value="look" /> aim at it</label>
    </fieldset>
    <HexGrid :cells="cells" title="Movement sandbox" @select="select" />
    <p v-if="reach.data.value" data-testid="reach-summary">
      {{ reach.data.value.hexes.filter((h) => h.canEnd).length }} hexes reachable<template v-if="reach.data.value.pathCostFt !== undefined">,
        path costs {{ reach.data.value.pathCostFt }} ft</template>
    </p>
    <p v-if="sight.data.value" data-testid="sight-summary">
      {{ sight.data.value.visible ? `Visible, ${sight.data.value.cover.replace('_', '-')} cover (+${String(sight.data.value.acBonus)} AC)` : 'Out of sight' }}
    </p>
    <p v-if="reach.isError.value || sight.isError.value" role="alert" class="g-alert">The preview could not be computed.</p>
  </div>
</template>

<style scoped>
.sandbox {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
fieldset {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin: 0;
  padding: 0;
  border: 0;
}
label {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
</style>
