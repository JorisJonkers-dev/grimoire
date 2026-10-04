<script setup lang="ts">
import type { LivePath, LivePathSight } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const props = defineProps<{ path: LivePath; mover: string }>()
defineEmits<{ confirm: []; cancel: [] }>()

const covers: Record<LivePathSight['cover'], string> = { none: '', half: ', behind half cover', three_quarters: ', behind three-quarters cover', total: '' }
const sightLine = (s: LivePathSight) => (s.visible ? `${s.label} sees ${props.mover} there${covers[s.cover]}.` : `${s.label} has no line to ${props.mover} there.`)
</script>

<template>
  <section class="plan g-card" role="status" aria-label="Planned walk" data-testid="walk-preview">
    <p class="cost">Walk {{ path.costFt }} ft</p>
    <ul v-if="path.threats.length" class="threats" data-testid="walk-threats">
      <li v-for="t in path.threats" :key="t.tokenId">Leaving {{ t.label }}'s reach draws an opportunity attack.</li>
    </ul>
    <p v-else class="safe">No opportunity attacks.</p>
    <ul v-if="path.sight.length" class="sight" data-testid="walk-sight">
      <li v-for="s in path.sight" :key="s.tokenId">{{ sightLine(s) }}</li>
    </ul>
    <div class="row">
      <GButton variant="primary" aria-keyshortcuts="C" aria-label="Confirm the walk" data-testid="confirm-walk" @click="$emit('confirm')">Confirm</GButton>
      <GButton aria-keyshortcuts="Escape" data-testid="cancel-walk" @click="$emit('cancel')">Cancel</GButton>
    </div>
  </section>
</template>

<style scoped>
.plan {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 8px;
  align-self: center;
}
.cost {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
ul {
  margin: 0;
  padding-left: 18px;
}
.threats {
  color: var(--color-enemy-soft);
}
.safe,
.sight {
  margin: 0;
  color: var(--color-text-2);
}
.row {
  display: flex;
  gap: 8px;
}
</style>
