<script setup lang="ts">
import { computed } from 'vue'
import type { LiveCombat, LiveToken } from '@/infrastructure/api/types.gen'
import { StatusIcon } from '@/shared/ui'
import { initials } from './board'
import { effectLabel } from './conditions'

const props = withDefaults(defineProps<{ combat: LiveCombat; tokens?: LiveToken[] }>(), { tokens: () => [] })
const effectsOf = (tokenId: string) => props.tokens.find((t) => t.id === tokenId)?.effects ?? []
const dyingOf = (tokenId: string) => {
  const d = props.tokens.find((t) => t.id === tokenId)?.dying
  if (!d) return ''
  if (d.dead) return 'Dead'
  return d.stable ? 'Stable' : `Dying ${String(d.successes)}✓ ${String(d.failures)}✗`
}
const tied = computed(() => {
  const ranks = props.combat.combatants.map((c) => c.rank)
  return (rank?: number) => ranks.filter((r) => r === rank).length > 1
})
</script>

<template>
  <section class="rail" aria-label="Initiative" data-testid="initiative-rail">
    <h2 class="round">{{ combat.status === 'rolling' ? 'Rolling initiative' : `Round ${String(combat.round)}` }}</h2>
    <ol>
      <li
        v-for="c in combat.combatants"
        :key="c.id"
        :class="['slot', `slot--${c.kind}`, { 'slot--acting': c.acting, 'slot--done': c.done }]"
        :aria-current="c.acting ? 'step' : undefined"
        :data-testid="`rail-${c.label}`"
      >
        <span class="badge" aria-hidden="true">{{ initials(c.label) }}</span>
        <span class="name">{{ c.label }}</span>
        <span class="init">
          <template v-if="c.initiative !== undefined">{{ c.initiative }}<template v-if="tied(c.rank)"> · tied</template></template>
          <template v-else>rolling…</template>
        </span>
        <span v-if="c.surprised" class="surprised" data-testid="surprised">Surprised</span>
        <span v-if="dyingOf(c.tokenId)" class="fallen" data-testid="fallen">{{ dyingOf(c.tokenId) }}</span>
        <span v-if="effectsOf(c.tokenId).length" class="statuses" data-testid="statuses">
          <StatusIcon v-for="e in effectsOf(c.tokenId)" :key="e.id" :slug="e.slug" :label="effectLabel(e)" :size="12" />
        </span>
        <span v-if="c.acting" class="sr-only">acting now</span>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.fallen {
  font-size: 12px;
  color: var(--color-enemy-soft);
}
.statuses {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 2px;
}
.surprised {
  font-size: 12px;
  color: var(--color-enemy-soft);
}
.rail {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.round {
  margin: 0;
  font-size: 15px;
  color: var(--color-text-2);
}
ol {
  display: flex;
  gap: 8px;
  margin: 0;
  padding: 0 0 4px;
  overflow-x: auto;
  list-style: none;
}
.slot {
  display: grid;
  justify-items: center;
  gap: 2px;
  min-width: 72px;
  padding: 6px;
  border: 2px solid var(--color-line, #3a3326);
  border-radius: 10px;
  background: var(--color-surface, #1c1814);
}
.slot--acting {
  border-color: var(--color-gold-high);
  box-shadow: 0 0 0 2px rgb(212 175 55 / 35%);
}
.slot--done {
  opacity: 0.55;
}
.badge {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  font-family: var(--font-display);
  font-weight: 700;
  background: var(--color-enemy-fill);
  border: 2px solid var(--color-enemy);
}
.slot--party .badge {
  background: var(--color-party-fill);
  border-color: var(--color-party);
}
.name {
  max-width: 88px;
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.init {
  font-size: 12px;
  color: var(--color-text-2);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
