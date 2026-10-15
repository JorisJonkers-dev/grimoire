<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Part, PartHolds, PartKind } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import PartFields from './PartFields.vue'
import ScalingEditor from './ScalingEditor.vue'
import { freshPart, heading, namedGroups } from './partForm'

// A list of Parts drawn from the served kind definitions: a picker of the kinds this list takes, each
// Part's fields, and the Parts beneath it in their groups, drawn by this same list.
const parts = defineModel<Part[]>({ required: true })
const props = defineProps<{ kinds: PartKind[]; holds: PartHolds; allowed: string[]; label: string; adds: string; noun: string; prefix: string; scales?: boolean }>()

const kindOf = (slug: string) => props.kinds.find((k) => k.slug === slug)
const offered = computed(() => props.allowed.flatMap((slug) => props.kinds.filter((k) => k.slug === slug)))
const chosen = ref('')
const adding = computed({ get: () => chosen.value || (offered.value[0]?.slug ?? ''), set: (v: string) => (chosen.value = v) })
const dragged = ref(-1)

function add() {
  const kind = kindOf(adding.value)
  if (kind) parts.value.push(freshPart(kind, props.holds))
}
function move(from: number, to: number) {
  if (to < 0 || to >= parts.value.length || from === to) return
  const [p] = parts.value.splice(from, 1)
  if (p) parts.value.splice(to, 0, p)
}
function addMode(p: Part) {
  p.groups = [...(p.groups ?? []), { name: '', parts: [] }]
}
function removeMode(p: Part, m: number) {
  p.groups = p.groups?.filter((_, i) => i !== m)
}
const named = (i: number) => `${props.noun} ${String(i + 1)}`
const scalable = (kind: PartKind) => (props.scales ? kind.fields.filter((f) => f.scalable) : [])
</script>

<template>
  <div class="editor">
    <ol class="parts" :aria-label="label" :data-testid="prefix">
      <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the arrows are the accessible path -->
      <li
        v-for="(p, i) in parts"
        :key="i"
        class="part"
        draggable="true"
        :data-testid="`${prefix}-${String(i)}`"
        @dragstart.stop="dragged = i"
        @dragover.prevent
        @drop.prevent.stop="move(dragged, i)"
      >
        <div class="head">
          <span class="kind">{{ kindOf(p.kind)?.label ?? p.kind }}</span>
          <span class="row tools">
            <button type="button" class="tool" :aria-label="`Move ${named(i)} up`" :data-testid="`${prefix}-${String(i)}-up`" @click="move(i, i - 1)">↑</button>
            <button type="button" class="tool" :aria-label="`Move ${named(i)} down`" :data-testid="`${prefix}-${String(i)}-down`" @click="move(i, i + 1)">↓</button>
            <button type="button" class="tool" :aria-label="`Remove ${named(i)}`" :data-testid="`${prefix}-${String(i)}-remove`" @click="parts.splice(i, 1)">×</button>
          </span>
        </div>
        <template v-for="kind in [kindOf(p.kind)]" :key="kind?.slug ?? p.kind">
          <template v-if="kind">
            <PartFields :model-value="p" :kind="kind" :holds="holds" :prefix="`${prefix}-${String(i)}`" />
            <fieldset v-if="scalable(kind).length" class="group">
              <legend>Scaling</legend>
              <ScalingEditor v-model="p.scaling" :fields="scalable(kind)" :label="`${named(i)} scaling`" :prefix="`${prefix}-${String(i)}-scaling`" />
            </fieldset>
            <fieldset v-for="g in namedGroups(p, kind)" :key="g.name" class="group">
              <legend>{{ heading(g.name) }}</legend>
              <PartsEditor
                :model-value="g.parts"
                :kinds="kinds"
                :holds="holds"
                :allowed="holds.beneath"
                :label="heading(g.name)"
                :adds="`Add a Part to ${heading(g.name)}`"
                :noun="`${named(i)}, ${g.name}, part`"
                :prefix="`${prefix}-${String(i)}-${g.name}`"
                :scales="scales"
              />
            </fieldset>
            <template v-if="kind.openGroups">
              <fieldset v-for="(g, m) in p.groups" :key="m" class="group">
                <legend>Mode {{ m + 1 }}</legend>
                <span class="row">
                  <label class="g-field"><span>Name</span><input v-model="g.name" maxlength="80" :data-testid="`${prefix}-${String(i)}-mode-${String(m)}-name`" /></label>
                  <button
                    type="button"
                    class="tool"
                    :aria-label="`Remove mode ${String(m + 1)} of ${named(i)}`"
                    :data-testid="`${prefix}-${String(i)}-mode-${String(m)}-remove`"
                    @click="removeMode(p, m)"
                  >
                    ×
                  </button>
                </span>
                <PartsEditor
                  :model-value="g.parts"
                  :kinds="kinds"
                  :holds="holds"
                  :allowed="holds.beneath"
                  :label="`Mode ${String(m + 1)}`"
                  :adds="`Add a Part to mode ${String(m + 1)}`"
                  :noun="`${named(i)}, mode ${String(m + 1)}, part`"
                  :prefix="`${prefix}-${String(i)}-mode-${String(m)}`"
                  :scales="scales"
                />
              </fieldset>
              <GButton :data-testid="`${prefix}-${String(i)}-add-mode`" @click="addMode(p)">Add a mode</GButton>
            </template>
          </template>
        </template>
      </li>
    </ol>
    <div class="row">
      <label class="g-field"><span>{{ adds }}</span>
        <select v-model="adding" :data-testid="`${prefix}-kind`"><option v-for="k in offered" :key="k.slug" :value="k.slug">{{ k.label }}</option></select>
      </label>
      <GButton :data-testid="`${prefix}-add`" @click="add">Add</GButton>
    </div>
  </div>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.parts {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}
.part {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 0;
  border-top: 1px solid var(--color-rule);
  cursor: grab;
}
.head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.kind {
  min-width: 80px;
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.group {
  margin: 0;
  padding: 0 0 0 14px;
  border: 0;
  border-left: 1px solid var(--color-rule);
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.tools {
  flex-wrap: nowrap;
  flex-shrink: 0;
  margin-left: auto;
}
.tool {
  min-width: 44px;
  min-height: 44px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text);
  cursor: pointer;
}
select {
  min-height: 44px;
  min-width: 0;
}
</style>
