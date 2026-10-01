<script setup lang="ts">
import { useInfiniteQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { listEntriesInfiniteOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { EntryPage, ListEntriesData, Ruleset } from '@/infrastructure/api/types.gen'
import { GButton, GField, GRow } from '@/shared/ui'
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
  <main class="g-page">
    <CompendiumTabs :current="kind" />
    <p v-if="!known" role="alert" data-testid="entry-kind-missing">There is no such section in this grimoire.</p>
    <template v-else>
      <h1>{{ kindLabel(kind) }}</h1>
      <form class="filters" role="search" @submit.prevent>
        <GField v-model="search" type="search" label="Search by name" data-testid="entry-search" />
        <label class="g-field">
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
        <ul class="g-list rows" data-testid="entry-list">
          <li v-for="entry in items" :key="entry.slug">
            <GRow
              :to="{ name: 'entry', params: { kind, slug: entry.slug }, query: ruleset ? { ruleset } : {} }"
              :title="entry.name"
              :subtitle="entry.subtitle"
              :data-testid="`entry-${entry.slug}`"
            >
              <template #trailing>
                <span class="g-tag">{{ entry.ruleset === 'srd-2024' ? '2024' : '2014' }}</span>
              </template>
            </GRow>
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
.filters {
  display: grid;
  grid-template-columns: 1fr minmax(140px, auto);
  gap: 10px;
  align-items: start;
}
.rows {
  gap: 0;
}
</style>
