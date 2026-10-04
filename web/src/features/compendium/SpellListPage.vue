<script setup lang="ts">
import { useInfiniteQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { listSpellsInfiniteOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { ListSpellsData, Ruleset, SpellPage } from '@/infrastructure/api/types.gen'
import { GButton, GField, GPageHead, GRow } from '@/shared/ui'
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
  <main class="g-page">
    <GPageHead eyebrow="Compendium" title="Spells" />
    <CompendiumTabs current="spell" />
    <form class="filters" role="search" @submit.prevent>
      <GField v-model="search" type="search" label="Search by name" data-testid="spell-search" class="grow" />
      <label class="g-field">
        <span>Level</span>
        <select v-model="level" data-testid="spell-level">
          <option value="">Any</option>
          <option v-for="n in 10" :key="n" :value="String(n - 1)">{{ levelLabel(n - 1) }}</option>
        </select>
      </label>
      <label class="g-field">
        <span>School</span>
        <select v-model="school">
          <option value="">Any</option>
          <option v-for="s in schools" :key="s" :value="s">{{ titleCase(s) }}</option>
        </select>
      </label>
      <label class="g-field">
        <span>Class</span>
        <select v-model="klass" data-testid="spell-class">
          <option value="">Any</option>
          <option v-for="c in classes" :key="c" :value="c">{{ titleCase(c) }}</option>
        </select>
      </label>
      <label class="g-field">
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
      <ul class="g-list rows" data-testid="spell-list">
        <li v-for="spell in items" :key="spell.slug">
          <GRow
            :to="{ name: 'spell', params: { slug: spell.slug }, query: ruleset ? { ruleset } : {} }"
            :title="spell.name"
            :subtitle="`${levelLabel(spell.level)} · ${titleCase(spell.school)}`"
            :data-testid="`spell-${spell.slug}`"
          >
            <template #leading>
              <span class="level" aria-hidden="true">{{ spell.level === 0 ? '·' : spell.level }}</span>
            </template>
            <template #trailing>
              <span class="marks">
                <span v-if="spell.concentration">Concentration</span>
                <span v-if="spell.ritual">Ritual</span>
                <span class="rules">{{ spell.ruleset === 'srd-2024' ? '2024' : '2014' }}</span>
              </span>
            </template>
          </GRow>
        </li>
      </ul>
      <GButton v-if="spells.hasNextPage.value" :disabled="spells.isFetchingNextPage.value" @click="spells.fetchNextPage()">
        Load more
      </GButton>
    </template>
  </main>
</template>

<style scoped>
.filters {
  display: grid;
  grid-template-columns: minmax(200px, 2fr) repeat(4, minmax(120px, 1fr));
  gap: 10px;
  align-items: start;
}
@media (max-width: 899px) {
  .filters {
    grid-template-columns: 1fr 1fr;
  }
  .grow {
    grid-column: 1 / -1;
  }
}
/* The level stands first in the row, in gold figures. */
.level {
  width: 36px;
  flex: none;
  font-family: var(--font-display);
  font-weight: 700;
  font-size: 18px;
  color: var(--color-gold-high);
}
.marks {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 4px 16px;
  font-size: 14px;
  color: var(--color-text-2);
}
.rules {
  font-size: 13px;
  color: var(--color-text-3);
}
</style>
