<script setup lang="ts">
import type { LiveSuggestion, LiveToken, Tactics } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

withDefaults(
  defineProps<{ token: LiveToken; armed: number | null; blocked: string; suggestion?: LiveSuggestion; target?: string; tactics?: Tactics }>(),
  { suggestion: undefined, target: 'its target', tactics: undefined },
)
const emit = defineEmits<{ arm: [attackNo: number]; use: []; tactics: [value: Tactics]; area: [effect: string] }>()
const spells = [
  { slug: 'burning-hands', name: 'Burning Hands' },
  { slug: 'thunderwave', name: 'Thunderwave' },
  { slug: 'shatter', name: 'Shatter' },
  { slug: 'grease', name: 'Grease' },
  { slug: 'fireball', name: 'Fireball' },
  { slug: 'lightning-bolt', name: 'Lightning Bolt' },
  { slug: 'cone-of-cold', name: 'Cone of Cold' },
]
const styles: { value: Tactics; label: string }[] = [
  { value: 'auto', label: 'From Intelligence' },
  { value: 'simple', label: 'Simple' },
  { value: 'cunning', label: 'Cunning' },
  { value: 'off', label: 'Off' },
]
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
    <div v-if="suggestion" class="suggestion" data-testid="suggestion">
      <p>
        <strong>Suggested: </strong>
        <template v-if="suggestion.attackNo !== undefined">{{ token.attacks?.[suggestion.attackNo]?.name }} against {{ target }}.</template>
        {{ suggestion.reason }}
      </p>
      <GButton v-if="suggestion.attackNo !== undefined" variant="primary" data-testid="use-suggestion" @click="emit('use')">Use suggestion</GButton>
    </div>
    <label class="g-field tactics">
      <span>Area spell</span>
      <select :disabled="blocked !== ''" data-testid="area-spell" @change="emit('area', ($event.target as HTMLSelectElement).value)">
        <option value="">Choose to aim…</option>
        <option v-for="s in spells" :key="s.slug" :value="s.slug">{{ s.name }}</option>
      </select>
    </label>
    <label v-if="tactics" class="g-field tactics">
      <span>Tactics</span>
      <select :value="tactics" data-testid="tactics" @change="emit('tactics', ($event.target as HTMLSelectElement).value as Tactics)">
        <option v-for="s in styles" :key="s.value" :value="s.value">{{ s.label }}</option>
      </select>
    </label>
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
.suggestion {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  width: 100%;
}
.suggestion p {
  margin: 0;
}
.tactics {
  min-width: 160px;
}
.stat,
.blocked,
.hint {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-2);
}
</style>
