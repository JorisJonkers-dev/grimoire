<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getSpeciesBuildOptions, previewSpeciesMutation, saveSpeciesBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { SpeciesSpell } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const species = useQuery(computed(() => ({ ...getSpeciesBuildOptions(path.value), retry: false })))
const preview = useMutation(previewSpeciesMutation())
const save = useMutation(saveSpeciesBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: species.data, key: computed(() => getSpeciesBuildOptions(path.value).queryKey), preview, save, fallback: 'Species',
})

const sizes = ['tiny', 'small', 'medium', 'large', 'huge']
const types = ['aberration', 'beast', 'celestial', 'construct', 'dragon', 'elemental', 'fey', 'fiend', 'giant', 'humanoid', 'monstrosity', 'ooze', 'plant', 'undead']
const damage = ['acid', 'bludgeoning', 'cold', 'fire', 'force', 'lightning', 'necrotic', 'piercing', 'poison', 'psychic', 'radiant', 'slashing', 'thunder']
const speedKinds = ['climb', 'fly', 'swim', 'burrow']
const senseKinds = ['darkvision', 'blindsight', 'tremorsense', 'truesight']
const freshSpell = (): SpeciesSpell => ({ level: 1, spell: '', name: '', uses: 'long_rest' })

function toggle(list: string[], value: string) {
  const i = list.indexOf(value)
  if (i >= 0) list.splice(i, 1)
  else list.push(value)
}
</script>

<template>
  <main class="g-page builder">
    <RouterLink :to="{ name: 'library-entry', params: { entryId: String(route.params.entryId) } }" class="back">← Entry</RouterLink>
    <p v-if="species.isError.value" role="alert" class="g-alert" data-testid="species-error">This entry cannot be opened in the species builder.</p>
    <p v-else-if="!design">Opening the species…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Species builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="species-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="species-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="species-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="species-form" @submit.prevent="runSave">
          <fieldset class="checks">
            <legend>Size (one, or two to choose from)</legend>
            <label v-for="s in sizes" :key="s" class="check">
              <input type="checkbox" :checked="design.sizes.includes(s)" :data-testid="`species-size-${s}`" @change="toggle(design.sizes, s)" /><span>{{ s }}</span>
            </label>
          </fieldset>
          <div class="grid">
            <label class="g-field"><span>Creature type</span>
              <select v-model="design.creatureType" data-testid="species-type"><option v-for="t in types" :key="t" :value="t">{{ t }}</option></select>
            </label>
            <label class="g-field"><span>Speed (feet)</span><input v-model.number="design.speedFt" type="number" min="10" max="60" step="5" data-testid="species-speed" /></label>
          </div>

          <h2>Speeds and senses</h2>
          <ol class="rows">
            <li v-for="(m, i) in design.speeds" :key="`speed-${String(i)}`" class="row-item">
              <label class="g-field"><span>Speed</span>
                <select v-model="m.kind" :data-testid="`species-speed-${String(i)}-kind`"><option v-for="k in speedKinds" :key="k" :value="k">{{ k }}</option></select>
              </label>
              <label class="g-field small"><span>Feet</span><input v-model.number="m.feet" type="number" min="5" max="120" :data-testid="`species-speed-${String(i)}-feet`" /></label>
              <button type="button" class="drop" :aria-label="`Remove speed ${String(i + 1)}`" :data-testid="`species-speed-${String(i)}-remove`" @click="design.speeds.splice(i, 1)">×</button>
            </li>
            <li v-for="(m, i) in design.senses" :key="`sense-${String(i)}`" class="row-item">
              <label class="g-field"><span>Sense</span>
                <select v-model="m.kind" :data-testid="`species-sense-${String(i)}-kind`"><option v-for="k in senseKinds" :key="k" :value="k">{{ k }}</option></select>
              </label>
              <label class="g-field small"><span>Feet</span><input v-model.number="m.feet" type="number" min="5" max="300" :data-testid="`species-sense-${String(i)}-feet`" /></label>
              <button type="button" class="drop" :aria-label="`Remove sense ${String(i + 1)}`" :data-testid="`species-sense-${String(i)}-remove`" @click="design.senses.splice(i, 1)">×</button>
            </li>
          </ol>
          <div class="row">
            <GButton data-testid="species-add-speed" @click="design.speeds.push({ kind: 'swim', feet: 30 })">+ Speed</GButton>
            <GButton data-testid="species-add-sense" @click="design.senses.push({ kind: 'darkvision', feet: 60 })">+ Sense</GButton>
          </div>
          <fieldset class="checks">
            <legend>Resistances</legend>
            <label v-for="d in damage" :key="d" class="check">
              <input type="checkbox" :checked="design.resistances.includes(d)" :data-testid="`species-resist-${d}`" @change="toggle(design.resistances, d)" /><span>{{ d }}</span>
            </label>
          </fieldset>

          <h2>Traits</h2>
          <ol class="rows">
            <li v-for="(t, i) in design.traits" :key="i" class="row-item">
              <label class="g-field"><span>Name</span><input v-model="t.name" maxlength="60" :data-testid="`species-trait-${String(i)}-name`" /></label>
              <label class="g-field wide"><span>What it does</span><textarea v-model="t.text" rows="2" maxlength="2000" :data-testid="`species-trait-${String(i)}-text`" /></label>
              <button type="button" class="drop" :aria-label="`Remove trait ${String(i + 1)}`" :data-testid="`species-trait-${String(i)}-remove`" @click="design.traits.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="species-add-trait" @click="design.traits.push({ name: '', text: '' })">+ Trait</GButton>

          <h2>Innate spells</h2>
          <ol class="rows">
            <li v-for="(s, i) in design.spells" :key="i" class="row-item">
              <label class="g-field small"><span>From level</span><input v-model.number="s.level" type="number" min="1" max="20" :data-testid="`species-spell-${String(i)}-level`" /></label>
              <label class="g-field"><span>Spell</span><input v-model="s.name" maxlength="60" :data-testid="`species-spell-${String(i)}-name`" /></label>
              <label class="g-field"><span>Slug</span><input v-model="s.spell" maxlength="80" :data-testid="`species-spell-${String(i)}-slug`" /></label>
              <label class="g-field"><span>Cast</span>
                <select v-model="s.uses" :data-testid="`species-spell-${String(i)}-uses`"><option value="at_will">at will</option><option value="long_rest">once per long rest</option></select>
              </label>
              <button type="button" class="drop" :aria-label="`Remove spell ${String(i + 1)}`" :data-testid="`species-spell-${String(i)}-remove`" @click="design.spells.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="species-add-spell" @click="design.spells.push(freshSpell())">+ Spell</GButton>

          <h2>Lineages</h2>
          <ol class="rows">
            <li v-for="(l, i) in design.lineages" :key="i" class="row-item">
              <label class="g-field"><span>Lineage</span><input v-model="l.name" maxlength="40" :data-testid="`species-lineage-${String(i)}-name`" /></label>
              <label class="g-field wide"><span>What it gives</span><textarea v-model="l.text" rows="2" maxlength="2000" :data-testid="`species-lineage-${String(i)}-text`" /></label>
              <span v-for="(s, j) in l.spells" :key="j" class="nested">
                <label class="g-field small"><span>From level</span><input v-model.number="s.level" type="number" min="1" max="20" :data-testid="`species-lineage-${String(i)}-spell-${String(j)}-level`" /></label>
                <label class="g-field"><span>Spell</span><input v-model="s.name" maxlength="60" :data-testid="`species-lineage-${String(i)}-spell-${String(j)}-name`" /></label>
                <label class="g-field"><span>Slug</span><input v-model="s.spell" maxlength="80" :data-testid="`species-lineage-${String(i)}-spell-${String(j)}-slug`" /></label>
                <label class="g-field"><span>Cast</span>
                  <select v-model="s.uses" :data-testid="`species-lineage-${String(i)}-spell-${String(j)}-uses`"><option value="at_will">at will</option><option value="long_rest">once per long rest</option></select>
                </label>
              </span>
              <GButton :data-testid="`species-lineage-${String(i)}-add-spell`" @click="l.spells.push(freshSpell())">+ Lineage spell</GButton>
              <button type="button" class="drop" :aria-label="`Remove lineage ${String(i + 1)}`" :data-testid="`species-lineage-${String(i)}-remove`" @click="design.lineages.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="species-add-lineage" @click="design.lineages.push({ name: '', text: '', spells: [] })">+ Lineage</GButton>

          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="species-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="species-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="How it reads" data-testid="species-lines">
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
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 8px 12px;
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
.row-item,
.nested {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px 12px;
}
.row-item {
  padding: 10px 0;
  border-top: 1px solid var(--color-line);
}
.nested {
  flex-basis: 100%;
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
