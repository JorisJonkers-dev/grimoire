<script setup lang="ts">
// The pages of one Character in one Campaign, as tabs.
const props = withDefaults(
  defineProps<{ campaignId: string; characterId: string; current: 'sheet' | 'inventory' | 'spells' | 'retrain'; others?: boolean; retrain?: boolean }>(),
  { others: true, retrain: true },
)
const to = (name: string) => ({ name, params: { id: props.campaignId, characterId: props.characterId } })
const on = (tab: string) => (props.current === tab ? 'page' : undefined)
</script>

<template>
  <nav class="g-tabsnav" aria-label="Character">
    <RouterLink :to="to('character')" :aria-current="on('sheet')">Sheet</RouterLink>
    <template v-if="others">
      <RouterLink :to="to('character-inventory')" :aria-current="on('inventory')" data-testid="open-inventory">Inventory</RouterLink>
      <RouterLink :to="to('character-spells')" :aria-current="on('spells')" data-testid="open-spells">Spells</RouterLink>
      <RouterLink v-if="retrain" :to="to('character-retrain')" :aria-current="on('retrain')" data-testid="open-retrain">Retrain</RouterLink>
    </template>
  </nav>
</template>
