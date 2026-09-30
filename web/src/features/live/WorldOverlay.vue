<script setup lang="ts">
import { computed } from 'vue'
import type { LiveWorld } from '@/infrastructure/api/types.gen'
import { type Layout, toPixel } from '@/shared/hex'

const props = defineProps<{ world: LiveWorld; layout: Layout; from: string | null }>()
const nodes = computed(() => props.world.nodes.map((n) => ({ ...n, p: toPixel(props.layout, n) })))
const byId = computed(() => new Map(nodes.value.map((n) => [n.id, n])))
const lines = computed(() =>
  props.world.routes.flatMap((r) => {
    const a = byId.value.get(r.fromNodeId)
    const b = byId.value.get(r.toNodeId)
    return a && b ? [{ id: r.id, a: a.p, b: b.p }] : []
  }),
)
const party = computed(() => (props.world.partyNodeId ? byId.value.get(props.world.partyNodeId) : undefined))
</script>

<template>
  <g class="overlay" aria-hidden="true">
    <line v-for="l in lines" :key="l.id" :x1="l.a.x" :y1="l.a.y" :x2="l.b.x" :y2="l.b.y" class="route" :data-route="l.id" />
    <g v-for="n in nodes" :key="n.id" :data-node="n.name">
      <circle :cx="n.p.x" :cy="n.p.y" :r="layout.size * 0.3" :class="['node', { 'node--from': n.id === from }]" />
      <text :x="n.p.x" :y="n.p.y - layout.size * 0.45" text-anchor="middle" class="name">{{ n.name }}</text>
    </g>
    <circle v-if="party" :cx="party.p.x" :cy="party.p.y" :r="layout.size * 0.55" class="party" data-testid="party-marker" />
  </g>
</template>

<style scoped>
.overlay {
  pointer-events: none;
}
.route {
  stroke: var(--color-gold-high);
  stroke-width: 3;
  stroke-dasharray: 8 5;
}
.node {
  fill: var(--color-gold-high);
  stroke: #000;
  stroke-width: 2;
}
.node--from {
  fill: var(--color-party);
}
.name {
  font-size: 14px;
  font-weight: 700;
  fill: #fff;
  paint-order: stroke;
  stroke: #000;
  stroke-width: 3;
}
.party {
  fill: none;
  stroke: #fff;
  stroke-width: 4;
}
</style>
