<script setup lang="ts">
import type { LiveReactionSetting, LiveToken } from '@/infrastructure/api/types.gen'

type Kind = LiveReactionSetting['kind']
type Mode = LiveReactionSetting['mode']

const props = defineProps<{ token: LiveToken }>()
const emit = defineEmits<{ set: [kind: Kind, mode: Mode, condition: '' | 'target_bloodied'] }>()
const kinds: { kind: Kind; name: string }[] = [
  { kind: 'opportunity_attack', name: 'Opportunity attacks' },
  { kind: 'shield', name: 'Shield' },
  { kind: 'readied', name: 'Readied attacks' },
  { kind: 'effect', name: 'Other reactions' },
]
const current = (kind: Kind) => props.token.reactions?.find((r) => r.kind === kind)
const modeOf = (kind: Kind): Mode => current(kind)?.mode ?? 'ask'
const bloodied = (kind: Kind) => current(kind)?.condition === 'target_bloodied'
function change(kind: Kind, mode: Mode, onlyBloodied: boolean) {
  emit('set', kind, mode, mode === 'always' && onlyBloodied ? 'target_bloodied' : '')
}
</script>

<template>
  <section class="g-card settings" :aria-label="`${token.label}'s reactions`" data-testid="reaction-settings">
    <h2>{{ token.label }}'s reactions</h2>
    <div v-for="k in kinds" :key="k.kind" class="row">
      <label class="g-field grow">
        <span>{{ k.name }}</span>
        <select :value="modeOf(k.kind)" :data-testid="`reaction-${k.kind}`" @change="change(k.kind, ($event.target as HTMLSelectElement).value as Mode, bloodied(k.kind))">
          <option value="ask">Ask me</option>
          <option value="always">Always take it</option>
          <option value="never">Never</option>
        </select>
      </label>
      <label v-if="modeOf(k.kind) === 'always' && k.kind !== 'shield'" class="check">
        <input
          type="checkbox"
          :checked="bloodied(k.kind)"
          :data-testid="`reaction-${k.kind}-bloodied`"
          @change="change(k.kind, 'always', ($event.target as HTMLInputElement).checked)"
        />
        <span>Only against a bloodied creature</span>
      </label>
    </div>
    <p class="hint">Shield is only ever taken when it turns a hit into a miss.</p>
  </section>
</template>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.hint {
  margin: 0;
  color: var(--color-text-3);
  font-size: 13px;
}
</style>
