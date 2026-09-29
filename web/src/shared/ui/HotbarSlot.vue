<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{ label: string; kind?: 'action' | 'bonus'; available?: boolean; reason?: string; suggested?: boolean }>(),
  { kind: 'action', available: true, reason: '', suggested: false },
)
defineEmits<{ use: [] }>()

const accessibleName = computed(() => {
  const economy = props.kind === 'action' ? 'action' : 'bonus action'
  const state = props.available ? '' : `, unavailable: ${props.reason || 'not now'}`
  const hint = props.suggested ? ', suggested' : ''
  return `${props.label}, ${economy}${hint}${state}`
})
</script>

<template>
  <button
    type="button"
    :class="['slot', `slot--${kind}`, { 'slot--off': !available, 'slot--suggested': suggested }]"
    :disabled="!available"
    :aria-label="accessibleName"
    :title="available ? undefined : reason"
    @click="$emit('use')"
  >
    <span v-if="suggested" class="badge" aria-hidden="true">Suggested</span>
    <span class="icon" aria-hidden="true"><slot name="icon" /></span>
    <span class="label">{{ label }}</span>
  </button>
</template>

<style scoped>
.slot {
  position: relative;
  width: 72px;
  height: 72px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  border-radius: var(--radius-lg);
  background: var(--color-raised);
  color: var(--color-text);
  font-family: var(--font-ui);
  font-size: 12px;
  cursor: pointer;
}
.slot--action {
  border: 1px solid var(--color-gold);
}
.slot--bonus {
  border: 1px solid var(--color-bronze);
}
.slot--off {
  border: 1px dashed var(--color-line);
  background: #140f0b;
  color: var(--color-text-3);
  cursor: not-allowed;
}
.slot--suggested {
  border: 2px solid var(--color-enemy);
}
.badge {
  position: absolute;
  top: -9px;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background: var(--color-enemy);
  color: #1a130c;
  font-size: 10px;
  font-weight: 800;
  text-transform: uppercase;
}
.icon {
  display: flex;
  width: 24px;
  height: 24px;
}
.slot:focus-visible {
  outline: 3px solid var(--color-gold-high);
  outline-offset: 2px;
}
</style>
