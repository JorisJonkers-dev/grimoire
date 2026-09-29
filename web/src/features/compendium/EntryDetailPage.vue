<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getEntryOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Ruleset } from '@/infrastructure/api/types.gen'
import { isEntryKind, kindLabel } from './kinds'
import RulesText from './RulesText.vue'

const route = useRoute()
const kind = computed(() => String(route.params.kind))
const slug = computed(() => String(route.params.slug))
const ruleset = computed(() => (typeof route.query.ruleset === 'string' ? (route.query.ruleset as Ruleset) : undefined))
const entry = useQuery(
  computed(() => ({
    ...getEntryOptions({
      path: { kind: isEntryKind(kind.value) ? kind.value : 'condition', slug: slug.value },
      query: ruleset.value ? { ruleset: ruleset.value } : {},
    }),
    enabled: isEntryKind(kind.value),
    retry: false,
  })),
)
const missing = computed(() => !isEntryKind(kind.value) || entry.isError.value)
</script>

<template>
  <main class="entry">
    <RouterLink v-if="kindLabel(kind)" :to="{ name: 'entries', params: { kind } }" class="back">← {{ kindLabel(kind) }}</RouterLink>
    <p v-if="missing" role="alert" data-testid="entry-missing">That entry is not in this grimoire.</p>
    <p v-else-if="entry.isPending.value">Turning the page…</p>
    <article v-else-if="entry.data.value" data-testid="entry-detail">
      <header>
        <h1>{{ entry.data.value.name }}</h1>
        <p class="sub">{{ entry.data.value.subtitle }}</p>
        <nav class="rules" aria-label="Ruleset">
          <RouterLink :to="{ name: 'entry', params: { kind, slug }, query: { ruleset: 'srd-2024' } }" :aria-current="entry.data.value.ruleset === 'srd-2024' ? 'page' : undefined">2024</RouterLink>
          <RouterLink :to="{ name: 'entry', params: { kind, slug }, query: { ruleset: 'srd-2014' } }" :aria-current="entry.data.value.ruleset === 'srd-2014' ? 'page' : undefined">2014</RouterLink>
        </nav>
      </header>
      <dl v-if="entry.data.value.facts.length" class="facts" data-testid="entry-facts">
        <template v-for="fact in entry.data.value.facts" :key="fact.label">
          <dt>{{ fact.label }}</dt>
          <dd>{{ fact.value }}</dd>
        </template>
      </dl>
      <section v-for="(section, i) in entry.data.value.sections" :key="i" class="section">
        <h2>{{ section.title }}</h2>
        <RulesText :text="section.text" :mentions="entry.data.value.mentions" />
      </section>
    </article>
  </main>
</template>

<style scoped>
.entry {
  width: 100%;
  max-width: 760px;
  margin: 0 auto;
  padding: 16px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.back {
  color: var(--color-gold-high);
}
h1 {
  margin: 0;
  font-family: var(--font-display);
}
.sub {
  margin: 4px 0 0;
  font-family: var(--font-flavour);
  font-style: italic;
  color: var(--color-text-2);
}
.rules {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
.rules a {
  min-height: 36px;
  padding: 6px 14px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-pill);
  color: var(--color-text);
  text-decoration: none;
}
.rules a[aria-current='page'] {
  border-color: var(--color-gold);
  color: var(--color-gold-high);
}
.facts {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 6px 16px;
  margin: 12px 0;
}
dt {
  color: var(--color-text-2);
}
dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.section h2 {
  margin: 8px 0 4px;
  font-family: var(--font-display);
  font-size: 17px;
}
</style>
