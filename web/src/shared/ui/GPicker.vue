<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'

/** One choice a picker offers. */
export type PickerOption = { value: string; label: string; hint?: string }

const props = withDefaults(
  defineProps<{
    label: string
    options: PickerOption[]
    /** Asks the server for matches once typing settles; its answers replace the local ones. */
    search?: (query: string) => Promise<PickerOption[]>
    debounceMs?: number
    /** How many characters before the server is asked. */
    minChars?: number
  }>(),
  { search: undefined, debounceMs: 250, minChars: 2 },
)
const model = defineModel<string>({ default: '' })
const emit = defineEmits<{ select: [option: PickerOption] }>()
const id = useId()

const labelOf = (value: string) => props.options.find((o) => o.value === value)?.label ?? ''
const query = ref(labelOf(model.value))
const open = ref(false)
const active = ref(-1)
const remote = ref<PickerOption[] | null>(null)
const state = ref<'idle' | 'searching' | 'failed'>('idle')
let timer: ReturnType<typeof setTimeout> | undefined
let asked = 0

const shown = computed(() => {
  if (remote.value) return remote.value
  const q = query.value.trim().toLowerCase()
  return q === '' ? props.options : props.options.filter((o) => o.label.toLowerCase().includes(q))
})
const empty = computed(() => {
  if (state.value === 'searching') return 'Searching…'
  if (state.value === 'failed') return 'The search failed. Try again.'
  return `Nothing matches “${query.value.trim()}”.`
})

watch(model, (v) => {
  const label = labelOf(v)
  if (label) query.value = label
})

function ask(q: string) {
  clearTimeout(timer)
  const search = props.search
  if (!search || q.trim().length < props.minChars) {
    remote.value = null
    state.value = 'idle'
    return
  }
  state.value = 'searching'
  remote.value = []
  timer = setTimeout(() => {
    const mine = ++asked
    search(q.trim()).then(
      (found) => {
        if (mine !== asked) return
        remote.value = found
        state.value = 'idle'
      },
      () => {
        if (mine !== asked) return
        state.value = 'failed'
      },
    )
  }, props.debounceMs)
}

function typed(ev: Event) {
  query.value = (ev.target as HTMLInputElement).value
  open.value = true
  active.value = -1
  ask(query.value)
}

function pick(o: PickerOption) {
  model.value = o.value
  query.value = o.label
  open.value = false
  emit('select', o)
}

function key(ev: KeyboardEvent) {
  const n = shown.value.length
  switch (ev.key) {
    case 'ArrowDown':
      ev.preventDefault()
      if (!open.value) open.value = true
      else if (n) active.value = (active.value + 1) % n
      break
    case 'ArrowUp':
      ev.preventDefault()
      if (n) active.value = active.value <= 0 ? n - 1 : active.value - 1
      break
    case 'Enter': {
      const o = shown.value[active.value] ?? (n === 1 ? shown.value[0] : undefined)
      if (open.value && o) {
        ev.preventDefault()
        pick(o)
      }
      break
    }
    case 'Escape':
      open.value = false
      break
  }
}

onBeforeUnmount(() => {
  clearTimeout(timer)
})
</script>

<template>
  <div :class="['g-picker', { 'g-picker--open': open }]">
    <input
      :id="id"
      :value="query"
      type="text"
      role="combobox"
      placeholder=" "
      autocomplete="off"
      aria-autocomplete="list"
      :aria-expanded="open ? 'true' : 'false'"
      :aria-controls="`${id}-list`"
      :aria-activedescendant="open && active >= 0 ? `${id}-${active}` : undefined"
      @input="typed"
      @focus="open = true"
      @blur="open = false"
      @keydown="key"
    />
    <label :for="id">{{ label }}</label>
    <ul v-if="open" :id="`${id}-list`" role="listbox" :aria-label="label" class="g-picker__list">
      <li
        v-for="(o, i) in shown"
        :id="`${id}-${i}`"
        :key="o.value"
        role="option"
        tabindex="-1"
        :aria-selected="i === active ? 'true' : 'false'"
        :class="{ active: i === active }"
        @mousedown.prevent="pick(o)"
      >
        <span class="g-picker__label">{{ o.label }}</span>
        <span v-if="o.hint" class="g-picker__hint">{{ o.hint }}</span>
      </li>
      <li v-if="shown.length === 0" class="g-picker__empty" role="presentation">{{ empty }}</li>
    </ul>
  </div>
</template>

<style scoped>
.g-picker {
  position: relative;
}
input {
  box-sizing: border-box;
  width: 100%;
  min-height: 52px;
  padding: 22px 12px 6px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  color: var(--color-text);
  font: inherit;
  font-size: 16px;
}
input:focus {
  outline: none;
  border-color: var(--color-gold);
}
label {
  position: absolute;
  top: 16px;
  left: 13px;
  color: var(--color-text-3);
  font-size: 16px;
  pointer-events: none;
  transition:
    top var(--motion-max),
    font-size var(--motion-max);
}
input:focus + label,
input:not(:placeholder-shown) + label {
  top: 6px;
  font-size: 12px;
  color: var(--color-gold-high);
}
/* The list hangs flush from the field: one shape, square where they meet. */
.g-picker--open input {
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
}
.g-picker__list {
  position: absolute;
  z-index: 20;
  top: 100%;
  left: 0;
  right: 0;
  max-height: 280px;
  overflow-y: auto;
  margin: 0;
  padding: 0;
  list-style: none;
  border: 1px solid var(--color-gold);
  border-top: 0;
  border-radius: 0 0 var(--radius-sm) var(--radius-sm);
  background: var(--color-raised);
}
li {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  cursor: pointer;
}
li + li {
  border-top: 1px solid var(--color-line);
}
li.active,
li[role='option']:hover {
  background: var(--color-surface);
  color: var(--color-gold-high);
}
.g-picker__hint,
.g-picker__empty {
  color: var(--color-text-3);
  font-size: 14px;
}
.g-picker__empty {
  cursor: default;
}
</style>
