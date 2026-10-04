<script setup lang="ts">
import { onMounted, ref } from 'vue'

// here is what the controls of the page behind name as their keys.
defineProps<{ here: { keys: string; what: string }[] }>()
defineEmits<{ close: [] }>()

// The keys that work everywhere, then the ones the controls on this page name.
const standing = [
  { keys: '?', what: 'Show or hide these keys' },
  { keys: 'Tab', what: 'Move to the next control; with Shift, the one before' },
  { keys: 'Enter', what: 'Press what has the focus; on the map, choose the hex' },
  { keys: 'Arrows', what: 'On the map, move from hex to hex' },
  { keys: '1 to 0', what: 'In a fight, play a tile of the first action bar; with Shift, the second' },
]
const pad = [
  { button: 'D-pad or left stick', what: 'Move the focus; on the map, from hex to hex' },
  { button: 'A', what: 'Press what has the focus' },
  { button: 'B', what: 'Escape: cancel, or put these keys away' },
  { button: 'X', what: 'E: end the turn' },
  { button: 'Y', what: 'M: find your Character on the map' },
  { button: 'LB and RB', what: 'Jump to the previous or next part of the page' },
  { button: 'LT', what: 'C: confirm' },
  { button: 'RT', what: 'R: roll the dice' },
  { button: 'Start', what: 'Show or hide these keys' },
]
const close = ref<HTMLButtonElement | null>(null)
onMounted(() => close.value?.focus())
</script>

<template>
  <div class="veil">
    <section class="keys" role="dialog" aria-modal="true" aria-label="Keys and gamepad" data-testid="key-map">
      <div class="top">
        <h2>Keys</h2>
        <button ref="close" type="button" class="close" data-testid="key-map-close" @click="$emit('close')">Close</button>
      </div>
      <dl>
        <div v-for="row in [...standing, ...here]" :key="`${row.keys} ${row.what}`" class="row" data-testid="key-row">
          <dt><kbd>{{ row.keys }}</kbd></dt>
          <dd>{{ row.what }}</dd>
        </div>
      </dl>
      <h2>Gamepad</h2>
      <dl>
        <div v-for="row in pad" :key="row.button" class="row" data-testid="pad-row">
          <dt><kbd>{{ row.button }}</kbd></dt>
          <dd>{{ row.what }}</dd>
        </div>
      </dl>
    </section>
  </div>
</template>

<style scoped>
.veil {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgb(14 11 9 / 72%);
}
.keys {
  width: min(560px, 100%);
  max-height: 100%;
  overflow-y: auto;
  box-sizing: border-box;
  padding: 18px 20px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
  color: var(--color-text);
}
.top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
h2 {
  margin: 12px 0 6px;
  font-family: var(--font-display);
  font-size: 18px;
  color: var(--color-gold-high);
}
.top h2 {
  margin-top: 0;
}
dl {
  margin: 0;
}
.row {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 12px;
  padding: 5px 0;
  border-bottom: 1px solid var(--color-line);
  font-size: 15px;
}
dd {
  margin: 0;
  color: var(--color-text-2);
}
kbd {
  font-family: var(--font-ui);
  font-weight: 700;
  color: var(--color-text);
}
.close {
  min-height: var(--size-touch-target);
  padding: 0 14px;
  border: 1px solid var(--color-bronze);
  border-radius: var(--radius-sm);
  background: var(--color-raised);
  color: var(--color-text);
  cursor: pointer;
}
</style>
