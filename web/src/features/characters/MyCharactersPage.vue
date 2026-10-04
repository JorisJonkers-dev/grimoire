<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { listMyCharactersOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

const mine = useQuery(listMyCharactersOptions())
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">Your heroes</span>
      <h1>Your Characters</h1>
    </header>
    <p>Each Character is yours across Campaigns: a new Campaign gets its own progress, while the name and Backstory follow it everywhere.</p>
    <p v-if="mine.isError.value" role="alert" class="g-alert">Your Characters could not be read.</p>
    <ul v-else-if="mine.data.value?.items.length" class="cards" data-testid="my-characters">
      <li v-for="c in mine.data.value.items" :key="c.id" class="g-card">
        <RouterLink :to="{ name: 'my-character', params: { characterId: c.id } }" class="name">{{ c.name }}</RouterLink>
        <p class="dim">{{ c.species }} {{ c.class }} · {{ c.background }}</p>
        <p v-if="c.campaigns.length">{{ c.campaigns.map((e) => `${e.campaignName} (level ${e.level})`).join(' · ') }}</p>
        <p v-else class="dim">Not in a Campaign</p>
      </li>
    </ul>
    <p v-else-if="mine.isSuccess.value" data-testid="my-characters-none">
      No Characters yet. Make one from a Campaign's party page.
    </p>
  </main>
</template>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 260px), 1fr));
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.cards p {
  margin: 4px 0 0;
}
.name {
  font-size: 18px;
  font-weight: 600;
}
.dim {
  color: var(--color-text-3);
  font-size: 14px;
}
</style>
