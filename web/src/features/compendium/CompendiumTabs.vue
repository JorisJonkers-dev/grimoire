<script setup lang="ts">
import { entryKinds } from './kinds'

defineProps<{ current: string }>()
</script>

<template>
  <nav class="tabs" aria-label="Compendium">
    <RouterLink :to="{ name: 'spells' }" :aria-current="current === 'spell' ? 'page' : undefined">Spells</RouterLink>
    <RouterLink
      v-for="k in entryKinds"
      :key="k.kind"
      :to="{ name: 'entries', params: { kind: k.kind } }"
      :aria-current="current === k.kind ? 'page' : undefined"
    >
      {{ k.label }}
    </RouterLink>
    <RouterLink :to="{ name: 'guides' }" :aria-current="current === 'guides' ? 'page' : undefined">Guides</RouterLink>
  </nav>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 4px;
  overflow-x: auto;
  border-bottom: 1px solid var(--color-line);
  scrollbar-width: thin;
}
.tabs a {
  flex: none;
  display: inline-flex;
  align-items: center;
  min-height: 44px;
  padding: 0 14px;
  margin-bottom: -1px;
  border-bottom: 2px solid transparent;
  color: var(--color-text-2);
  text-decoration: none;
  white-space: nowrap;
}
.tabs a[aria-current='page'] {
  border-bottom-color: var(--color-gold);
  color: var(--color-gold-high);
}
</style>
