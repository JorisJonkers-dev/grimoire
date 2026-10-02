<script setup lang="ts">
import { computed } from 'vue'
import { encode } from 'uqr'

const props = defineProps<{ value: string; label: string }>()
// One path of unit squares, dark on a light quiet zone so any camera reads it in either theme.
const qr = computed(() => {
  const { data, size } = encode(props.value, { ecc: 'M', border: 2 })
  let path = ''
  data.forEach((row, y) => {
    row.forEach((dark, x) => {
      if (dark) path += `M${String(x)} ${String(y)}h1v1h-1z`
    })
  })
  return { path, size }
})
</script>

<template>
  <svg class="qr" role="img" :aria-label="label" :viewBox="`0 0 ${qr.size} ${qr.size}`" shape-rendering="crispEdges">
    <rect width="100%" height="100%" fill="#fff" />
    <path :d="qr.path" fill="#000" />
  </svg>
</template>

<style scoped>
.qr {
  width: 200px;
  max-width: 100%;
  height: auto;
  border-radius: 8px;
}
</style>
