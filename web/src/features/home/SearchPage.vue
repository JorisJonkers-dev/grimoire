<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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
// The page is for typing into: the caret starts in its box.
const box = ref<HTMLInputElement | null>(null)
onMounted(() => box.value?.focus())

const long = computed(() => asked.value.length >= MIN)
const results = useQuery(computed(() => ({ ...searchOptions({ query: { q: asked.value } }), enabled: long.value, retry: false })))
const hits = computed(() => (long.value ? (results.data.value?.hits ?? []) : []))
const titles: Record<SearchHit['group'], string> = { compendium: 'Compendium', library: 'Library', campaigns: 'Campaigns', people: 'People' }
const groups = computed(() =>
  (Object.keys(titles) as SearchHit['group'][]).map((g) => ({ group: g, title: titles[g], hits: hits.value.filter((h) => h.group === g) })).filter((g) => g.hits.length > 0),
)
// The places to look, each with how much was found there; one may be chosen to see it alone.
const scope = ref<SearchHit['group'] | 'all'>('all')
const scopes = computed(() => [{ key: 'all' as const, title: 'Everything', n: hits.value.length }, ...groups.value.map((g) => ({ key: g.group, title: g.title, n: g.hits.length }))])
const shown = computed(() => groups.value.filter((g) => scope.value === 'all' || g.group === scope.value))
// The result in hand shows in full beside the list: the one pointed at, else the first listed.
const inHand = ref('')
const listed = computed(() => shown.value.flatMap((g) => g.hits))
const previewed = computed(() => listed.value.find((h) => h.path === inHand.value) ?? listed.value[0])
watch(hits, () => { scope.value = 'all' })
const count = computed(() => {
  const n = hits.value.length
  if (n === 0) return `Nothing found for “${asked.value}”.`
  return `${String(n)} ${n === 1 ? 'result' : 'results'} for “${asked.value}”.`
})
</script>

<template>
  <main class="g-page">
    <header class="g-headline">
      <span class="g-eyebrow">Everywhere</span>
      <h1>Search</h1>
    </header>
    <section class="palette">
      <label class="ask">
        <svg width="20" height="20" viewBox="0 0 16 16" aria-hidden="true"><circle cx="7" cy="7" r="5" fill="none" stroke="currentColor" stroke-width="1.4" /><path d="M11 11 L14.5 14.5" stroke="currentColor" stroke-width="1.4" /></svg>
        <span class="sr">Search the compendium, your Library, your Campaigns and your Friends</span>
        <input ref="box" v-model="typed" type="search" maxlength="80" autocomplete="off" placeholder="Search the compendium, your Library, your Campaigns and your Friends" aria-keyshortcuts="/" data-testid="search-input" />
        <kbd aria-hidden="true">/</kbd>
      </label>
      <p v-if="!long" class="hint" data-testid="search-hint">Type at least 2 characters.</p>
      <p v-else-if="results.isError.value" role="alert" class="g-alert" data-testid="search-problem">Search is not available just now. Sign in and try again.</p>
      <template v-else-if="results.isSuccess.value">
        <div v-if="hits.length" class="scopes" role="group" aria-label="Where to look" data-testid="search-scopes">
          <button v-for="o in scopes" :key="o.key" type="button" :aria-pressed="scope === o.key" @click="scope = o.key">{{ o.title }} · {{ o.n }}</button>
        </div>
        <p role="status" class="hint" data-testid="search-count">{{ count }}</p>
        <div v-if="hits.length" class="found">
          <div class="list">
            <section v-for="g in shown" :key="g.group" :aria-labelledby="`search-${g.group}`" data-testid="search-group">
              <h2 :id="`search-${g.group}`">{{ g.title }}</h2>
              <ul>
                <li v-for="h in g.hits" :key="h.path + h.title" :class="['hit', { on: previewed === h }]" data-testid="search-hit">
                  <RouterLink :to="h.path" @focus="inHand = h.path" @mouseenter="inHand = h.path">{{ h.title }}</RouterLink>
                  <span class="preview">{{ h.preview }}</span>
                </li>
              </ul>
            </section>
          </div>
          <article v-if="previewed" class="card" aria-label="The result in hand" data-testid="search-preview">
            <span class="g-eyebrow">{{ titles[previewed.group] }} · {{ previewed.kind }}</span>
            <h2>{{ previewed.title }}</h2>
            <p>{{ previewed.preview }}</p>
            <RouterLink :to="previewed.path" class="g-button-link g-button-link--primary">Open</RouterLink>
          </article>
        </div>
      </template>
    </section>
  </main>
</template>

<style scoped>
/* Search is one panel: what you ask, where to look, what was found, and the one in hand in full. */
.palette {
  max-width: 1040px;
  border: 1px solid var(--color-rule);
  border-radius: var(--radius-panel);
  background: var(--color-surface);
}
.ask {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 60px;
  padding: 0 18px;
  border-bottom: 1px solid var(--color-line);
  color: var(--color-text-3);
}
.ask input {
  flex: 1;
  min-width: 0;
  border: 0;
  font: inherit;
  font-size: 20px;
  color: var(--color-text);
  background: transparent;
}
.ask input:focus {
  outline: none;
}
.ask:focus-within {
  box-shadow: inset 0 -2px 0 var(--color-brass-edge);
}
kbd {
  padding: 2px 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-chip);
  font-family: var(--font-ui);
  font-size: 12px;
}
.scopes {
  display: flex;
  flex-wrap: wrap;
  gap: 0 22px;
  padding: 6px 18px;
  border-bottom: 1px solid var(--color-line);
}
.scopes button {
  min-height: 44px;
  padding: 0;
  border: 0;
  font-family: var(--font-label);
  font-size: 14px;
  color: var(--color-text-2);
  background: transparent;
  cursor: pointer;
}
.scopes button[aria-pressed='true'] {
  color: var(--color-gold-high);
  box-shadow: inset 0 -2px 0 var(--color-gold);
}
.hint {
  margin: 0;
  padding: 10px 18px;
  font-size: 14px;
  color: var(--color-text-2);
}
.palette > .g-alert {
  margin: 12px 18px;
}
.found {
  display: grid;
  grid-template-columns: minmax(0, 430px) minmax(0, 1fr);
  border-top: 1px solid var(--color-rule);
}
.list {
  padding-bottom: 8px;
  border-right: 1px solid var(--color-rule);
}
.list h2 {
  margin: 0;
  padding: 12px 18px 6px;
  font-size: 13px;
}
ul {
  margin: 0;
  padding: 0;
  list-style: none;
}
.hit {
  position: relative;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 18px;
  border-left: 2px solid transparent;
}
.hit.on {
  border-left-color: var(--color-gold);
  background: var(--color-selected);
}
.hit a {
  color: var(--color-text);
  text-decoration: none;
}
/* The whole row is the link. */
.hit a::after {
  position: absolute;
  inset: 0;
  content: '';
}
.hit.on a {
  color: var(--color-gold-high);
}
.preview {
  font-size: 13px;
  text-align: right;
  color: var(--color-text-2);
}
.card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 10px;
  padding: 22px 26px;
}
.card h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 30px;
  font-weight: 700;
  letter-spacing: 0;
  text-transform: none;
  color: var(--color-text);
}
.card p {
  margin: 0 0 6px;
  font-family: var(--font-flavour);
  font-size: 17px;
  line-height: 1.5;
  color: var(--color-text-2);
}
.sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
@media (max-width: 899px) {
  .found {
    grid-template-columns: minmax(0, 1fr);
  }
  .list {
    border-right: 0;
  }
  .card {
    display: none;
  }
  .hit {
    flex-direction: column;
    gap: 2px;
  }
  .preview {
    text-align: left;
  }
}
</style>
