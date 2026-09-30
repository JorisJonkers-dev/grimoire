<script setup lang="ts">
import { useInfiniteQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { listSpellsInfiniteOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { ListSpellsData, Ruleset, SpellPage } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import CompendiumTabs from './CompendiumTabs.vue'
import { classes, levelLabel, schools, titleCase } from './highlight'

const search = ref('')
const debounced = ref('')
const level = ref('')
const school = ref('')
const klass = ref('')
const ruleset = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (value) => {
  clearTimeout(timer)
  timer = setTimeout(() => (debounced.value = value.trim()), 250)
})

const query = computed<NonNullable<ListSpellsData['query']>>(() => ({
  ...(debounced.value ? { q: debounced.value } : {}),
  ...(level.value !== '' ? { level: Number(level.value) } : {}),
  ...(school.value ? { school: school.value } : {}),
  ...(klass.value ? { class: klass.value } : {}),
  ...(ruleset.value ? { ruleset: ruleset.value as Ruleset } : {}),
}))

const spells = useInfiniteQuery(
  computed(() => ({
    ...listSpellsInfiniteOptions({ query: query.value }),
    initialPageParam: {},
    getNextPageParam: (last: SpellPage) => last.nextCursor,
  })),
)
const items = computed(() => spells.data.value?.pages.flatMap((p) => p.items) ?? [])
</script>

<template>
  <main class="spells">
    <CompendiumTabs current="spell" />
    <h1>Spells</h1>
    <form class="filters" role="search" @submit.prevent>
      <label class="field grow">
        <span>Name</span>
        <input v-model="search" type="search" placeholder="Fireball, bless…" data-testid="spell-search" />
      </label>
      <label class="field">
        <span>Level</span>
        <select v-model="level" data-testid="spell-level">
          <option value="">Any</option>
          <option v-for="n in 10" :key="n" :value="String(n - 1)">{{ levelLabel(n - 1) }}</option>
        </select>
      </label>
      <label class="field">
        <span>School</span>
        <select v-model="school">
          <option value="">Any</option>
          <option v-for="s in schools" :key="s" :value="s">{{ titleCase(s) }}</option>
        </select>
      </label>
      <label class="field">
        <span>Class</span>
        <select v-model="klass" data-testid="spell-class">
          <option value="">Any</option>
          <option v-for="c in classes" :key="c" :value="c">{{ titleCase(c) }}</option>
        </select>
      </label>
      <label class="field">
        <span>Rules</span>
        <select v-model="ruleset">
          <option value="">2024 leads</option>
          <option value="srd-2024">2024 only</option>
          <option value="srd-2014">2014 only</option>
        </select>
      </label>
    </form>

    <p v-if="spells.isPending.value">Opening the grimoire…</p>
    <p v-else-if="spells.isError.value" role="alert">The spell list could not be loaded. Try again shortly.</p>
    <template v-else>
      <p v-if="items.length === 0" data-testid="spell-empty">No spells match.</p>
      <ul class="list" data-testid="spell-list">
        <li v-for="spell in items" :key="spell.slug">
          <RouterLink :to="{ name: 'spell', params: { slug: spell.slug }, query: ruleset ? { ruleset } : {} }" class="row" :data-testid="`spell-${spell.slug}`">
            <span class="name">{{ spell.name }}</span>
            <span class="meta">{{ levelLabel(spell.level) }} · {{ titleCase(spell.school) }}</span>
            <span class="tags">
              <span v-if="spell.concentration" class="tag">C</span>
              <span v-if="spell.ritual" class="tag">R</span>
              <span class="tag">{{ spell.ruleset === 'srd-2024' ? '2024' : '2014' }}</span>
            </span>
          </RouterLink>
        </li>
      </ul>
      <GButton v-if="spells.hasNextPage.value" :disabled="spells.isFetchingNextPage.value" @click="spells.fetchNextPage()">
        Load more
      </GButton>
    </template>
  </main>
</template>

<style scoped>
.spells {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  box-sizing: border-box;
  max-width: 960px;
  margin: 0 auto;
  padding: 16px;
}
h1 {
  margin: 0;
  font-family: var(--font-display);
}
.filters {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
  color: var(--color-text-2);
}
.grow {
  grid-column: 1 / -1;
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
.tags {
  grid-row: 1 / span 2;
  grid-column: 2;
  display: flex;
  gap: 4px;
  align-items: center;
}
.tag {
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--color-bronze);
  font-size: 12px;
  color: var(--color-text-2);
}
</style>
