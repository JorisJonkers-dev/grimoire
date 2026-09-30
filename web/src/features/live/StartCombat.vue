<script setup lang="ts">
import { reactive } from 'vue'
import type { LiveCombatantSetup, LiveToken } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ tokens: LiveToken[] }>()
const emit = defineEmits<{ start: [combatants: LiveCombatantSetup[]] }>()
const picks = reactive(props.tokens.map((token) => ({ token, on: token.kind !== 'object', initiativeBonus: 0, speedFt: 30 })))
function start() {
  const chosen = picks.filter((p) => p.on).map((p) => ({ tokenId: p.token.id, initiativeBonus: p.initiativeBonus, speedFt: p.speedFt }))
  if (chosen.length > 0) emit('start', chosen)
}
</script>

<template>
  <form class="g-card start" aria-label="Start combat" data-testid="start-combat" @submit.prevent="start()">
    <h2>Who fights?</h2>
    <table>
      <thead>
        <tr><th scope="col">Token</th><th scope="col">Initiative bonus</th><th scope="col">Speed (ft)</th></tr>
      </thead>
      <tbody>
        <tr v-for="p in picks" :key="p.token.id">
          <td>
            <label class="check"><input v-model="p.on" type="checkbox" :data-testid="`fights-${p.token.label}`" />{{ p.token.label }}</label>
          </td>
          <td><input v-model.number="p.initiativeBonus" type="number" min="-10" max="20" :aria-label="`${p.token.label} initiative bonus`" :data-testid="`bonus-${p.token.label}`" /></td>
          <td><input v-model.number="p.speedFt" type="number" min="0" max="120" step="5" :aria-label="`${p.token.label} speed`" :data-testid="`speed-${p.token.label}`" /></td>
        </tr>
      </tbody>
    </table>
    <GButton type="submit" variant="primary" data-testid="begin-combat">Roll initiative</GButton>
  </form>
</template>

<style scoped>
.start {
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow-x: auto;
}
h2 {
  margin: 0;
  font-size: 18px;
}
table {
  border-collapse: collapse;
}
th {
  font-size: 13px;
  font-weight: 600;
  text-align: left;
  color: var(--color-text-2);
}
td {
  padding: 2px 6px 2px 0;
}
td input[type='number'] {
  width: 5em;
  min-height: 40px;
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
</style>
