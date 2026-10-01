<script setup lang="ts">
import { RouterLink, type RouteLocationRaw } from 'vue-router'

withDefaults(defineProps<{ title: string; subtitle?: string; to?: RouteLocationRaw }>(), { subtitle: '', to: undefined })
defineEmits<{ click: [] }>()
</script>

<template>
  <component :is="to ? RouterLink : 'button'" :to="to" :type="to ? undefined : 'button'" class="g-row" @click="to ? undefined : $emit('click')">
    <slot name="leading" />
    <span class="g-row__text">
      <span class="g-row__title">{{ title }}</span>
      <span v-if="subtitle" class="g-row__subtitle">{{ subtitle }}</span>
    </span>
    <slot name="trailing" />
    <svg class="g-row__chevron" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="M6 3l5 5-5 5" fill="none" stroke="currentColor" stroke-width="1.6" /></svg>
  </component>
</template>

<style scoped>
.g-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 56px;
  padding: 8px 4px 8px 12px;
  box-sizing: border-box;
  border: 0;
  border-bottom: 1px solid var(--color-line);
  background: none;
  color: var(--color-text);
  font: inherit;
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}
.g-row:hover,
.g-row:focus-visible {
  background: var(--color-surface);
}
.g-row:focus-visible {
  outline: 2px solid var(--color-gold-high);
  outline-offset: -2px;
}
.g-row__text {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}
.g-row__title {
  font-size: 16px;
  font-weight: 700;
}
.g-row__subtitle {
  color: var(--color-text-2);
  font-size: 14px;
}
.g-row__chevron {
  flex: none;
  color: var(--color-bronze);
}
</style>
