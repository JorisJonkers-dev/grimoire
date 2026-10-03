<script setup lang="ts">
import type { LiveRosterEntry } from '@/infrastructure/api/types.gen'
import { StatusIcon } from '@/shared/ui'
import { effectLabel } from './conditions'

defineProps<{ entry: LiveRosterEntry }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <section class="card g-card" role="dialog" :aria-label="`Effects on ${entry.label}`" data-testid="effects-card">
    <header class="head">
      <h2>{{ entry.label }}</h2>
      <button type="button" class="close" aria-label="Close" data-testid="effects-card-close" @click="emit('close')">×</button>
    </header>
    <ul class="g-list">
      <li v-for="e in entry.effects" :key="e.id" class="effect">
        <StatusIcon :slug="e.slug" :label="effectLabel(e)" :icon="e.icon" :color="e.color" :size="18" />
        <span>{{ effectLabel(e, true) }}</span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: min(320px, calc(100vw - 32px));
  background: color-mix(in srgb, var(--color-surface) 88%, transparent);
  backdrop-filter: blur(8px);
}
.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
.close {
  min-width: 44px;
  min-height: 44px;
  border: 0;
  background: transparent;
  color: var(--color-text);
  font-size: 22px;
  cursor: pointer;
}
.effect {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
