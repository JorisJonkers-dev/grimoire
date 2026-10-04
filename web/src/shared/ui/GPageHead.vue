<script setup lang="ts">
// The head of a page: a small line saying where it belongs, its name, and what can be done with it.
defineProps<{ eyebrow?: string; title: string }>()
</script>

<template>
  <header class="g-head">
    <div class="g-head__text">
      <span v-if="eyebrow" class="g-eyebrow" data-testid="page-eyebrow">{{ eyebrow }}</span>
      <h1>{{ title }}</h1>
    </div>
    <div v-if="$slots.default" class="g-head__actions">
      <slot />
    </div>
  </header>
</template>

<style scoped>
.g-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}
.g-head__text {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.g-head__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}
</style>
