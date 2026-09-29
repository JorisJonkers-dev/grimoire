<script setup lang="ts">
withDefaults(defineProps<{ variant?: 'primary' | 'secondary' | 'danger'; type?: 'button' | 'submit'; disabled?: boolean }>(), {
  variant: 'secondary',
  type: 'button',
  disabled: false,
})
</script>

<template>
  <button :type="type" :disabled="disabled" :class="['g-button', `g-button--${variant}`]">
    <slot />
  </button>
</template>

<style scoped>
.g-button {
  min-height: 48px;
  padding: 0 20px;
  border-radius: var(--radius-md);
  font-family: var(--font-ui);
  font-weight: 700;
  font-size: 16px;
  cursor: pointer;
  transition: filter var(--motion-max);
}
.g-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
.g-button--primary {
  border: 1px solid var(--color-gold-high);
  background: var(--color-gold);
  color: #1a130c;
  font-family: var(--font-display);
  letter-spacing: 0.06em;
}
.g-button--secondary {
  border: 1px solid var(--color-bronze);
  background: var(--color-raised);
  color: var(--color-text);
}
.g-button--danger {
  border: 1px solid var(--color-enemy);
  background: var(--color-enemy-fill);
  color: var(--color-enemy-soft);
}
.g-button:focus-visible {
  outline: 3px solid var(--color-gold-high);
  outline-offset: 2px;
}
</style>
