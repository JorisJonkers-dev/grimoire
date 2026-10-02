<script setup lang="ts">
import GField from './GField.vue'

withDefaults(defineProps<{ label?: string }>(), { label: 'Code from your authenticator app' })
const model = defineModel<string>({ default: '' })
// Keep only digits, so a pasted "123 456" still fits the six places.
function digits(v: string | number) {
  model.value = String(v).replace(/\D/g, '').slice(0, 6)
}
</script>

<template>
  <GField
    :model-value="model"
    :label="label"
    :maxlength="6"
    autocomplete="one-time-code"
    inputmode="numeric"
    pattern="[0-9]*"
    enterkeyhint="done"
    class="code"
    @update:model-value="digits"
  />
</template>

<style scoped>
.code :deep(input) {
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.3em;
}
</style>
