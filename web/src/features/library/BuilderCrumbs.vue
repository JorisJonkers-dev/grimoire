<script setup lang="ts">
// Where a builder is: in the Library, on one entry, in this builder.
defineProps<{ entryId: string; here: string }>()
</script>

<template>
  <nav aria-label="Breadcrumb" class="g-crumbs">
    <RouterLink :to="{ name: 'library' }">Library</RouterLink> <span aria-hidden="true">›</span>
    <RouterLink :to="{ name: 'library-entry', params: { entryId } }" data-testid="to-entry">Entry</RouterLink> <span aria-hidden="true">›</span>
    <span aria-current="page">{{ here }}</span>
  </nav>
</template>
