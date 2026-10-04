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
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}
.numbers {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 12px;
}
.label {
  font-size: 12px;
  color: var(--color-text-2);
}
strong {
  font-family: var(--font-display);
  font-size: 26px;
}
.temp {
  color: var(--color-gold-high);
}
.bar {
  height: 8px;
  border-radius: var(--radius-chip);
  background: var(--color-raised);
  overflow: hidden;
}
.bar span {
  display: block;
  height: 100%;
  background: var(--color-success);
}
.bar span.low {
  background: var(--color-enemy-soft);
}
.controls {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 8px;
}
.controls label {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
input {
  width: 80px;
  min-height: 44px;
  padding: 0 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font: inherit;
}
</style>
