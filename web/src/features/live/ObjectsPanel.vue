<script setup lang="ts">
import { ref } from 'vue'
import type { LiveObject } from '@/infrastructure/api/types.gen'
import type { Outgoing } from '@/realtime/liveSession'
import { GButton } from '@/shared/ui'

const props = defineProps<{ objects: LiveObject[]; dm: boolean; user?: string }>()
const emit = defineEmits<{ send: [cmd: Outgoing] }>()
const damage = ref<Record<string, number>>({})
const state = (o: LiveObject) => {
  if (o.kind === 'trap') return o.armed === false && props.dm ? 'spent' : 'trap'
  const s = o.broken ? 'broken' : o.kind === 'lever' ? (o.open ? 'pulled' : 'up') : o.open ? 'open' : 'closed'
  return o.locked ? `${s}, locked` : s
}
const methods = [
  { method: 'key', name: 'Key' },
  { method: 'tools', name: "Thieves' tools" },
  { method: 'force', name: 'Force' },
  { method: 'knock', name: 'Knock' },
] as const
const usable = (o: LiveObject) => !o.broken && !o.locked && ['door', 'curtain', 'chest', 'lever'].includes(o.kind)
function hit(o: LiveObject) {
  const n = damage.value[o.id] ?? 0
  if (n) emit('send', { kind: 'damage_object', objectId: o.id, hpDelta: -n })
}
</script>

<template>
  <section class="g-card objects" aria-label="Objects" data-testid="objects">
    <h2>Objects</h2>
    <ul class="g-list">
      <li v-for="o in props.objects" :key="o.id" class="object" :data-testid="`object-${o.name}`">
        <span>
          {{ o.name }} · {{ state(o) }}<template v-if="dm && o.hp !== undefined"> · {{ o.hp }}/{{ o.hpMax }} HP, AC {{ o.ac }}</template>
          <template v-if="o.secret"> · secret</template>
        </span>
        <span class="row">
          <GButton v-if="usable(o) && user" :data-testid="`use-${o.name}`" @click="emit('send', { kind: 'use_object', tokenId: user, objectId: o.id })">
            {{ o.kind === 'lever' ? 'Pull' : o.open ? 'Close' : 'Open' }}
          </GButton>
          <template v-if="user && o.locked && !o.broken">
            <GButton
              v-for="m in methods"
              :key="m.method"
              :data-testid="`unlock-${m.method}-${o.name}`"
              @click="emit('send', { kind: 'unlock', tokenId: user, objectId: o.id, method: m.method })"
            >
              {{ m.name }}
            </GButton>
          </template>
          <GButton v-if="user && o.kind === 'trap' && o.armed !== false" :data-testid="`disarm-${o.name}`" @click="emit('send', { kind: 'disarm', tokenId: user, objectId: o.id })">
            Disarm
          </GButton>
          <template v-if="dm">
            <GButton v-if="o.secret" :data-testid="`find-${o.name}`" @click="emit('send', { kind: 'find_object', objectId: o.id })">Found</GButton>
            <input v-model.number="damage[o.id]" type="number" min="0" max="1000" :aria-label="`Damage to ${o.name}`" :data-testid="`damage-${o.name}`" />
            <GButton :data-testid="`hit-${o.name}`" @click="hit(o)">Damage</GButton>
            <GButton variant="danger" :data-testid="`remove-${o.name}`" @click="emit('send', { kind: 'remove_object', objectId: o.id })">Remove</GButton>
          </template>
        </span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.objects {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 16px;
}
.object,
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.row input {
  width: 5em;
}
</style>
