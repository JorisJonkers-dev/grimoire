<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{ name: string; src?: string; size?: number }>(), { src: '', size: 40 })
const initials = computed(
  () =>
    props.name
      .trim()
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase())
      .join('') || '?',
)
</script>

<template>
  <span class="g-avatar" :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * 0.4)}px` }">
    <img v-if="src" :src="src" :alt="name" />
    <span v-else role="img" :aria-label="name">{{ initials }}</span>
  </span>
</template>

<style scoped>
.g-avatar {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--color-bronze);
  border-radius: 50%;
  background: var(--color-raised);
  color: var(--color-gold-high);
  font-family: var(--font-display);
}
img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
