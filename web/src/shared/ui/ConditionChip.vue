<script setup lang="ts">
withDefaults(defineProps<{ label: string; tone?: 'boon' | 'bane'; detail?: string }>(), { tone: 'boon', detail: '' })
</script>

<template>
  <span :class="['chip', `chip--${tone}`]">
    <span class="dot" aria-hidden="true" />
    <span>{{ label }}<template v-if="detail"> · {{ detail }}</template></span>
  </span>
</template>

<style scoped>
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: var(--radius-pill);
  font-size: 14px;
}
.chip--boon {
  background: var(--color-raised);
  border: 1px solid var(--color-bronze);
}
.chip--bane {
  background: var(--color-enemy-fill);
  border: 1px solid var(--color-enemy);
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-gold-high);
}
.chip--bane .dot {
  background: var(--color-enemy-soft);
  border-radius: 1px;
  transform: rotate(45deg);
}
</style>
