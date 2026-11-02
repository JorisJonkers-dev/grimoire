<script setup lang="ts">
import { personas, type Persona } from './role'

defineProps<{ persona: Persona }>()
const emit = defineEmits<{ switch: [persona: Persona] }>()
</script>

<template>
  <!-- The demo's own control: look at the same world as each kind of caller. Nothing here is in the real app. -->
  <aside class="demo" aria-label="Demo" data-testid="demo-switcher">
    <span class="tag">Demo</span>
    <fieldset>
      <legend>View as</legend>
      <label v-for="p in personas" :key="p.id" :class="{ on: p.id === persona }">
        <input type="radio" name="demo-persona" :value="p.id" :checked="p.id === persona" :data-testid="`demo-as-${p.id}`" @change="emit('switch', p.id)" />
        {{ p.label }}
      </label>
    </fieldset>
    <span class="note">Nothing leaves your browser; a reload starts afresh.</span>
  </aside>
</template>

<style scoped>
.demo {
  position: fixed;
  z-index: 50;
  right: 12px;
  bottom: 76px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  max-width: calc(100vw - 24px);
  padding: 8px 10px;
  border: 1px solid var(--color-brass-edge);
  border-radius: var(--radius-panel);
  background: var(--color-raised);
  color: var(--color-text);
  font-family: var(--font-ui);
  font-size: 14px;
}
.tag {
  font-family: var(--font-label);
  color: var(--color-gold-high);
}
fieldset {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  float: left;
  margin-right: 4px;
  color: var(--color-text-2);
}
label {
  padding: 2px 8px;
  border: 1px solid var(--color-edge);
  border-radius: var(--radius-chip);
  cursor: pointer;
}
label.on {
  border-color: var(--color-gold);
  background: var(--color-selected);
}
input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
label:focus-within {
  outline: 2px solid var(--color-gold-high);
}
.note {
  flex-basis: 100%;
  color: var(--color-text-3);
  font-size: 12px;
}
</style>
