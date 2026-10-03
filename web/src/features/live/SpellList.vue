<script setup lang="ts">
import { ref } from 'vue'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { areaSpells, summonings } from './actionBar'

defineProps<{ token: LiveToken; blocked: string }>()
const emit = defineEmits<{ area: [effect: string, slot: number]; summon: [effect: string] }>()
const slot = ref(0)
</script>

<template>
  <section class="spell-list g-card" :aria-label="`${token.label}'s spells`" data-testid="spell-list">
    <h2>{{ token.label }}'s spells</h2>
    <label class="g-field">
      <span>Spell slot</span>
      <select v-model.number="slot" data-testid="list-slot">
        <option :value="0">Lowest</option>
        <option v-for="n in 9" :key="n" :value="n">Level {{ n }}</option>
      </select>
    </label>
    <div class="spells">
      <GButton v-for="s in areaSpells" :key="s.slug" :disabled="blocked !== ''" :title="blocked" :data-testid="`list-spell-${s.slug}`" @click="emit('area', s.slug, slot)">{{ s.name }}</GButton>
      <GButton v-for="s in summonings" :key="s.slug" :disabled="blocked !== ''" :title="blocked" :data-testid="`list-summon-${s.slug}`" @click="emit('summon', s.slug)">{{ s.name }}</GButton>
    </div>
  </section>
</template>

<style scoped>
.spell-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
h2 {
  margin: 0;
  font-size: 18px;
}
.spells {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
/* Wider screens cast from the hotbar; this list is the phone's Spells page. */
@media (min-width: 900px) {
  .spell-list {
    display: none;
  }
}
</style>
