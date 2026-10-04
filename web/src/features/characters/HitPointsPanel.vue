<script setup lang="ts">
import { computed, ref } from 'vue'
import { GButton } from '@/shared/ui'
import type { HitPointChange } from './build'

const props = defineProps<{ current: number; max: number; temp: number; editable: boolean }>()
const emit = defineEmits<{ change: [HitPointChange] }>()
const amount = ref(1)
const pct = computed(() => Math.round((props.current / props.max) * 100))
const valid = computed(() => Number.isInteger(amount.value) && amount.value > 0 && amount.value <= 1000)
</script>

<template>
  <section class="hp" aria-label="Hit points">
    <div class="numbers">
      <span class="label">Hit points</span>
      <strong data-testid="hp">{{ current }} / {{ max }}</strong>
      <span v-if="temp > 0" class="temp" data-testid="temp-hp">+{{ temp }} temporary</span>
    </div>
    <span class="bar" role="img" :aria-label="`${String(pct)} percent of hit points`">
      <span :style="{ width: `${String(pct)}%` }" :class="{ low: pct <= 25 }" />
    </span>
    <form v-if="editable" class="controls" @submit.prevent>
      <label>
        <span class="label">Amount</span>
        <input v-model.number="amount" type="number" min="1" max="1000" inputmode="numeric" data-testid="hp-amount" />
      </label>
      <GButton variant="danger" :disabled="!valid" data-testid="hp-damage" @click="emit('change', { damage: amount })">Damage</GButton>
      <GButton :disabled="!valid" data-testid="hp-heal" @click="emit('change', { heal: amount })">Heal</GButton>
      <GButton :disabled="!valid" data-testid="hp-temp" @click="emit('change', { tempHp: amount })">Temporary</GButton>
    </form>
  </section>
</template>

<style scoped>
.hp {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
  padding: 10px 14px;
}
.numbers {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 12px;
}
.label {
  font-size: 12px;
  color: var(--color-text-3);
}
strong {
  font-family: var(--font-display);
  font-size: 26px;
  line-height: 1.1;
}
.temp {
  font-size: 14px;
  color: var(--color-gold-high);
}
.bar {
  height: 7px;
  overflow: hidden;
  border-radius: 4px;
  background: var(--color-rule);
}
.bar span {
  display: block;
  height: 100%;
  background: var(--color-party);
}
.bar span.low {
  background: var(--color-danger-edge);
}
.controls {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 6px;
}
.controls label {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
input {
  width: 64px;
  min-height: 32px;
  padding: 0 8px;
  border: 0;
  border-radius: var(--radius-control) var(--radius-control) 0 0;
  font: inherit;
  color: var(--color-text);
  background: var(--color-inset);
  box-shadow: inset 0 -1px 0 var(--color-line);
}
.controls :deep(.g-button) {
  min-height: 32px;
  padding: 0 14px;
  font-size: 13px;
}
@media (pointer: coarse) {
  input,
  .controls :deep(.g-button) {
    min-height: 44px;
  }
}
</style>
