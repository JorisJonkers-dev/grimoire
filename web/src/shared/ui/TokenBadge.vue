<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    name: string
    allegiance: 'party' | 'enemy'
    iconUrl?: string
    hidden?: boolean
    active?: boolean
    size?: number
  }>(),
  { iconUrl: '', hidden: false, active: false, size: 44 },
)

const initials = computed(() =>
  props.name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part.charAt(0).toUpperCase())
    .join(''),
)
const label = computed(() => {
  const who = props.allegiance === 'party' ? 'ally' : 'enemy'
  return `${props.name}, ${who}${props.hidden ? ', hidden' : ''}${props.active ? ', acting now' : ''}`
})
</script>

<template>
  <span
    role="img"
    :aria-label="label"
    :class="['token', `token--${allegiance}`, { 'token--hidden': hidden, 'token--active': active }]"
    :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * 0.38)}px` }"
  >
    <img v-if="iconUrl" :src="iconUrl" alt="" class="icon" />
    <span v-else aria-hidden="true">{{ initials }}</span>
  </span>
</template>

<style scoped>
.token {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  font-family: var(--font-display);
  font-weight: 700;
  color: var(--color-text);
  overflow: hidden;
}
.token--party {
  border-radius: 50%;
  background: var(--color-party-fill);
  border: 4px solid var(--color-party);
}
.token--enemy {
  background: var(--color-enemy-fill);
  border: 4px solid var(--color-enemy);
  clip-path: polygon(50% 0, 100% 25%, 100% 75%, 50% 100%, 0 75%, 0 25%);
}
.token--hidden {
  border-style: dashed;
  opacity: 0.8;
}
.token--active {
  box-shadow: 0 0 0 3px var(--color-ground), 0 0 0 5px var(--color-gold-high);
}
.icon {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
