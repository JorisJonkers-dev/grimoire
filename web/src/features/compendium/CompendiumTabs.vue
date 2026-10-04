<script setup lang="ts">
import { entryKinds } from './kinds'

defineProps<{ current: string }>()
</script>

<template>
  <nav class="tabs" aria-label="Compendium">
    <RouterLink :to="{ name: 'spells' }" :aria-current="current === 'spell' ? 'page' : undefined"><span class="mark" aria-hidden="true" />Spells</RouterLink>
    <RouterLink
      v-for="k in entryKinds"
      :key="k.kind"
      :to="{ name: 'entries', params: { kind: k.kind } }"
      :aria-current="current === k.kind ? 'page' : undefined"
    >
      <span class="mark" aria-hidden="true" />{{ k.label }}
    </RouterLink>
    <RouterLink :to="{ name: 'guides' }" :aria-current="current === 'guides' ? 'page' : undefined"><span class="mark" aria-hidden="true" />Guides</RouterLink>
  </nav>
</template>

<style scoped>
/* The kinds of the Compendium: a diamond and a gold rule mark the one that is open. */
.tabs {
  display: flex;
  gap: 6px;
  overflow-x: auto;
  border-bottom: 1px solid var(--color-rule);
  scrollbar-width: thin;
}
.tabs a {
  flex: none;
  display: flex;
  align-items: center;
  gap: 9px;
  height: 44px;
  padding: 0 14px;
  color: var(--color-text-2);
  font-family: var(--font-label);
  font-size: 17px;
  white-space: nowrap;
}
.tabs a:hover {
  color: var(--color-gold-high);
}
.mark {
  display: none;
  width: 7px;
  height: 7px;
  background: var(--color-gold-high);
  transform: rotate(45deg);
}
.tabs a[aria-current='page'] {
  padding-left: 4px;
  box-shadow: inset 0 -2px 0 var(--color-gold);
  color: var(--color-gold-high);
}
.tabs a[aria-current='page'] .mark {
  display: block;
}
@media (max-width: 899px) {
  .tabs a {
    font-size: 15px;
    padding: 0 10px;
  }
}
</style>
