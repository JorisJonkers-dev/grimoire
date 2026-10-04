<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { BAR_TILES, arrange, masteries, move, pin, stow, tilesOf, type Layout, type Slot } from './actionBar'
import { HOLD_MS } from './motion'

const props = defineProps<{ token: LiveToken; layout: Layout; blocked: string; armed: string; startEditing?: boolean }>()
const emit = defineEmits<{ use: [key: string]; change: [layout: Layout] }>()

const view = computed(() => arrange(props.layout, tilesOf(props.token)))
// The layout as it shows: with the tiles that joined the end, and the stowed ones the token lacks now.
const current = computed<Layout>(() => ({
  bars: view.value.bars.map((b) => b.map((s) => s.key)),
  quick: props.layout.quick,
  stowed: [...new Set([...props.layout.stowed, ...view.value.stowed.map((t) => t.key)])],
}))
const editing = ref(props.startEditing)
const picked = ref('')
// The bar a + asked a tile for; the next tap in the drawer answers it.
const asked = ref<number>()
let dragged = ''
let held = false
let timer: ReturnType<typeof setTimeout> | undefined

function change(next: Layout) {
  picked.value = ''
  asked.value = undefined
  emit('change', next)
}
function play(s: Slot) {
  if (s.tile && !props.blocked) emit('use', s.key)
}
// While editing, a tap picks a tile up and the next tap on a place puts it there.
function tap(s: Slot, bar: number, at: number) {
  if (held) held = false
  else if (!editing.value) play(s)
  else if (picked.value === s.key) picked.value = ''
  else if (picked.value) change(move(current.value, picked.value, bar, at))
  else picked.value = s.key
}
function add(bar: number) {
  if (picked.value) change(move(current.value, picked.value, bar, BAR_TILES))
  else asked.value = bar
}
function fromDrawer(key: string) {
  if (asked.value !== undefined) change(move(current.value, key, asked.value, BAR_TILES))
  else picked.value = picked.value === key ? '' : key
}
function drop(to: (key: string) => Layout) {
  if (dragged) change(to(dragged))
  dragged = ''
}
function press() {
  clearTimeout(timer)
  timer = setTimeout(() => {
    if (!editing.value) held = true
    editing.value = true
  }, HOLD_MS)
}
function release() {
  clearTimeout(timer)
}
function toggle() {
  editing.value = !editing.value
  picked.value = ''
  asked.value = undefined
}

// The keys 1 to 0 play the first bar, with Shift the second.
function onKey(e: KeyboardEvent) {
  const typing = e.target instanceof HTMLElement && ['INPUT', 'SELECT', 'TEXTAREA'].includes(e.target.tagName)
  const digit = /^Digit(\d)$/.exec(e.code)?.[1]
  if (editing.value || typing || e.ctrlKey || e.metaKey || e.altKey || digit === undefined) return
  const slot = view.value.bars[e.shiftKey ? 1 : 0]?.[(Number(digit) + 9) % 10]
  if (slot) play(slot)
}
onMounted(() => { window.addEventListener('keydown', onKey) })
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  clearTimeout(timer)
})
</script>

<template>
  <section :class="['bars', { 'bars--editing': editing }]" :aria-label="`${token.label}'s action bars`" data-testid="action-bars">
    <!-- Dropping is the mouse's shortcut; tapping a tile and then its place does the same from the keyboard. -->
    <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -->
    <ol
      v-for="(bar, b) in view.bars"
      :key="b"
      class="bar"
      :aria-label="`Bar ${String(b + 1)}`"
      :data-testid="`bar-${String(b + 1)}`"
      @dragover.prevent
      @drop="drop((key) => move(current, key, b, BAR_TILES))"
    >
      <li v-for="(s, i) in bar" :key="s.key" class="place">
        <button
          type="button"
          :class="['tile', { 'tile--greyed': !s.tile, 'tile--armed': !editing && armed === s.key, 'tile--picked': picked === s.key }]"
          :disabled="!editing && (!s.tile || blocked !== '')"
          :draggable="editing"
          :aria-pressed="editing ? picked === s.key : armed === s.key"
          :title="s.tile ? blocked || s.tile.tip : `${token.label} does not have ${s.name} now.`"
          :data-testid="s.tile?.testid ?? `missing-${s.key}`"
          @click="tap(s, b, i)"
          @pointerdown="press"
          @pointerup="release"
          @pointerleave="release"
          @dragstart="dragged = s.key"
          @dragover.prevent
          @drop.stop="drop((key) => move(current, key, b, i))"
        >
          <span class="face">
            <span class="name">{{ s.name }}</span>
            <span v-if="s.tile?.detail" class="stat">{{ s.tile.detail }}</span>
            <span v-if="s.tile?.mastery" class="g-tag mastery" :title="masteries[s.tile.mastery]">{{ s.tile.mastery }}</span>
          </span>
          <kbd v-if="b === 0 && !editing" class="key">{{ (i + 1) % 10 }}</kbd>
        </button>
        <span v-if="editing" class="tools">
          <button type="button" class="mini" :aria-label="`Put ${s.name} away`" :data-testid="`stow-${s.key}`" @click="change(stow(current, s.key))">×</button>
          <button
            type="button"
            class="mini"
            :aria-pressed="layout.quick.includes(s.key)"
            :aria-label="`${s.name} on the quick bar`"
            :data-testid="`pin-${s.key}`"
            @click="change(pin(current, s.key))"
          >
            ★
          </button>
        </span>
      </li>
      <li v-if="editing && bar.length < BAR_TILES" class="place">
        <button type="button" :class="['tile', 'tile--add', { 'tile--picked': asked === b }]" :aria-label="`Add a tile to bar ${String(b + 1)}`" :data-testid="`add-to-bar-${String(b + 1)}`" @click="add(b)">
          +
        </button>
      </li>
    </ol>
    <GButton class="edit" :aria-pressed="editing" data-testid="edit-bars" @click="toggle">{{ editing ? 'Done' : 'Edit bars' }}</GButton>
    <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -->
    <div v-if="editing" class="drawer" role="group" aria-label="Tiles put away" data-testid="bar-drawer" @dragover.prevent @drop="drop((key) => stow(current, key))">
      <p class="hint">Drag a tile, or tap one and then where it goes. Hold a tile to edit; ★ puts it on the quick bar.</p>
      <button
        v-for="t in view.stowed"
        :key="t.key"
        type="button"
        :class="['tile', { 'tile--picked': picked === t.key }]"
        draggable="true"
        :aria-pressed="picked === t.key"
        :title="t.tip"
        :data-testid="`stowed-${t.key}`"
        @click="fromDrawer(t.key)"
        @dragstart="dragged = t.key"
      >
        <span class="name">{{ t.name }}</span>
      </button>
    </div>
    <Teleport defer to="#quick-bar-slot">
      <div v-if="view.quick.length" class="quick" role="group" :aria-label="`${token.label}'s quick bar`" data-testid="quick-bar">
        <button
          v-for="s in view.quick"
          :key="s.key"
          type="button"
          :class="['tile', { 'tile--greyed': !s.tile, 'tile--armed': armed === s.key }]"
          :disabled="!s.tile || blocked !== ''"
          :title="s.tile?.tip"
          :data-testid="`quick-${s.tile?.testid ?? s.key}`"
          @click="play(s)"
        >
          <span class="name">{{ s.name }}</span>
        </button>
      </div>
    </Teleport>
  </section>
</template>

<style scoped>
.bars {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.bar,
.drawer,
.quick {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.bar {
  min-height: 44px;
}
.place {
  position: relative;
  display: flex;
}
.tile {
  position: relative;
  display: grid;
  align-content: center;
  justify-items: start;
  min-width: 64px;
  min-height: 44px;
  padding: 2px 10px;
  border: 1px solid var(--color-edge);
  border-radius: var(--radius-control);
  color: var(--color-text);
  background: var(--color-raised);
  cursor: pointer;
}
.tile:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
.tile--greyed {
  border-style: dashed;
}
.tile--armed,
.tile--picked {
  border-color: var(--color-brass-edge);
  background: var(--color-selected);
  outline: 2px solid var(--color-gold-high);
  outline-offset: -1px;
}
.tile--add {
  justify-items: center;
  min-width: 48px;
  font-size: 22px;
  border-style: dashed;
}
.face {
  display: grid;
  justify-items: start;
}
.name {
  font-family: var(--font-label);
  font-size: 14px;
  line-height: 1.2;
}
.stat {
  font-size: 11px;
  line-height: 1.2;
  color: var(--color-text-2);
}
.mastery {
  font-size: 11px;
  text-transform: capitalize;
}
.key {
  position: absolute;
  top: 2px;
  right: 6px;
  font-size: 11px;
  color: var(--color-text-2);
}
.tools {
  position: absolute;
  top: -8px;
  right: -6px;
  display: flex;
  gap: 2px;
}
.mini {
  min-width: 24px;
  min-height: 24px;
  padding: 0;
  border: 1px solid var(--color-line);
  border-radius: 50%;
  color: var(--color-text);
  background: var(--color-surface);
  cursor: pointer;
}
.mini[aria-pressed='true'] {
  border-color: var(--color-gold-high);
  color: var(--color-gold-high);
}
.edit {
  align-self: flex-start;
}
.drawer {
  padding: 8px;
  border: 1px dashed var(--color-line);
  border-radius: var(--radius-md);
}
.hint {
  flex: 1 0 100%;
  margin: 0;
  color: var(--color-text-2);
}
/* The face wiggles inside a tile that stays put, so a tile is as easy to grab as it looks. */
.bars--editing .bar .face {
  animation: wiggle 320ms ease-in-out infinite alternate;
}
@keyframes wiggle {
  from {
    transform: rotate(-1.2deg);
  }
  to {
    transform: rotate(1.2deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .bars--editing .bar .face {
    animation: none;
  }
}
/* The quick bar is for phones, where the bars sit below the map. */
.quick {
  justify-content: center;
  padding: 6px;
}
@media (min-width: 900px) {
  .quick {
    display: none;
  }
}
</style>
