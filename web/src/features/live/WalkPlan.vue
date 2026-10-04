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
/* A short note beside the map: what the walk costs and risks, and the yes or no. */
.plan {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 2px 12px;
  max-width: min(520px, 100%);
  margin: 0;
}
.cost {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
}
ul {
  grid-column: 1;
  margin: 0;
  padding-left: 16px;
  font-size: 12px;
}
.threats {
  color: var(--color-enemy-soft);
}
.safe,
.sight {
  grid-column: 1;
  margin: 0;
  font-size: 12px;
  color: var(--color-text-2);
}
.row {
  display: flex;
  grid-row: 1 / span 3;
  grid-column: 2;
  flex-direction: row-reverse;
  gap: 6px;
}
.row :deep(.g-button) {
  min-height: 30px;
  padding: 0 12px;
  font-size: 13px;
}
@media (pointer: coarse) {
  .row :deep(.g-button) {
    min-height: 44px;
  }
}
</style>
