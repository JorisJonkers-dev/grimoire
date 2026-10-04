<script setup lang="ts">
import { ref } from 'vue'
import type { LiveToken, Tactics } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { TACTICS } from './console'

const props = defineProps<{ creatures: LiveToken[]; inHand: string[]; acting: string[]; several: boolean }>()
const emit = defineEmits<{ take: [tokenId: string]; several: [on: boolean]; tactics: [value: Tactics]; hp: [delta: number] }>()
const amount = ref(1)
const change = (sign: number) => { emit('hp', sign * Math.max(1, Math.round(amount.value))) }
const held = (id: string) => props.inHand.includes(id)
</script>

<template>
  <section class="g-card switcher" aria-label="Creatures you run" data-testid="control-switcher">
    <h2>In hand</h2>
    <div class="row">
      <button
        v-for="t in creatures"
        :key="t.id"
        type="button"
        :class="['creature', { 'creature--held': held(t.id) }]"
        :aria-pressed="held(t.id)"
        :data-testid="`control-${t.label}`"
        @click="emit('take', t.id)"
      >
        {{ t.label }}<span v-if="acting.includes(t.id)" class="acting"> · acting</span>
      </button>
      <label class="check">
        <input type="checkbox" :checked="several" data-testid="control-several" @change="emit('several', ($event.target as HTMLInputElement).checked)" />
        <span>Several at once</span>
      </label>
    </div>
    <div v-if="several && inHand.length > 1" class="row" role="group" aria-label="All in hand" data-testid="bulk">
      <label class="g-field">
        <span>Tactics for all</span>
        <select data-testid="bulk-tactics" @change="emit('tactics', ($event.target as HTMLSelectElement).value as Tactics)">
          <option value="">Choose…</option>
          <option v-for="s in TACTICS" :key="s.value" :value="s.value">{{ s.label }}</option>
        </select>
      </label>
      <label class="g-field amount">
        <span>Hit points</span>
        <input v-model.number="amount" type="number" min="1" max="999" data-testid="bulk-hp" />
      </label>
      <GButton variant="danger" data-testid="bulk-hurt" @click="change(-1)">Damage all</GButton>
      <GButton data-testid="bulk-heal" @click="change(1)">Heal all</GButton>
    </div>
  </section>
</template>

<style scoped>
.switcher {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.creature {
  min-height: 44px;
  padding: 6px 12px;
  border: 2px solid var(--color-line);
  border-radius: 999px;
  color: var(--color-text);
  background: var(--color-raised);
  cursor: pointer;
}
.creature--held {
  border-color: var(--color-gold-high);
}
.acting {
  color: var(--color-gold-high);
}
.check {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 44px;
}
.amount input {
  width: 80px;
}
</style>
