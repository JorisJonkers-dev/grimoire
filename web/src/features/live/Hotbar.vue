<script setup lang="ts">
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

defineProps<{ token: LiveToken; armed: number | null; blocked: string }>()
const emit = defineEmits<{ arm: [attackNo: number] }>()
const signed = (n: number) => (n >= 0 ? `+${String(n)}` : String(n))
const damage = (a: NonNullable<LiveToken['attacks']>[number]) =>
  a.damage ? `${a.damage}${a.damageBonus ? signed(a.damageBonus) : ''}` : String(a.damageBonus)
const reach = (a: NonNullable<LiveToken['attacks']>[number]) =>
  [a.reachFt ? `reach ${String(a.reachFt)} ft` : '', a.rangeFt ? `range ${String(a.rangeFt)}/${String(a.longRangeFt)} ft` : ''].filter(Boolean).join(', ')
</script>

<template>
  <section class="hotbar" :aria-label="`${token.label}'s attacks`" :data-testid="`hotbar-${token.label}`">
    <GButton
      v-for="(a, i) in token.attacks ?? []"
      :key="a.name + String(i)"
      :class="['slot', { 'slot--armed': armed === i }]"
      :disabled="blocked !== ''"
      :aria-pressed="armed === i"
      :title="blocked || `${a.name}: ${signed(a.toHit)} to hit, ${damage(a)} ${a.damageType ?? ''}`"
      :data-testid="`attack-${String(i)}`"
      @click="emit('arm', i)"
    >
      <span class="name">{{ a.name }}</span>
      <span class="stat">{{ signed(a.toHit) }} · {{ damage(a) }} · {{ reach(a) }}</span>
    </GButton>
    <p v-if="blocked" class="blocked" data-testid="hotbar-blocked">{{ blocked }}</p>
    <p v-else-if="armed !== null" class="hint" role="status">Tap a creature to aim.</p>
  </section>
</template>

<style scoped>
.hotbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.slot {
  display: grid;
  gap: 2px;
  text-align: left;
}
.slot--armed {
  outline: 3px solid var(--color-gold-high);
}
.name {
  font-weight: 600;
}
.stat,
.blocked,
.hint {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-2);
}
</style>
