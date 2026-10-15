<script setup lang="ts">
import { computed, useId } from 'vue'
import type { Part, PartField, PartHolds, PartKind, PartValue } from '@/infrastructure/api/types.gen'
import { flagValue, helpOf, narrowed, numberValue, setValue, shownFields, textValue, toggled, words } from './partForm'

// One Part's fields, as its kind describes them and its hints arrange them.
const part = defineModel<Part>({ required: true })
const props = defineProps<{ kind: PartKind; holds: PartHolds; prefix: string }>()
const id = useId()
const sections = computed(() => shownFields(props.kind, props.holds.fills))
const options = (f: PartField) => narrowed(props.holds, props.kind.slug, f.name) ?? f.options
const picked = (f: PartField) => f.type === 'choice' || narrowed(props.holds, props.kind.slug, f.name) !== undefined
const help = (f: PartField) => helpOf(props.kind, f.name)
const helpId = (f: PartField) => (help(f) === undefined ? undefined : `${id}-${f.name}`)
const value = (f: PartField) => part.value.values[f.name]
function set(f: PartField, v: PartValue | undefined) {
  setValue(part.value, f.name, v)
}
const typed = (e: Event) => (e.target as HTMLInputElement).value
const ticked = (e: Event) => (e.target as HTMLInputElement).checked
</script>

<template>
  <div class="fields">
    <component :is="s.label === undefined ? 'div' : 'fieldset'" v-for="(s, i) in sections" :key="i" :class="s.label === undefined ? 'loose' : 'section'">
      <legend v-if="s.label !== undefined">{{ s.label }}</legend>
      <div v-for="f in s.fields" :key="f.name" :class="['field', { wide: f.type === 'text' && f.max > 80, many: f.type === 'choices' }]">
        <label v-if="f.type === 'flag'" class="check">
          <input
            type="checkbox"
            :checked="value(f)?.flag ?? false"
            :aria-describedby="helpId(f)"
            :data-testid="`${prefix}-${f.name}`"
            @change="set(f, flagValue(ticked($event)))"
          />
          <span>{{ f.label }}</span>
        </label>
        <fieldset v-else-if="f.type === 'choices'" class="types" :aria-describedby="helpId(f)">
          <legend>{{ f.label }}</legend>
          <label v-for="o in options(f)" :key="o" class="check">
            <input type="checkbox" :checked="value(f)?.texts?.includes(o) ?? false" :data-testid="`${prefix}-${f.name}-${o}`" @change="set(f, toggled(value(f), o))" />
            <span>{{ words(o) }}</span>
          </label>
        </fieldset>
        <label v-else class="g-field">
          <span>{{ f.label }}</span>
          <select v-if="picked(f)" :value="value(f)?.text ?? ''" :aria-describedby="helpId(f)" :data-testid="`${prefix}-${f.name}`" @change="set(f, textValue(typed($event)))">
            <option v-if="!f.required" value="">None</option>
            <option v-for="o in options(f)" :key="o" :value="o">{{ words(o) }}</option>
          </select>
          <input
            v-else-if="f.type === 'number'"
            type="number"
            :min="f.min"
            :max="f.max"
            :value="value(f)?.number ?? ''"
            :aria-describedby="helpId(f)"
            :data-testid="`${prefix}-${f.name}`"
            @input="set(f, numberValue(typed($event)))"
          />
          <input
            v-else
            :value="value(f)?.text ?? ''"
            :maxlength="f.max > 0 ? f.max : undefined"
            :aria-describedby="helpId(f)"
            :data-testid="`${prefix}-${f.name}`"
            @input="set(f, textValue(typed($event)))"
          />
        </label>
        <small v-if="help(f) !== undefined" :id="helpId(f)" class="help">{{ help(f) }}</small>
      </div>
    </component>
  </div>
</template>

<style scoped>
.fields,
.section {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 8px 12px;
  align-items: start;
}
.loose {
  display: contents;
}
.section {
  grid-column: 1 / -1;
  margin: 0;
  padding: 0;
  border: 0;
}
.section > legend {
  grid-column: 1 / -1;
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.wide,
.many {
  grid-column: 1 / -1;
}
.help {
  color: var(--color-text-2);
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
}
.types {
  display: flex;
  flex-wrap: wrap;
  gap: 0 12px;
  margin: 0;
  padding: 0;
  border: 0;
}
input,
select {
  min-height: 44px;
  min-width: 0;
}
</style>
