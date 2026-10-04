<script setup lang="ts">
import type { LiveCombatant } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

type Resource = 'action' | 'bonus_action' | 'reaction'
defineProps<{ combatant: LiveCombatant }>()
const emit = defineEmits<{ spend: [resource: Resource]; end: [] }>()
const pips: { key: Resource; field: 'action' | 'bonusAction' | 'reaction'; label: string }[] = [
  { key: 'action', field: 'action', label: 'Action' },
  { key: 'bonus_action', field: 'bonusAction', label: 'Bonus action' },
  { key: 'reaction', field: 'reaction', label: 'Reaction' },
]
</script>

<template>
  <section class="g-card turn" :aria-label="`${combatant.label}'s turn`" :data-testid="`turn-${combatant.label}`">
    <h2>{{ combatant.label }}'s turn</h2>
    <div class="pips">
      <GButton
        v-for="p in pips"
        :key="p.key"
        :class="['pip', { 'pip--spent': !combatant[p.field] }]"
        :disabled="!combatant[p.field]"
        :aria-label="`${p.label}: ${combatant[p.field] ? 'available, tap to spend' : 'spent'}`"
        :data-testid="`spend-${p.key}`"
        @click="emit('spend', p.key)"
      >
        {{ p.label }}
      </GButton>
      <span class="move" data-testid="movement">{{ combatant.movementFt }} / {{ combatant.speedFt }} ft</span>
    </div>
    <GButton variant="primary" aria-keyshortcuts="E" data-testid="end-turn" @click="emit('end')">End turn</GButton>
  </section>
</template>

<style scoped>
/* One line: whose turn, what is left of it, and the way out. */
.turn {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 12px;
  margin: 0;
  padding: 0;
  border: 0;
}
h2 {
  margin: 0;
  font-family: var(--font-label);
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
.pips {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}
.pip {
  min-height: 30px;
  padding: 0 10px;
  font-size: 13px;
}
.pip--spent {
  text-decoration: line-through;
}
.move {
  margin-left: 8px;
  font-size: 12px;
  color: var(--color-text-2);
}
.turn > :deep(.g-button:last-child) {
  margin-left: auto;
}
@media (pointer: coarse) {
  .pip {
    min-height: 44px;
  }
}
</style>
