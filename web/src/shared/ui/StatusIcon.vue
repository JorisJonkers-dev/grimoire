<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{ slug: string; label: string; size?: number }>(), { size: 14 })

// The canvas status set: a colour and a glyph per condition or effect; anything else is a plain dot.
const glyphs: Record<string, [string, string]> = {
  bless: ['#E8C66A', 'M12 8a4 4 0 1 0 0 8a4 4 0 1 0 0-8M12 3V6M12 18V21M3 12H6M18 12H21M5.6 5.6L7.7 7.7M16.3 16.3L18.4 18.4M5.6 18.4L7.7 16.3M16.3 7.7L18.4 5.6'],
  poisoned: ['#8DBF6A', 'M12 3C15.5 8 18 11 18 14.5A6 6 0 0 1 6 14.5C6 11 8.5 8 12 3Z'],
  prone: ['#BFB199', 'M4 18H20M7 14L12 9L17 14'],
  frightened: ['#A68BD0', 'M12 3C7 3 4 6.5 4 11C4 14 5.5 15.5 7 16.5V20H17V16.5C18.5 15.5 20 14 20 11C20 6.5 17 3 12 3Z'],
  blinded: ['#BFB199', 'M2 12C5 6 19 6 22 12C19 18 5 18 2 12ZM3 21L21 3'],
  charmed: ['#E08AA8', 'M12 20C5 15 3 11 3 8A4.5 4.5 0 0 1 12 6A4.5 4.5 0 0 1 21 8C21 11 19 15 12 20Z'],
  deafened: ['#BFB199', 'M8 9A4 4 0 0 1 16 9C16 13 12 13 12 17A2 2 0 0 1 8 17M3 21L21 3'],
  exhaustion: ['#B79A6A', 'M5 6H12L5 13H12M13 12H19L13 18H19'],
  grappled: ['#C99A6A', 'M5 12A3 3 0 0 1 5 6H11A3 3 0 0 1 11 12M13 12A3 3 0 0 0 13 18H19A3 3 0 0 0 19 12'],
  incapacitated: ['#D07A5A', 'M5 5L19 19M19 5L5 19'],
  invisible: ['#93B5D3', 'M4 12A8 8 0 0 1 12 4M16 5.5A8 8 0 0 1 19.5 9M20 13A8 8 0 0 1 15 19.5M10 19.8A8 8 0 0 1 4.6 15'],
  paralyzed: ['#7FA8DD', 'M13 2L5 14H11L10 22L19 9H13Z'],
  petrified: ['#A9A39A', 'M4 18L7 8L13 5L19 9L20 17L12 20Z'],
  restrained: ['#C8603F', 'M6 4V20M18 4V20M6 9H18M6 15H18'],
  stunned: ['#E8C66A', 'M12 3L13.5 8L19 8L14.5 11L16 16L12 13L8 16L9.5 11L5 8L10.5 8Z'],
  unconscious: ['#8F826C', 'M4 14A8 8 0 0 0 20 14M7 9H11M13 9H17'],
  'hunters-mark': ['#7FA8DD', 'M12 4a8 8 0 1 0 0 16a8 8 0 1 0 0-16M12 9a3 3 0 1 0 0 6a3 3 0 1 0 0-6'],
  'faerie-fire': ['#B48BE0', 'M12 2C13 7 18 9 18 15A6 6 0 0 1 6 15C6 12 8 10.5 9 8C10 10 11 10.5 11.5 11C12.5 8 12 5 12 2Z'],
}
const glyph = computed(() => glyphs[props.slug] ?? (['#BFB199', 'M12 8a4 4 0 1 0 0 8a4 4 0 1 0 0-8'] as [string, string]))
</script>

<template>
  <svg
    class="status-icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    :stroke="glyph[0]"
    stroke-width="2.4"
    stroke-linecap="round"
    stroke-linejoin="round"
    role="img"
    :aria-label="label"
  >
    <title>{{ label }}</title>
    <path :d="glyph[1]" />
  </svg>
</template>

<style scoped>
.status-icon {
  flex: none;
}
</style>
