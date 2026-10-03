<script setup lang="ts">
import { computed } from 'vue'
import type { LiveWorld } from '@/infrastructure/api/types.gen'
import { type Coord, type Layout, toPixel } from '@/shared/hex'

const props = defineProps<{ world: LiveWorld; layout: Layout; from: string | null; measured: Coord[] }>()
const nodes = computed(() => props.world.nodes.map((n) => ({ ...n, p: toPixel(props.layout, n) })))
const byId = computed(() => new Map(nodes.value.map((n) => [n.id, n])))
const lines = computed(() =>
  props.world.routes.flatMap((r) => {
    const a = byId.value.get(r.fromNodeId)
    const b = byId.value.get(r.toNodeId)
    return a && b ? [{ id: r.id, a: a.p, b: b.p }] : []
  }),
)
// The route being measured: a point at each waypoint, and a line through them once there are two.
const waypoints = computed(() => props.measured.map((c) => toPixel(props.layout, c)))
const drawn = computed(() => waypoints.value.map((p) => `${String(Number(p.x.toFixed(1)))},${String(Number(p.y.toFixed(1)))}`).join(' '))
const party = computed(() => (props.world.partyNodeId ? byId.value.get(props.world.partyNodeId) : undefined))
</script>

<template>
  <g class="overlay" aria-hidden="true">
    <line v-for="l in lines" :key="l.id" :x1="l.a.x" :y1="l.a.y" :x2="l.b.x" :y2="l.b.y" class="route" :data-route="l.id" />
    <g v-for="n in nodes" :key="n.id" :data-node="n.name">
      <!-- The outline of the local Map that lies there: solid once the party has found it. -->
      <rect
        v-if="n.mapId"
        :x="n.p.x - layout.size * 0.5"
        :y="n.p.y - layout.size * 0.5"
        :width="layout.size"
        :height="layout.size"
        :class="['local', { 'local--found': n.found }]"
        :data-found-map="n.found ? n.name : undefined"
        :data-local-map="n.found ? undefined : n.name"
      />
      <circle :cx="n.p.x" :cy="n.p.y" :r="layout.size * 0.3" :class="['node', { 'node--from': n.id === from, 'node--secret': n.secret }]" :data-secret="n.secret ? n.name : undefined" />
      <text :x="n.p.x" :y="n.p.y - layout.size * 0.6" text-anchor="middle" class="name">{{ n.secret ? `${n.name} (secret)` : n.name }}</text>
    </g>
    <polyline v-if="waypoints.length > 1" :points="drawn" class="measure" data-testid="measure-line" />
    <circle v-for="(p, i) in waypoints" :key="i" :cx="p.x" :cy="p.y" :r="layout.size * 0.16" class="waypoint" :data-waypoint="i" />
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
.node--secret {
  fill: var(--color-enemy);
  stroke-dasharray: 3 2;
}
.local {
  fill: none;
  stroke: #fff;
  stroke-width: 2;
  stroke-dasharray: 4 3;
  opacity: 0.6;
}
.local--found {
  stroke: var(--color-gold-high);
  stroke-dasharray: none;
  opacity: 1;
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
.measure {
  fill: none;
  stroke: #fff;
  stroke-width: 3;
  stroke-linejoin: round;
  paint-order: stroke;
}
.waypoint {
  fill: #fff;
  stroke: #000;
  stroke-width: 2;
}
.party {
  fill: none;
  stroke: #fff;
  stroke-width: 4;
}
</style>
