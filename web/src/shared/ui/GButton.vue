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
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: var(--size-control);
  padding: 0 18px;
  border-radius: var(--radius-control);
  font-family: var(--font-label);
  font-size: 16px;
  cursor: pointer;
  transition: filter var(--motion-max);
}
.g-button:hover:not(:disabled) {
  filter: brightness(1.12);
}
/* Out of use it is drawn hollow, with a dashed edge, rather than faded: faded text is hard to read. */
.g-button:disabled {
  border: 1px dashed var(--color-line);
  background: var(--color-inset);
  color: var(--color-text-3);
  cursor: not-allowed;
}
.g-button--primary {
  border: 1px solid var(--color-brass-edge);
  background: var(--color-brass);
  color: var(--color-brass-text);
}
.g-button--secondary {
  border: 1px solid var(--color-edge);
  background: var(--color-raised);
  color: var(--color-text);
}
.g-button--danger {
  border: 1px solid var(--color-danger-edge);
  background: var(--color-danger-fill);
  color: var(--color-danger-text);
}
</style>
