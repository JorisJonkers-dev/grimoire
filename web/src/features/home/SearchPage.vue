<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { searchOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { SearchHit } from '@/infrastructure/api/types.gen'

// Universal search: everything the caller may open, by name, each result with a line that previews it.
const MIN = 2
const PAUSE_MS = 250
const route = useRoute()
const router = useRouter()
const typed = ref(typeof route.query.q === 'string' ? route.query.q : '')
// asked is what is looked for: what was typed, once the typing pauses.
const asked = ref(typed.value.trim())
let pause: ReturnType<typeof setTimeout> | undefined
watch(typed, (now) => {
  clearTimeout(pause)
  pause = setTimeout(() => {
    asked.value = now.trim()
    void router.replace({ name: 'search', query: asked.value ? { q: asked.value } : {} })
  }, PAUSE_MS)
})
onBeforeUnmount(() => { clearTimeout(pause) })

const long = computed(() => asked.value.length >= MIN)
const results = useQuery(computed(() => ({ ...searchOptions({ query: { q: asked.value } }), enabled: long.value, retry: false })))
const hits = computed(() => (long.value ? (results.data.value?.hits ?? []) : []))
const titles: Record<SearchHit['group'], string> = { compendium: 'Compendium', library: 'Library', campaigns: 'Campaigns', people: 'People' }
const groups = computed(() =>
  (Object.keys(titles) as SearchHit['group'][]).map((g) => ({ group: g, title: titles[g], hits: hits.value.filter((h) => h.group === g) })).filter((g) => g.hits.length > 0),
)
const count = computed(() => {
  const n = hits.value.length
  if (n === 0) return `Nothing found for “${asked.value}”.`
  return `${String(n)} ${n === 1 ? 'result' : 'results'} for “${asked.value}”.`
})
</script>

<template>
  <main class="g-page">
    <h1>Search</h1>
    <label class="g-field">
      <span>Search the compendium, your Library, your Campaigns and your Friends</span>
      <input v-model="typed" type="search" maxlength="80" autocomplete="off" data-testid="search-input" />
    </label>
    <p v-if="!long" class="hint" data-testid="search-hint">Type at least 2 characters.</p>
    <p v-else-if="results.isError.value" role="alert" class="g-alert" data-testid="search-problem">Search is not available just now. Sign in and try again.</p>
    <template v-else-if="results.isSuccess.value">
      <p role="status" class="hint" data-testid="search-count">{{ count }}</p>
      <section v-for="g in groups" :key="g.group" class="g-card" :aria-labelledby="`search-${g.group}`" data-testid="search-group">
        <h2 :id="`search-${g.group}`">{{ g.title }}</h2>
        <ul class="g-list">
          <li v-for="h in g.hits" :key="h.path + h.title" class="hit" data-testid="search-hit">
            <RouterLink :to="h.path">{{ h.title }}</RouterLink>
            <span class="preview">{{ h.preview }}</span>
          </li>
        </ul>
      </section>
    </template>
  </main>
</template>

<style scoped>
.hint {
  margin: 0;
  color: var(--color-text-2);
}
h2 {
  margin: 0 0 8px;
}
.hit {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.hit a {
  color: var(--color-gold-high);
}
.preview {
  color: var(--color-text-2);
  font-size: 14px;
}
</style>
