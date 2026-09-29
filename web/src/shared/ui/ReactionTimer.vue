<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ secondsLeft: number; total: number }>()
const circumference = 2 * Math.PI * 27
const dash = computed(() => {
  const share = props.total > 0 ? Math.min(Math.max(props.secondsLeft / props.total, 0), 1) : 0
  return `${(share * circumference).toFixed(1)} ${circumference.toFixed(1)}`
})
</script>

<template>
  <span class="timer" role="timer" :aria-label="`${secondsLeft} seconds left`">
    <svg width="64" height="64" viewBox="0 0 64 64" aria-hidden="true">
      <circle cx="32" cy="32" r="27" fill="none" stroke="var(--color-line)" stroke-width="6" />
      <circle cx="32" cy="32" r="27" fill="none" stroke="var(--color-gold-high)" stroke-width="6" :stroke-dasharray="dash" transform="rotate(-90 32 32)" stroke-linecap="round" />
      <text x="32" y="39" text-anchor="middle" font-family="Cinzel" font-weight="700" font-size="20" fill="var(--color-text)">{{ secondsLeft }}</text>
    </svg>
  </span>
</template>
