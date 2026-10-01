<script setup lang="ts">
import { nextTick, ref, useId } from 'vue'

/** One tab: its value, its name, and an optional count shown beside it. */
export type Tab = { value: string; label: string; count?: number }

const props = defineProps<{ label: string; tabs: Tab[] }>()
const model = defineModel<string>({ required: true })
const id = useId()
const buttons = ref<HTMLButtonElement[]>([])

async function choose(i: number) {
  model.value = props.tabs[i]?.value ?? model.value
  await nextTick()
  buttons.value[i]?.focus()
}

function key(ev: KeyboardEvent) {
  const at = props.tabs.findIndex((t) => t.value === model.value)
  const n = props.tabs.length
  const to = { ArrowRight: (at + 1) % n, ArrowLeft: (at - 1 + n) % n, Home: 0, End: n - 1 }[ev.key]
  if (to === undefined) return
  ev.preventDefault()
  void choose(to)
}
</script>

<template>
  <div class="g-tabs">
    <div role="tablist" :aria-label="label" class="g-tabs__list">
      <button
        v-for="(t, i) in tabs"
        :id="`${id}-${t.value}`"
        :key="t.value"
        ref="buttons"
        type="button"
        role="tab"
        :aria-selected="t.value === model ? 'true' : 'false'"
        :aria-controls="`${id}-panel`"
        :tabindex="t.value === model ? 0 : -1"
        @click="choose(i)"
        @keydown="key"
      >
        {{ t.label }}<span v-if="t.count !== undefined" class="g-tabs__count">{{ t.count }}</span>
      </button>
    </div>
    <div :id="`${id}-panel`" role="tabpanel" :aria-labelledby="`${id}-${model}`" tabindex="0">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.g-tabs__list {
  display: flex;
  gap: 4px;
  overflow-x: auto;
  border-bottom: 1px solid var(--color-line);
}
button {
  min-height: 44px;
  padding: 0 14px;
  border: 0;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  background: none;
  color: var(--color-text-2);
  font: inherit;
  font-size: 15px;
  white-space: nowrap;
  cursor: pointer;
}
button[aria-selected='true'] {
  border-bottom-color: var(--color-gold);
  color: var(--color-gold-high);
}
button:focus-visible {
  outline: 2px solid var(--color-gold-high);
  outline-offset: -2px;
}
.g-tabs__count {
  margin-left: 6px;
  color: var(--color-text-3);
  font-size: 13px;
}
[role='tabpanel'] {
  padding-top: 16px;
}
[role='tabpanel']:focus-visible {
  outline: none;
}
</style>
