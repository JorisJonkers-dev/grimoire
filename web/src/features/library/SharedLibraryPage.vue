<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { listSharedEntriesOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LibraryKind } from '@/infrastructure/api/types.gen'
import { GRow, GTabs } from '@/shared/ui'
import { kindNames } from './fields'

const kinds = Object.keys(kindNames) as LibraryKind[]
const shown = ref('all')
const tabs = computed(() => [{ value: 'all', label: 'All' }, ...kinds.map((k) => ({ value: k, label: kindNames[k] ?? k }))])
const shared = useQuery(computed(() => ({ ...listSharedEntriesOptions(), retry: false })))
const list = computed(() => (shared.data.value ?? []).filter((e) => shown.value === 'all' || e.kind === shown.value))
</script>

<template>
  <main class="g-page shared">
    <RouterLink :to="{ name: 'library' }" class="back">← Library</RouterLink>
    <h1>Shared Library</h1>
    <p class="hint">Entries other DMs shared, checked by an Admin to hold no non-SRD text. Open one to link it into a Campaign you run; it stays read-only.</p>
    <p v-if="shared.isError.value" role="alert" class="g-alert" data-testid="shared-error">The Shared Library could not be opened.</p>
    <template v-else>
      <GTabs v-model="shown" label="Kinds" :tabs="tabs" />
      <p v-if="shared.isSuccess.value && list.length === 0" class="hint" data-testid="shared-empty">Nothing shared here yet.</p>
      <ul class="g-list" data-testid="shared-list">
        <li v-for="e in list" :key="e.id">
          <GRow :to="{ name: 'library-entry', params: { entryId: e.id } }" :title="e.name" :subtitle="kindNames[e.kind]" />
        </li>
      </ul>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.shared {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h1 {
  margin: 0;
  font-family: var(--font-display);
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.g-list {
  gap: 0;
}
</style>
