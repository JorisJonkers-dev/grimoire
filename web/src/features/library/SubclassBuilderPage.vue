<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { getSubclassBuildOptions, previewSubclassMutation, saveSubclassBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { SubclassBuild, SubclassDesign, SubclassResource } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const client = useQueryClient()
const path = computed(() => ({ path: { entryId: String(route.params.entryId) } }))
const subclass = useQuery(computed(() => ({ ...getSubclassBuildOptions(path.value), retry: false })))
const preview = useMutation(previewSubclassMutation())
const save = useMutation(saveSubclassBuildMutation())
const problem = computed(() => preview.error.value ?? save.error.value)
// copy takes plain data out of a reactive value.
const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) as T
const design = ref<SubclassDesign>()
const shown = ref<SubclassBuild>()
const status = ref('')
watch(
  () => subclass.data.value,
  (b) => {
    if (!b || design.value) return
    design.value = copy(b.design)
    shown.value = b
  },
  { immediate: true },
)
const name = computed(() => subclass.data.value?.entry?.name ?? '')
const readOnly = computed(() => subclass.data.value?.entry?.shared ?? false)

const classes = ['barbarian', 'bard', 'cleric', 'druid', 'fighter', 'monk', 'paladin', 'ranger', 'rogue', 'sorcerer', 'warlock', 'wizard']
const abilities = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma']
const bases = [
  { value: 'fixed', label: 'A fixed number' },
  { value: 'class_level', label: 'Per class level' },
  { value: 'ability', label: 'An ability modifier' },
  { value: 'proficiency', label: 'Proficiency Bonus' },
]
const recharges = [
  { value: 'short_rest', label: 'a Short or Long Rest' },
  { value: 'long_rest', label: 'a Long Rest' },
  { value: 'dawn', label: 'dawn' },
]
const dice = ['', 'd4', 'd6', 'd8', 'd10', 'd12']
// keyOf names a Resource by its name, as features point at it: Lantern Light is lantern-light.
const keyOf = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 30)

function addFeature() {
  design.value?.features.push({ level: 3, name: '', text: '' })
}
function addResource() {
  design.value?.resources.push({ key: '', name: '', basis: 'proficiency', fromLevel: 3, recharge: 'long_rest' })
}
function addChoice() {
  design.value?.choices.push({ level: 3, name: '', count: 1, options: [] })
}
function rename(r: SubclassResource, value: string) {
  const before = r.key
  r.name = value
  r.key = keyOf(value)
  for (const f of design.value?.features ?? []) if (before && f.uses === before) f.uses = r.key
}
function setBasis(r: SubclassResource, basis: string) {
  r.basis = basis
  r.amount = basis === 'fixed' || basis === 'class_level' ? (r.amount ?? 1) : undefined
  r.ability = basis === 'ability' ? (r.ability ?? 'wisdom') : undefined
}
function setOptions(i: number, text: string) {
  const c = design.value?.choices[i]
  if (c) c.options = text.split(',').map((o) => o.trim()).filter(Boolean)
}
function runPreview() {
  if (!design.value) return
  status.value = ''
  preview.mutate({ body: { name: name.value || 'Subclass', design: design.value } }, { onSuccess: (b) => (shown.value = b) })
}
function runSave() {
  if (!design.value) return
  status.value = ''
  save.mutate({ ...path.value, body: design.value }, {
    onSuccess: (b) => {
      shown.value = b
      client.setQueryData(getSubclassBuildOptions(path.value).queryKey, b)
      void client.invalidateQueries()
      status.value = `Saved as Revision ${String(b.entry?.revision ?? 0)}.`
    },
  })
}
</script>

<template>
  <main class="g-page builder">
    <RouterLink :to="{ name: 'library-entry', params: { entryId: String(route.params.entryId) } }" class="back">← Entry</RouterLink>
    <p v-if="subclass.isError.value" role="alert" class="g-alert" data-testid="subclass-error">This entry cannot be opened in the subclass builder.</p>
    <p v-else-if="!design">Opening the subclass…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Subclass builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="subclass-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="subclass-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="subclass-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="subclass-form" @submit.prevent="runSave">
          <label class="g-field"><span>Class</span>
            <select v-model="design.class" data-testid="subclass-class"><option v-for="c in classes" :key="c" :value="c">{{ c }}</option></select>
          </label>

          <h2>Features</h2>
          <ol class="rows" data-testid="features">
            <li v-for="(f, i) in design.features" :key="i" class="row-item" :data-testid="`feature-${String(i)}`">
              <label class="g-field small"><span>Level</span><input v-model.number="f.level" type="number" min="1" max="20" :data-testid="`feature-${String(i)}-level`" /></label>
              <label class="g-field"><span>Name</span><input v-model="f.name" maxlength="60" :data-testid="`feature-${String(i)}-name`" /></label>
              <label class="g-field wide"><span>What it does</span><textarea v-model="f.text" rows="2" maxlength="1000" :data-testid="`feature-${String(i)}-text`" /></label>
              <label class="g-field"><span>Spends</span>
                <select v-model="f.uses" :data-testid="`feature-${String(i)}-uses`">
                  <option :value="undefined">nothing</option>
                  <option v-for="r in design.resources" :key="r.key" :value="r.key">{{ r.name || 'unnamed Resource' }}</option>
                </select>
              </label>
              <label class="g-field"><span>Casts (spell slug)</span><input v-model="f.spell" maxlength="80" :data-testid="`feature-${String(i)}-spell`" /></label>
              <label class="g-field"><span>Spell name</span><input v-model="f.spellName" maxlength="60" :data-testid="`feature-${String(i)}-spell-name`" /></label>
              <button type="button" class="drop" :aria-label="`Remove feature ${String(i + 1)}`" :data-testid="`feature-${String(i)}-remove`" @click="design.features.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="add-feature" @click="addFeature">+ Feature</GButton>

          <h2>Resources</h2>
          <ol class="rows" data-testid="resources">
            <li v-for="(r, i) in design.resources" :key="i" class="row-item" :data-testid="`resource-${String(i)}`">
              <label class="g-field"><span>Name</span><input :value="r.name" maxlength="60" :data-testid="`resource-${String(i)}-name`" @input="rename(r, ($event.target as HTMLInputElement).value)" /></label>
              <label class="g-field"><span>Uses</span>
                <select :value="r.basis" :data-testid="`resource-${String(i)}-basis`" @change="setBasis(r, ($event.target as HTMLSelectElement).value)">
                  <option v-for="b in bases" :key="b.value" :value="b.value">{{ b.label }}</option>
                </select>
              </label>
              <label v-if="r.basis === 'fixed' || r.basis === 'class_level'" class="g-field small"><span>{{ r.basis === 'fixed' ? 'How many' : 'Per level' }}</span>
                <input v-model.number="r.amount" type="number" min="1" max="20" :data-testid="`resource-${String(i)}-amount`" />
              </label>
              <label v-if="r.basis === 'ability'" class="g-field"><span>Ability</span>
                <select v-model="r.ability" :data-testid="`resource-${String(i)}-ability`"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
              </label>
              <label class="g-field small"><span>From level</span><input v-model.number="r.fromLevel" type="number" min="1" max="20" :data-testid="`resource-${String(i)}-from`" /></label>
              <label class="g-field small"><span>Die</span>
                <select v-model="r.die" :data-testid="`resource-${String(i)}-die`"><option v-for="d in dice" :key="d" :value="d || undefined">{{ d || 'none' }}</option></select>
              </label>
              <label class="g-field"><span>Back on</span>
                <select v-model="r.recharge" :data-testid="`resource-${String(i)}-recharge`"><option v-for="x in recharges" :key="x.value" :value="x.value">{{ x.label }}</option></select>
              </label>
              <button type="button" class="drop" :aria-label="`Remove Resource ${String(i + 1)}`" :data-testid="`resource-${String(i)}-remove`" @click="design.resources.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="add-resource" @click="addResource">+ Resource</GButton>

          <h2>Choices</h2>
          <ol class="rows" data-testid="choices">
            <li v-for="(c, i) in design.choices" :key="i" class="row-item" :data-testid="`choice-${String(i)}`">
              <label class="g-field small"><span>Level</span><input v-model.number="c.level" type="number" min="1" max="20" :data-testid="`choice-${String(i)}-level`" /></label>
              <label class="g-field"><span>Name</span><input v-model="c.name" maxlength="40" :data-testid="`choice-${String(i)}-name`" /></label>
              <label class="g-field small"><span>Pick</span><input v-model.number="c.count" type="number" min="1" max="20" :data-testid="`choice-${String(i)}-count`" /></label>
              <label class="g-field wide"><span>Options, separated by commas</span>
                <input :value="c.options.join(', ')" :data-testid="`choice-${String(i)}-options`" @change="setOptions(i, ($event.target as HTMLInputElement).value)" />
              </label>
              <button type="button" class="drop" :aria-label="`Remove choice ${String(i + 1)}`" :data-testid="`choice-${String(i)}-remove`" @click="design.choices.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="add-choice" @click="addChoice">+ Choice</GButton>

          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="subclass-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="subclass-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="How it reads" data-testid="subclass-lines">
          <h2>{{ name }}</h2>
          <p v-for="(line, i) in shown?.lines ?? []" :key="i" class="line">{{ line }}</p>
        </aside>
      </div>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.builder {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
h1,
h2 {
  margin: 0;
  font-family: var(--font-display);
}
h2 {
  font-size: 17px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.line {
  margin: 0;
}
.layout {
  display: grid;
  gap: 14px;
}
@media (min-width: 960px) {
  .layout {
    grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr);
    align-items: start;
  }
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.rows {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}
.row-item {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px 12px;
  padding: 10px 0;
  border-top: 1px solid var(--color-line);
}
.small {
  max-width: 110px;
}
.wide {
  flex-basis: 100%;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.drop {
  min-width: 44px;
  min-height: 44px;
  margin-left: auto;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text);
  cursor: pointer;
}
input,
select,
textarea {
  min-height: 44px;
  min-width: 0;
}
</style>
