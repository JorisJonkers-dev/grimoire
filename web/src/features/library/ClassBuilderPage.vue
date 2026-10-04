<script setup lang="ts">
import BuilderCrumbs from './BuilderCrumbs.vue'
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getClassBuildOptions, previewClassMutation, saveClassBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'
import { abilities, casterKinds, castingFor, srdClasses } from './classPresets'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const klass = useQuery(computed(() => ({ ...getClassBuildOptions(path.value), retry: false })))
const preview = useMutation(previewClassMutation())
const save = useMutation(saveClassBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: klass.data, key: computed(() => getClassBuildOptions(path.value).queryKey), preview, save, fallback: 'Class',
})
const levels = Array.from({ length: 20 }, (_, i) => i + 1)
const casts = computed(() => design.value !== undefined && design.value.casting.kind !== 'none')

function toggle(list: string[], value: string) {
  const i = list.indexOf(value)
  if (i >= 0) list.splice(i, 1)
  else list.push(value)
}
function setFeatLevels(text: string) {
  if (design.value) design.value.featLevels = text.split(',').map((x) => Number(x.trim())).filter((n) => n > 0)
}
function setKind(kind: string) {
  if (design.value) design.value.casting = castingFor(kind, design.value.casting)
}
function setSlot(level: number, spell: number, value: string) {
  const row = design.value?.casting.slots?.[level - 1]
  if (row) row[spell - 1] = Number(value)
}
function addColumn() {
  design.value?.columns.push({ name: '', values: Array<string>(20).fill('') })
}
function addFeature() {
  design.value?.features.push({ level: 1, name: '', text: '' })
}
</script>

<template>
  <main class="g-page builder">
    <BuilderCrumbs :entry-id="String(route.params.entryId)" here="Class builder" />
    <p v-if="klass.isError.value" role="alert" class="g-alert" data-testid="class-error">This entry cannot be opened in the class builder.</p>
    <p v-else-if="!design">Opening the class…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Class builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="class-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="class-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="class-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="class-form" @submit.prevent="runSave">
          <fieldset class="grid">
            <legend>The class</legend>
            <label class="g-field"><span>Hit Die</span>
              <select v-model.number="design.hitDie" data-testid="class-hit-die"><option v-for="d in [6, 8, 10, 12]" :key="d" :value="d">d{{ d }}</option></select>
            </label>
            <label class="g-field"><span>First saving throw</span>
              <select v-model="design.saves[0]" data-testid="class-save-0"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
            </label>
            <label class="g-field"><span>Second saving throw</span>
              <select v-model="design.saves[1]" data-testid="class-save-1"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
            </label>
            <label class="g-field"><span>Skills to pick</span><input v-model.number="design.skills" type="number" min="1" max="6" data-testid="class-skills" /></label>
            <label class="g-field"><span>Subclass at level</span><input v-model.number="design.subclassLevel" type="number" min="1" max="20" data-testid="class-subclass-level" /></label>
            <label class="g-field"><span>Feats at levels</span><input :value="design.featLevels.join(', ')" data-testid="class-feat-levels" @change="setFeatLevels(($event.target as HTMLInputElement).value)" /></label>
          </fieldset>
          <fieldset class="checks">
            <legend>Primary abilities (one or two)</legend>
            <label v-for="a in abilities" :key="a" class="check">
              <input type="checkbox" :checked="design.primary.includes(a)" :data-testid="`class-primary-${a}`" @change="toggle(design.primary, a)" /><span>{{ a }}</span>
            </label>
            <label class="check"><input v-model="design.anyPrimary" type="checkbox" data-testid="class-any-primary" /><span>Any one of them is enough to multiclass</span></label>
          </fieldset>
          <fieldset class="checks">
            <legend>Training</legend>
            <label v-for="a in ['light', 'medium', 'heavy', 'shields']" :key="a" class="check">
              <input type="checkbox" :checked="design.armor.includes(a)" :data-testid="`class-armor-${a}`" @change="toggle(design.armor, a)" /><span>{{ a === 'shields' ? 'Shields' : `${a} armor` }}</span>
            </label>
            <label v-for="w in ['simple', 'martial']" :key="w" class="check">
              <input type="checkbox" :checked="design.weapons.includes(w)" :data-testid="`class-weapons-${w}`" @change="toggle(design.weapons, w)" /><span>{{ w }} weapons</span>
            </label>
          </fieldset>

          <h2>Features</h2>
          <ol class="rows" data-testid="class-features">
            <li v-for="(f, i) in design.features" :key="i" class="row-item">
              <label class="g-field small"><span>Level</span><input v-model.number="f.level" type="number" min="1" max="20" :data-testid="`class-feature-${String(i)}-level`" /></label>
              <label class="g-field"><span>Name</span><input v-model="f.name" maxlength="60" :data-testid="`class-feature-${String(i)}-name`" /></label>
              <label class="g-field wide"><span>What it does</span><textarea v-model="f.text" rows="2" maxlength="2000" :data-testid="`class-feature-${String(i)}-text`" /></label>
              <button type="button" class="drop" :aria-label="`Remove feature ${String(i + 1)}`" :data-testid="`class-feature-${String(i)}-remove`" @click="design.features.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="class-add-feature" @click="addFeature">+ Feature</GButton>

          <h2>Spellcasting</h2>
          <div class="grid">
            <label class="g-field"><span>Casts</span>
              <select :value="design.casting.kind" data-testid="class-casting-kind" @change="setKind(($event.target as HTMLSelectElement).value)">
                <option v-for="k in casterKinds" :key="k.value" :value="k.value">{{ k.label }}</option>
              </select>
            </label>
            <template v-if="casts">
              <label class="g-field"><span>Ability</span>
                <select v-model="design.casting.ability" data-testid="class-casting-ability"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
              </label>
              <label class="g-field"><span>Spell list</span>
                <select v-model="design.casting.spellList" data-testid="class-spell-list"><option v-for="c in srdClasses" :key="c" :value="c">{{ c }}</option></select>
              </label>
              <label class="check"><input v-model="design.casting.spellbook" type="checkbox" data-testid="class-spellbook" /><span>Keeps a spellbook</span></label>
              <label class="check"><input v-model="design.casting.afterRest" type="checkbox" data-testid="class-after-rest" /><span>Prepares after a long rest</span></label>
            </template>
          </div>
          <div v-if="design.casting.kind === 'points' && design.casting.costs" class="costs" data-testid="class-costs">
            <span>Cost by spell level</span>
            <label v-for="(_, i) in design.casting.costs" :key="i" class="g-field tiny"><span>{{ i + 1 }}</span>
              <input v-model.number="design.casting.costs[i]" type="number" min="1" max="50" :data-testid="`class-cost-${String(i + 1)}`" />
            </label>
          </div>

          <h2>Level table</h2>
          <div class="columns">
            <span v-for="(c, i) in design.columns" :key="i" class="column-name">
              <label class="g-field"><span>Column {{ i + 1 }}</span><input v-model="c.name" maxlength="30" :data-testid="`class-column-${String(i)}-name`" /></label>
              <button type="button" class="drop" :aria-label="`Remove column ${String(i + 1)}`" :data-testid="`class-column-${String(i)}-remove`" @click="design.columns.splice(i, 1)">×</button>
            </span>
            <GButton data-testid="class-add-column" @click="addColumn">+ Column</GButton>
          </div>
          <div class="table-wrap">
            <table class="level-table" data-testid="class-table">
              <thead>
                <tr>
                  <th scope="col">Level</th>
                  <template v-if="casts">
                    <th scope="col">Cantrips</th>
                    <th scope="col">Prepared</th>
                    <template v-if="design.casting.kind === 'slots'"><th v-for="s in 9" :key="s" scope="col">{{ s }}</th></template>
                    <template v-if="design.casting.kind === 'points'"><th scope="col">Points</th><th scope="col">Up to</th></template>
                  </template>
                  <th v-for="(c, i) in design.columns" :key="i" scope="col">{{ c.name || `Column ${String(i + 1)}` }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="l in levels" :key="l">
                  <th scope="row">{{ l }}</th>
                  <template v-if="casts && design.casting.cantrips && design.casting.prepared">
                    <td><input v-model.number="design.casting.cantrips[l - 1]" type="number" min="0" :aria-label="`Cantrips at level ${String(l)}`" :data-testid="`class-cantrips-${String(l)}`" /></td>
                    <td><input v-model.number="design.casting.prepared[l - 1]" type="number" min="0" :aria-label="`Prepared at level ${String(l)}`" :data-testid="`class-prepared-${String(l)}`" /></td>
                    <template v-if="design.casting.kind === 'slots' && design.casting.slots">
                      <td v-for="s in 9" :key="s">
                        <input :value="design.casting.slots[l - 1]?.[s - 1] ?? 0" type="number" min="0" :aria-label="`Level ${String(s)} slots at level ${String(l)}`" :data-testid="`class-slots-${String(l)}-${String(s)}`" @input="setSlot(l, s, ($event.target as HTMLInputElement).value)" />
                      </td>
                    </template>
                    <template v-if="design.casting.kind === 'points' && design.casting.points && design.casting.maxSpell">
                      <td><input v-model.number="design.casting.points[l - 1]" type="number" min="0" :aria-label="`Points at level ${String(l)}`" :data-testid="`class-points-${String(l)}`" /></td>
                      <td><input v-model.number="design.casting.maxSpell[l - 1]" type="number" min="0" max="9" :aria-label="`Highest spell at level ${String(l)}`" :data-testid="`class-max-spell-${String(l)}`" /></td>
                    </template>
                  </template>
                  <td v-for="(c, i) in design.columns" :key="i">
                    <input v-model="c.values[l - 1]" maxlength="20" :aria-label="`${c.name || 'Column'} at level ${String(l)}`" :data-testid="`class-column-${String(i)}-${String(l)}`" />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="class-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="class-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="How it reads" data-testid="class-lines">
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
@media (min-width: 1100px) {
  .layout {
    grid-template-columns: minmax(0, 2fr) minmax(280px, 1fr);
    align-items: start;
  }
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 8px 12px;
  margin: 0;
  padding: 0;
  border: 0;
}
.checks {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
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
  border-top: 1px solid var(--color-rule);
}
.small {
  max-width: 110px;
}
.wide {
  flex-basis: 100%;
}
.costs,
.columns,
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.column-name {
  display: flex;
  align-items: end;
  gap: 6px;
}
.tiny {
  width: 64px;
}
.table-wrap {
  overflow-x: auto;
}
.level-table {
  border-collapse: collapse;
  font-size: 14px;
}
.level-table th,
.level-table td {
  padding: 2px 4px;
  border-bottom: 1px solid var(--color-rule);
  text-align: center;
}
.level-table input {
  width: 56px;
  min-height: 36px;
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
