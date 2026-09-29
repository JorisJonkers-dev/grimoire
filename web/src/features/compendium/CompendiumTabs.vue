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
  </nav>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 6px;
  overflow-x: auto;
  padding-bottom: 4px;
  scrollbar-width: thin;
}
.tabs a {
  flex: none;
  display: inline-flex;
  align-items: center;
  min-height: 40px;
  padding: 0 14px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-pill);
  color: var(--color-text);
  text-decoration: none;
  white-space: nowrap;
}
.tabs a[aria-current='page'] {
  border-color: var(--color-gold);
  color: var(--color-gold-high);
}
</style>
