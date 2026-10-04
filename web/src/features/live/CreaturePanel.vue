<script setup lang="ts">
import type { LiveCombatant, LiveToken, SessionAction, Tactics } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { TACTICS } from './console'

withDefaults(defineProps<{ token: LiveToken; combatant?: LiveCombatant; suggestion?: string; notes: SessionAction[]; noUndo?: boolean }>(), {
  combatant: undefined, suggestion: undefined, noUndo: false,
})
const emit = defineEmits<{ tactics: [value: Tactics]; use: []; undo: [seq: number] }>()
const what = (a: SessionAction) => a.kind.replaceAll('_', ' ')
</script>

<template>
  <section class="g-card creature" :aria-label="`${token.label}, a creature you run`" :data-testid="`creature-${token.label}`">
    <h2>{{ token.label }}</h2>
    <p class="stats">{{ token.hp }} / {{ token.hpMax }} hit points · AC {{ token.ac }}</p>
    <label v-if="combatant?.tactics" class="g-field">
      <span>Tactics</span>
      <select :value="combatant.tactics" data-testid="creature-tactics" @change="emit('tactics', ($event.target as HTMLSelectElement).value as Tactics)">
        <option v-for="s in TACTICS" :key="s.value" :value="s.value">{{ s.label }}</option>
      </select>
    </label>
    <div v-if="suggestion" class="suggestion">
      <p data-testid="creature-suggestion"><strong>Suggested: </strong>{{ suggestion }}</p>
      <GButton v-if="combatant?.suggestion?.attackNo !== undefined" variant="primary" data-testid="creature-use" @click="emit('use')">Use it</GButton>
    </div>
    <div data-testid="agent-notes">
      <h3>Agent notes</h3>
      <p v-if="notes.length === 0" class="none">No agent has acted on {{ token.label }}.</p>
      <ol v-else class="g-list">
        <li v-for="a in notes" :key="a.seq" class="note">
          <span><strong>#{{ a.seq }}</strong> {{ what(a) }} · via {{ a.client ?? 'an agent' }}</span>
          <GButton v-if="a.undoable && !noUndo" :aria-label="`Undo #${String(a.seq)}: ${what(a)}`" :data-testid="`note-undo-${String(a.seq)}`" @click="emit('undo', a.seq)">Undo</GButton>
        </li>
      </ol>
    </div>
  </section>
</template>

<style scoped>
.creature {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h3 {
  margin: 0;
}
section.creature > h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0;
  text-transform: none;
  color: var(--color-text);
}
h3 {
  font-family: var(--font-label);
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
.stats,
.none,
.suggestion p {
  margin: 0;
  font-size: 13px;
}
.stats,
.none {
  color: var(--color-text-2);
}
.suggestion strong {
  font-family: var(--font-label);
  font-size: 11px;
  font-weight: 400;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-gold);
}
.note {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 13px;
}
</style>
