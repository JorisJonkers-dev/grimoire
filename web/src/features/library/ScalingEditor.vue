<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PartField, PartScaling, PartValue } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { flagValue, numberValue, textValue, words } from './partForm'
import { type Axis, type Rule, axes, classed, firstValue, freshRule, onAxis, rulesFor } from './scalingForm'

// The rules that grow a Part's scalable fields, or a spell's own, drawn from the fields as served.
const rules = defineModel<PartScaling[] | undefined>()
const props = defineProps<{ fields: PartField[]; prefix: string; label: string }>()

const list = computed(() => rules.value ?? [])
const fieldOf = (name: string) => props.fields.find((f) => f.name === name)
const chosen = ref('')
const adding = computed({ get: () => chosen.value || (props.fields[0]?.name ?? ''), set: (v: string) => (chosen.value = v) })

function emit(next: PartScaling[]) {
  rules.value = next.length === 0 ? undefined : next
}
function put(i: number, s: PartScaling) {
  emit(list.value.map((r, j) => (j === i ? s : r)))
}
function touched() {
  emit([...list.value])
}
function add() {
  const f = fieldOf(adding.value)
  if (f) emit([...list.value, freshRule(f, 'slot_level', rulesFor(f, 'slot_level')[0] ?? 'table')])
}
function setField(i: number, s: PartScaling, name: string) {
  const f = fieldOf(name)
  if (f) put(i, freshRule(f, s.axis, rulesFor(f, s.axis)[0] ?? 'table', s.class))
}
function setAxis(i: number, s: PartScaling, axis: Axis) {
  const f = fieldOf(s.field)
  if (f) put(i, onAxis(s, f, axis))
}
function setRule(i: number, s: PartScaling, rule: Rule) {
  const f = fieldOf(s.field)
  if (f) put(i, freshRule(f, s.axis, rule, s.class))
}
function setStepValue(s: PartScaling, n: number, v: PartValue | undefined) {
  const step = s.steps?.[n]
  if (step) step.value = v ?? {}
  touched()
}
function addStep(s: PartScaling) {
  const f = fieldOf(s.field)
  const last = s.steps?.at(-1)?.at ?? 1
  if (f) s.steps = [...(s.steps ?? []), { at: Math.min(last + 1, 20), value: firstValue(f) }]
  touched()
}
function removeStep(s: PartScaling, n: number) {
  s.steps = s.steps?.filter((_, j) => j !== n)
  touched()
}
const typed = (e: Event) => (e.target as HTMLInputElement).value
const ticked = (e: Event) => (e.target as HTMLInputElement).checked
const named = (i: number) => `${props.label} rule ${String(i + 1)}`
</script>

<template>
  <div class="scaling">
    <fieldset v-for="(s, i) in list" :key="i" class="rule" :data-testid="`${prefix}-${String(i)}`">
      <legend>{{ named(i) }}</legend>
      <template v-for="f in [fieldOf(s.field)]" :key="f?.name ?? s.field">
        <div class="row">
          <label class="g-field"><span>Grows</span>
            <select :value="s.field" :data-testid="`${prefix}-${String(i)}-field`" @change="setField(i, s, typed($event))">
              <option v-for="o in fields" :key="o.name" :value="o.name">{{ o.label }}</option>
            </select>
          </label>
          <label class="g-field"><span>With</span>
            <select :value="s.axis" :data-testid="`${prefix}-${String(i)}-axis`" @change="setAxis(i, s, typed($event) as Axis)">
              <option v-for="a in axes" :key="a" :value="a">{{ words(a) }}</option>
            </select>
          </label>
          <label v-if="classed(s.axis)" class="g-field"><span>Class</span>
            <input :value="s.class ?? ''" maxlength="80" :data-testid="`${prefix}-${String(i)}-class`" @input="s.class = typed($event); touched()" />
          </label>
          <label class="g-field"><span>Rule</span>
            <select :value="s.rule" :data-testid="`${prefix}-${String(i)}-rule`" @change="setRule(i, s, typed($event) as Rule)">
              <option v-for="r in rulesFor(f, s.axis)" :key="r" :value="r">{{ r }}</option>
            </select>
          </label>
          <button type="button" class="tool" :aria-label="`Remove ${named(i)}`" :data-testid="`${prefix}-${String(i)}-remove`" @click="emit(list.filter((_, j) => j !== i))">×</button>
        </div>
        <div v-if="s.rule === 'linear'" class="row">
          <label class="g-field"><span>For each level above</span>
            <input type="number" min="0" max="20" :value="s.base ?? 0" :data-testid="`${prefix}-${String(i)}-base`" @input="s.base = Number(typed($event)); touched()" />
          </label>
          <label class="g-field"><span>Add</span>
            <input
              v-if="f?.type === 'number'"
              type="number"
              :value="s.per?.number ?? ''"
              :data-testid="`${prefix}-${String(i)}-per`"
              @input="s.per = numberValue(typed($event)); touched()"
            />
            <input v-else :value="s.per?.text ?? ''" maxlength="8" :data-testid="`${prefix}-${String(i)}-per`" @input="s.per = textValue(typed($event)); touched()" />
          </label>
        </div>
        <label v-else-if="s.rule === 'formula'" class="g-field"><span>Formula of level</span>
          <input :value="s.formula ?? ''" maxlength="60" :data-testid="`${prefix}-${String(i)}-formula`" @input="s.formula = typed($event); touched()" />
        </label>
        <label v-else-if="s.rule === 'column'" class="g-field"><span>Column</span>
          <input :value="s.column ?? ''" maxlength="80" :data-testid="`${prefix}-${String(i)}-column`" @input="s.column = typed($event); touched()" />
        </label>
        <ol v-else class="steps" :aria-label="`Steps of ${named(i)}`">
          <li v-for="(st, n) in s.steps ?? []" :key="n" class="row">
            <label class="g-field"><span>From level</span>
              <input type="number" min="1" max="20" :value="st.at" :data-testid="`${prefix}-${String(i)}-step-${String(n)}-at`" @input="st.at = Number(typed($event)); touched()" />
            </label>
            <label v-if="f?.type === 'flag'" class="check">
              <input type="checkbox" :checked="st.value.flag ?? false" :data-testid="`${prefix}-${String(i)}-step-${String(n)}-value`" @change="setStepValue(s, n, flagValue(ticked($event)))" />
              <span>{{ f.label }}</span>
            </label>
            <label v-else class="g-field"><span>{{ f?.label ?? s.field }}</span>
              <select v-if="f?.type === 'choice'" :value="st.value.text ?? ''" :data-testid="`${prefix}-${String(i)}-step-${String(n)}-value`" @change="setStepValue(s, n, textValue(typed($event)))">
                <option v-for="o in f.options" :key="o" :value="o">{{ words(o) }}</option>
              </select>
              <input
                v-else-if="f?.type === 'number'"
                type="number"
                :min="f.min"
                :max="f.max"
                :value="st.value.number ?? ''"
                :data-testid="`${prefix}-${String(i)}-step-${String(n)}-value`"
                @input="setStepValue(s, n, numberValue(typed($event)))"
              />
              <input v-else :value="st.value.text ?? ''" maxlength="8" :data-testid="`${prefix}-${String(i)}-step-${String(n)}-value`" @input="setStepValue(s, n, textValue(typed($event)))" />
            </label>
            <button
              type="button"
              class="tool"
              :aria-label="`Remove step ${String(n + 1)} of ${named(i)}`"
              :data-testid="`${prefix}-${String(i)}-step-${String(n)}-remove`"
              @click="removeStep(s, n)"
            >
              ×
            </button>
          </li>
          <li><GButton :data-testid="`${prefix}-${String(i)}-add-step`" @click="addStep(s)">Add a step</GButton></li>
        </ol>
      </template>
    </fieldset>
    <div v-if="fields.length" class="row">
      <label class="g-field"><span>Add a rule to grow</span>
        <select v-model="adding" :data-testid="`${prefix}-add-field`"><option v-for="f in fields" :key="f.name" :value="f.name">{{ f.label }}</option></select>
      </label>
      <GButton :data-testid="`${prefix}-add`" @click="add">Add a rule</GButton>
    </div>
  </div>
</template>

<style scoped>
.scaling,
.steps {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.steps {
  margin: 0;
  padding: 0;
  list-style: none;
}
.rule {
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
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
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
input,
select {
  min-height: 44px;
  min-width: 0;
}
</style>
