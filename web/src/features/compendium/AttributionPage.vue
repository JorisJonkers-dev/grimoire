<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { listSourcesOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'

const sources = useQuery({ ...listSourcesOptions(), retry: false })
</script>

<template>
  <main class="attribution">
    <h1>Attribution</h1>
    <p>
      Grimoire is created by <a href="https://jorisjonkers.dev">Joris Jonkers</a> and licensed under the
      Attribution Assurance License.
    </p>
    <p v-if="sources.isError.value" role="alert">The source list could not be loaded.</p>
    <article v-for="source in sources.data.value ?? []" :key="source.key" class="source" data-testid="source">
      <h2>{{ source.title }}</h2>
      <p class="license">{{ source.license }} · {{ source.rulesetYear }} rules</p>
      <p>{{ source.attribution }}</p>
      <a :href="source.url">{{ source.url }}</a>
    </article>
  </main>
</template>

<style scoped>
.attribution {
  max-width: 760px;
  margin: 0 auto;
  padding: 16px;
}
h1,
h2 {
  font-family: var(--font-display);
}
a {
  color: var(--color-gold-high);
}
.source {
  padding: 14px 16px;
  margin-bottom: 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}
.license {
  color: var(--color-text-2);
}
</style>
