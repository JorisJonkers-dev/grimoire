<script setup lang="ts">
import type { LiveSuggestion, LiveToken, Tactics } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

withDefaults(
  defineProps<{ token: LiveToken; armed: number | null; blocked: string; suggestion?: LiveSuggestion; target?: string; tactics?: Tactics }>(),
  { suggestion: undefined, target: 'its target', tactics: undefined },
)
const emit = defineEmits<{
  arm: [attackNo: number]
  use: []
  tactics: [value: Tactics]
  area: [effect: string]
  action: [action: string]
  ready: [attackNo: number]
  unarmed: [option: string]
}>()
// The 2024 actions besides Attack and Ready, in the order the rules list them.
const actions = [
  { key: 'dash', name: 'Dash', tip: 'Gain extra movement equal to your Speed this turn.' },
  { key: 'disengage', name: 'Disengage', tip: 'Your movement provokes no Opportunity Attacks this turn.' },
  { key: 'dodge', name: 'Dodge', tip: 'Attacks against you have Disadvantage until your next turn.' },
  { key: 'help', name: 'Help', tip: 'Give an ally Advantage on their next check or attack.' },
  { key: 'hide', name: 'Hide', tip: 'DC 15 Dexterity (Stealth); on a success you are Invisible.' },
  { key: 'influence', name: 'Influence', tip: 'Sway a creature with a Charisma or Wisdom check.' },
  { key: 'magic', name: 'Magic', tip: 'Cast a spell or use a magic item.' },
  { key: 'search', name: 'Search', tip: 'Wisdom (Perception) to find what is hidden.' },
  { key: 'study', name: 'Study', tip: 'Intelligence to recall or work something out.' },
  { key: 'utilize', name: 'Utilize', tip: 'Use an object: open a door, pull a lever.' },
]
const unarmed = [
  { key: 'grapple', name: 'Grapple' },
  { key: 'shove_push', name: 'Shove away' },
  { key: 'shove_prone', name: 'Shove down' },
]
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
    <div class="actions" role="group" :aria-label="`${token.label}'s actions`">
      <GButton
        v-for="a in actions"
        :key="a.key"
        class="action"
        :disabled="blocked !== ''"
        :title="a.tip"
        :data-testid="`action-${a.key}`"
        @click="emit('action', a.key)"
      >
        {{ a.name }}
      </GButton>
      <GButton v-for="u in unarmed" :key="u.key" class="action" :disabled="blocked !== ''" :data-testid="`unarmed-${u.key}`" @click="emit('unarmed', u.key)">
        {{ u.name }}
      </GButton>
    </div>
    <label class="g-field tactics">
      <span>Ready an attack</span>
      <select :disabled="blocked !== ''" data-testid="ready" @change="emit('ready', Number(($event.target as HTMLSelectElement).value))">
        <option value="">When a creature comes within reach…</option>
        <option v-for="(a, i) in token.attacks ?? []" v-show="a.reachFt > 0" :key="a.name + String(i)" :value="i">{{ a.name }}</option>
      </select>
    </label>
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
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  width: 100%;
}
.action {
  min-height: 40px;
  padding: 0 12px;
  font-size: 14px;
}
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
