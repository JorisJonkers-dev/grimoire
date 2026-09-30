<script setup lang="ts">
import { useInfiniteQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { listEntriesInfiniteOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { EntryPage, ListEntriesData, Ruleset } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import CompendiumTabs from './CompendiumTabs.vue'
import { isEntryKind, kindLabel } from './kinds'

const route = useRoute()
const kind = computed(() => String(route.params.kind))
const known = computed(() => isEntryKind(kind.value))
const search = ref('')
const debounced = ref('')
const ruleset = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (value) => {
  clearTimeout(timer)
  timer = setTimeout(() => (debounced.value = value.trim()), 250)
})
watch(kind, () => {
  search.value = ''
  debounced.value = ''
})

const query = computed(
  () =>
    ({
      kind: kind.value,
      ...(debounced.value ? { q: debounced.value } : {}),
      ...(ruleset.value ? { ruleset: ruleset.value as Ruleset } : {}),
    }) as ListEntriesData['query'],
)

const entries = useInfiniteQuery(
  computed(() => ({
    ...listEntriesInfiniteOptions({ query: query.value }),
    initialPageParam: {},
    getNextPageParam: (last: EntryPage) => last.nextCursor,
    enabled: known.value,
  })),
)
const items = computed(() => entries.data.value?.pages.flatMap((p) => p.items) ?? [])
</script>

<template>
  <main class="entries">
    <CompendiumTabs :current="kind" />
    <p v-if="!known" role="alert" data-testid="entry-kind-missing">There is no such section in this grimoire.</p>
    <template v-else>
      <h1>{{ kindLabel(kind) }}</h1>
      <form class="filters" role="search" @submit.prevent>
        <label class="field grow">
          <span>Name</span>
          <input v-model="search" type="search" placeholder="Search by name" data-testid="entry-search" />
        </label>
        <label class="field">
          <span>Rules</span>
          <select v-model="ruleset" data-testid="entry-ruleset">
            <option value="">2024 leads</option>
            <option value="srd-2024">2024 only</option>
            <option value="srd-2014">2014 only</option>
          </select>
        </label>
      </form>
      <p v-if="entries.isPending.value">Opening the grimoire…</p>
      <p v-else-if="entries.isError.value" role="alert">This list could not be loaded. Try again shortly.</p>
      <template v-else>
        <p v-if="items.length === 0" data-testid="entry-empty">Nothing matches.</p>
        <ul class="list" data-testid="entry-list">
          <li v-for="entry in items" :key="entry.slug">
            <RouterLink :to="{ name: 'entry', params: { kind, slug: entry.slug }, query: ruleset ? { ruleset } : {} }" class="row" :data-testid="`entry-${entry.slug}`">
              <span class="name">{{ entry.name }}</span>
              <span class="meta">{{ entry.subtitle }}</span>
              <span class="tag">{{ entry.ruleset === 'srd-2024' ? '2024' : '2014' }}</span>
            </RouterLink>
          </li>
        </ul>
        <GButton v-if="entries.hasNextPage.value" :disabled="entries.isFetchingNextPage.value" @click="entries.fetchNextPage()">
          Load more
        </GButton>
      </template>
    </template>
  </main>
</template>

<style scoped>
.entries {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  max-width: 960px;
  margin: 0 auto;
  padding: 16px;
  box-sizing: border-box;
}
h1 {
  margin: 0;
  font-family: var(--font-display);
}
.filters {
  display: grid;
  grid-template-columns: 1fr minmax(140px, auto);
  gap: 10px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
  color: var(--color-text-2);
}
input,
select {
  min-height: 44px;
  padding: 0 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
  font: inherit;
  font-size: 16px;
}
.list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 2px 12px;
  min-height: 44px;
  padding: 10px 14px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
  text-decoration: none;
}
.row:hover,
.row:focus-visible {
  border-color: var(--color-gold);
}
.name {
  font-weight: 700;
}
.meta {
  grid-column: 1;
  font-size: 14px;
  color: var(--color-text-2);
}
.tag {
  grid-row: 1 / span 2;
  grid-column: 2;
  align-self: center;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-bronze);
  font-size: 12px;
  color: var(--color-text-2);
}
</style>
