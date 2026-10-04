<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getEntryOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { Ruleset } from '@/infrastructure/api/types.gen'
import CopyLink from './CopyLink.vue'
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
  <main class="g-page">
    <nav aria-label="Breadcrumb" class="g-crumbs">
      <RouterLink :to="{ name: 'spells' }">Compendium</RouterLink> <span aria-hidden="true">›</span>
      <template v-if="kindLabel(kind)">
        <RouterLink :to="{ name: 'entries', params: { kind } }" class="back">{{ kindLabel(kind) }}</RouterLink> <span aria-hidden="true">›</span>
      </template>
      <span aria-current="page">{{ entry.data.value?.name ?? slug }}</span>
    </nav>
    <p v-if="missing" role="alert" data-testid="entry-missing">That entry is not in this grimoire.</p>
    <p v-else-if="entry.isPending.value">Turning the page…</p>
    <article v-else-if="entry.data.value" class="body" data-testid="entry-detail">
      <header class="g-detail-head">
        <div>
          <span class="g-eyebrow">{{ kindLabel(kind) }} · SRD {{ entry.data.value.ruleset === 'srd-2024' ? '5.2' : '5.1' }}</span>
          <h1>{{ entry.data.value.name }}</h1>
          <p class="g-meta sub">{{ entry.data.value.subtitle }}</p>
        </div>
        <div class="g-acts">
          <nav class="g-segment" aria-label="Ruleset">
            <RouterLink :to="{ name: 'entry', params: { kind, slug }, query: { ruleset: 'srd-2024' } }" :aria-current="entry.data.value.ruleset === 'srd-2024' ? 'page' : undefined">2024</RouterLink>
            <RouterLink :to="{ name: 'entry', params: { kind, slug }, query: { ruleset: 'srd-2014' } }" :aria-current="entry.data.value.ruleset === 'srd-2014' ? 'page' : undefined">2014</RouterLink>
          </nav>
          <CopyLink :to="{ name: 'entry', params: { kind, slug }, query: { ruleset: entry.data.value.ruleset } }" />
        </div>
      </header>
      <dl v-if="entry.data.value.facts.length" class="g-facts facts" data-testid="entry-facts">
        <div v-for="fact in entry.data.value.facts" :key="fact.label">
          <dt>{{ fact.label }}</dt>
          <dd>{{ fact.value }}</dd>
        </div>
      </dl>
      <section v-for="(section, i) in entry.data.value.sections" :key="i" class="part section">
        <h2>{{ section.title }}</h2>
        <RulesText :text="section.text" :mentions="entry.data.value.mentions" class="g-prose" />
      </section>
    </article>
  </main>
</template>

<style scoped>
.g-page {
  gap: 32px;
  padding-top: 40px;
}
.body {
  display: flex;
  flex-direction: column;
  gap: 36px;
  max-width: 980px;
}
.part {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
</style>
