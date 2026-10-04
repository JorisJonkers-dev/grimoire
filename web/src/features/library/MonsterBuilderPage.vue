<script setup lang="ts">
import BuilderCrumbs from './BuilderCrumbs.vue'
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getMonsterBuildOptions, previewMonsterMutation, saveMonsterBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton } from '@/shared/ui'
import { abilities } from './classPresets'
import { useBuilder } from './useBuilder'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const monster = useQuery(computed(() => ({ ...getMonsterBuildOptions(path.value), retry: false })))
const preview = useMutation(previewMonsterMutation())
const save = useMutation(saveMonsterBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: monster.data, key: computed(() => getMonsterBuildOptions(path.value).queryKey), preview, save, fallback: 'Creature',
})

const sizes = ['tiny', 'small', 'medium', 'large', 'huge', 'gargantuan']
const types = ['aberration', 'beast', 'celestial', 'construct', 'dragon', 'elemental', 'fey', 'fiend', 'giant', 'humanoid', 'monstrosity', 'ooze', 'plant', 'undead']
const damage = ['acid', 'bludgeoning', 'cold', 'fire', 'force', 'lightning', 'necrotic', 'piercing', 'poison', 'psychic', 'radiant', 'slashing', 'thunder']
const challenges = [{ v: 0, l: '0' }, { v: 0.125, l: '1/8' }, { v: 0.25, l: '1/4' }, { v: 0.5, l: '1/2' }, ...Array.from({ length: 30 }, (_, i) => ({ v: i + 1, l: String(i + 1) }))]
const defences = [
  { key: 'resistances', label: 'Resistances' },
  { key: 'immunities', label: 'Immunities' },
  { key: 'vulnerabilities', label: 'Vulnerabilities' },
] as const

function toggle(list: string[], value: string) {
  const i = list.indexOf(value)
  if (i >= 0) list.splice(i, 1)
  else list.push(value)
}
function setAura(on: boolean) {
  if (design.value) design.value.aura = on ? { name: 'Aura', feet: 10, text: '' } : undefined
}
function setLegendary(on: boolean) {
  if (design.value) design.value.legendary = on ? { uses: 3, resistance: 0, actions: [{ name: 'Attack', cost: 1, text: 'It makes one attack.' }] } : undefined
}
function setLair(on: boolean) {
  if (design.value) design.value.lair = on ? { actions: [{ name: 'Lair action', text: '' }], regional: [] } : undefined
}
</script>

<template>
  <main class="g-page builder">
    <BuilderCrumbs :entry-id="entryId" here="Monster builder" />
    <p v-if="monster.isError.value" role="alert" class="g-alert" data-testid="monster-error">This entry cannot be opened in the monster builder.</p>
    <p v-else-if="!design">Opening the creature…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Monster builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="monster-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="monster-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="monster-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="monster-form" @submit.prevent="runSave">
          <div class="grid">
            <label class="g-field"><span>Size</span><select v-model="design.size" data-testid="monster-size"><option v-for="s in sizes" :key="s" :value="s">{{ s }}</option></select></label>
            <label class="g-field"><span>Type</span><select v-model="design.creatureType" data-testid="monster-type"><option v-for="t in types" :key="t" :value="t">{{ t }}</option></select></label>
            <label class="g-field"><span>AC</span><input v-model.number="design.ac" type="number" min="5" max="30" data-testid="monster-ac" /></label>
            <label class="g-field"><span>Hit points</span><input v-model.number="design.hp" type="number" min="1" max="1000" data-testid="monster-hp" /></label>
            <label class="g-field"><span>Speed (ft)</span><input v-model.number="design.speedFt" type="number" min="0" max="120" step="5" data-testid="monster-speed" /></label>
            <label class="g-field"><span>Challenge</span>
              <select v-model.number="design.challenge" data-testid="monster-challenge"><option v-for="c in challenges" :key="c.v" :value="c.v">{{ c.l }}</option></select>
            </label>
            <label class="g-field"><span>Damage threshold</span><input v-model.number="design.threshold" type="number" min="0" max="50" data-testid="monster-threshold" /></label>
            <label class="g-field"><span>Multiattack (0 none)</span><input v-model.number="design.multiattack" type="number" min="0" max="4" data-testid="monster-multiattack" /></label>
            <label class="check"><input v-model="design.swarm" type="checkbox" data-testid="monster-swarm" /><span>Swarm</span></label>
          </div>
          <fieldset class="grid">
            <legend>Abilities</legend>
            <label v-for="a in abilities" :key="a" class="g-field small"><span>{{ a.slice(0, 3).toUpperCase() }}</span>
              <input v-model.number="design.abilities[a]" type="number" min="1" max="30" :data-testid="`monster-${a}`" />
            </label>
          </fieldset>
          <fieldset class="checks">
            <legend>Proficient saves</legend>
            <label v-for="a in abilities" :key="a" class="check">
              <input type="checkbox" :checked="design.saves.includes(a)" :data-testid="`monster-save-${a}`" @change="toggle(design.saves, a)" /><span>{{ a }}</span>
            </label>
          </fieldset>
          <fieldset v-for="d in defences" :key="d.key" class="checks">
            <legend>{{ d.label }}</legend>
            <label v-for="t in damage" :key="t" class="check">
              <input type="checkbox" :checked="design[d.key].includes(t)" :data-testid="`monster-${d.key}-${t}`" @change="toggle(design[d.key], t)" /><span>{{ t }}</span>
            </label>
          </fieldset>
          <ol class="rows">
            <li v-for="(s, i) in design.senses" :key="i" class="row-item">
              <label class="g-field"><span>Sense</span>
                <select v-model="s.kind" :data-testid="`monster-sense-${String(i)}-kind`"><option v-for="k in ['darkvision', 'blindsight', 'tremorsense', 'truesight']" :key="k" :value="k">{{ k }}</option></select>
              </label>
              <label class="g-field small"><span>Feet</span><input v-model.number="s.feet" type="number" min="5" max="300" :data-testid="`monster-sense-${String(i)}-feet`" /></label>
              <button type="button" class="drop" :aria-label="`Remove sense ${String(i + 1)}`" :data-testid="`monster-sense-${String(i)}-remove`" @click="design.senses.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="monster-add-sense" @click="design.senses.push({ kind: 'darkvision', feet: 60 })">+ Sense</GButton>

          <h2>Traits</h2>
          <ol class="rows">
            <li v-for="(t, i) in design.traits" :key="i" class="row-item">
              <label class="g-field"><span>Name</span><input v-model="t.name" maxlength="60" :data-testid="`monster-trait-${String(i)}-name`" /></label>
              <label class="g-field wide"><span>What it does</span><textarea v-model="t.text" rows="2" maxlength="2000" :data-testid="`monster-trait-${String(i)}-text`" /></label>
              <button type="button" class="drop" :aria-label="`Remove trait ${String(i + 1)}`" :data-testid="`monster-trait-${String(i)}-remove`" @click="design.traits.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="monster-add-trait" @click="design.traits.push({ name: '', text: '' })">+ Trait</GButton>
          <label class="check"><input :checked="Boolean(design.aura)" type="checkbox" data-testid="monster-aura" @change="setAura(($event.target as HTMLInputElement).checked)" /><span>Has an aura</span></label>
          <div v-if="design.aura" class="grid">
            <label class="g-field"><span>Aura</span><input v-model="design.aura.name" maxlength="60" data-testid="monster-aura-name" /></label>
            <label class="g-field small"><span>Feet</span><input v-model.number="design.aura.feet" type="number" min="5" max="60" data-testid="monster-aura-feet" /></label>
            <label class="g-field wide"><span>What it does</span><input v-model="design.aura.text" maxlength="2000" data-testid="monster-aura-text" /></label>
          </div>

          <h2>Actions</h2>
          <ol class="rows">
            <li v-for="(a, i) in design.actions" :key="i" class="row-item">
              <label class="g-field"><span>Name</span><input v-model="a.name" maxlength="60" :data-testid="`monster-action-${String(i)}-name`" /></label>
              <label class="g-field"><span>Kind</span>
                <select v-model="a.kind" :data-testid="`monster-action-${String(i)}-kind`"><option v-for="k in ['melee', 'ranged', 'save', 'special']" :key="k" :value="k">{{ k }}</option></select>
              </label>
              <template v-if="a.kind === 'melee' || a.kind === 'ranged'">
                <label class="g-field small"><span>To hit</span><input v-model.number="a.toHit" type="number" min="-5" max="20" :data-testid="`monster-action-${String(i)}-hit`" /></label>
                <label v-if="a.kind === 'melee'" class="g-field small"><span>Reach</span><input v-model.number="a.reachFt" type="number" min="5" max="30" :data-testid="`monster-action-${String(i)}-reach`" /></label>
                <label v-if="a.kind === 'ranged'" class="g-field small"><span>Range</span><input v-model.number="a.rangeFt" type="number" min="5" max="600" :data-testid="`monster-action-${String(i)}-range`" /></label>
                <label v-if="a.kind === 'ranged'" class="g-field small"><span>Long</span><input v-model.number="a.longRangeFt" type="number" min="0" max="1200" :data-testid="`monster-action-${String(i)}-long`" /></label>
              </template>
              <template v-if="a.kind === 'save'">
                <label class="g-field"><span>Save</span>
                  <select v-model="a.saveAbility" :data-testid="`monster-action-${String(i)}-save`"><option v-for="ab in abilities" :key="ab" :value="ab">{{ ab }}</option></select>
                </label>
                <label class="g-field small"><span>DC</span><input v-model.number="a.dc" type="number" min="8" max="30" :data-testid="`monster-action-${String(i)}-dc`" /></label>
              </template>
              <template v-if="a.kind !== 'special'">
                <label class="g-field small"><span>Damage</span><input v-model="a.damage" maxlength="10" placeholder="2d6" :data-testid="`monster-action-${String(i)}-damage`" /></label>
                <label class="g-field small"><span>Bonus</span><input v-model.number="a.damageBonus" type="number" min="-5" max="30" :data-testid="`monster-action-${String(i)}-bonus`" /></label>
                <label class="g-field"><span>Type</span>
                  <select v-model="a.damageType" :data-testid="`monster-action-${String(i)}-type`"><option v-for="t in damage" :key="t" :value="t">{{ t }}</option></select>
                </label>
              </template>
              <label class="g-field small"><span>Recharge</span>
                <select v-model.number="a.recharge" :data-testid="`monster-action-${String(i)}-recharge`"><option :value="0">never</option><option v-for="n in [4, 5, 6]" :key="n" :value="n">{{ n }}+</option></select>
              </label>
              <label class="g-field wide"><span>More</span><input v-model="a.text" maxlength="2000" :data-testid="`monster-action-${String(i)}-text`" /></label>
              <button type="button" class="drop" :aria-label="`Remove action ${String(i + 1)}`" :data-testid="`monster-action-${String(i)}-remove`" @click="design.actions.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="monster-add-action" @click="design.actions.push({ name: '', kind: 'special', text: '' })">+ Action</GButton>

          <h2>Legendary, lair and phases</h2>
          <label class="check"><input :checked="Boolean(design.legendary)" type="checkbox" data-testid="monster-legendary" @change="setLegendary(($event.target as HTMLInputElement).checked)" /><span>Legendary</span></label>
          <template v-if="design.legendary">
            <div class="grid">
              <label class="g-field small"><span>Actions a round</span><input v-model.number="design.legendary.uses" type="number" min="1" max="5" data-testid="monster-legendary-uses" /></label>
              <label class="g-field small"><span>Resistance a day</span><input v-model.number="design.legendary.resistance" type="number" min="0" max="5" data-testid="monster-legendary-resistance" /></label>
            </div>
            <ol class="rows">
              <li v-for="(a, i) in design.legendary.actions" :key="i" class="row-item">
                <label class="g-field"><span>Name</span><input v-model="a.name" maxlength="60" :data-testid="`monster-legendary-${String(i)}-name`" /></label>
                <label class="g-field small"><span>Cost</span><input v-model.number="a.cost" type="number" min="1" max="3" :data-testid="`monster-legendary-${String(i)}-cost`" /></label>
                <label class="g-field wide"><span>What it does</span><input v-model="a.text" maxlength="2000" :data-testid="`monster-legendary-${String(i)}-text`" /></label>
                <button type="button" class="drop" :aria-label="`Remove legendary action ${String(i + 1)}`" :data-testid="`monster-legendary-${String(i)}-remove`" @click="design.legendary.actions.splice(i, 1)">×</button>
              </li>
            </ol>
            <GButton data-testid="monster-add-legendary" @click="design.legendary.actions.push({ name: '', cost: 1, text: '' })">+ Legendary action</GButton>
          </template>
          <label class="check"><input :checked="Boolean(design.lair)" type="checkbox" data-testid="monster-lair" @change="setLair(($event.target as HTMLInputElement).checked)" /><span>Has a lair</span></label>
          <template v-if="design.lair">
            <ol class="rows">
              <li v-for="(a, i) in design.lair.actions" :key="i" class="row-item">
                <label class="g-field"><span>Lair action</span><input v-model="a.name" maxlength="60" :data-testid="`monster-lair-${String(i)}-name`" /></label>
                <label class="g-field wide"><span>What happens</span><input v-model="a.text" maxlength="2000" :data-testid="`monster-lair-${String(i)}-text`" /></label>
                <button type="button" class="drop" :aria-label="`Remove lair action ${String(i + 1)}`" :data-testid="`monster-lair-${String(i)}-remove`" @click="design.lair.actions.splice(i, 1)">×</button>
              </li>
              <li v-for="(_, i) in design.lair.regional" :key="`r${String(i)}`" class="row-item">
                <label class="g-field wide"><span>Regional effect</span><input v-model="design.lair.regional[i]" maxlength="2000" :data-testid="`monster-regional-${String(i)}`" /></label>
                <button type="button" class="drop" :aria-label="`Remove regional effect ${String(i + 1)}`" :data-testid="`monster-regional-${String(i)}-remove`" @click="design.lair.regional.splice(i, 1)">×</button>
              </li>
            </ol>
            <div class="row">
              <GButton data-testid="monster-add-lair" @click="design.lair.actions.push({ name: '', text: '' })">+ Lair action</GButton>
              <GButton data-testid="monster-add-regional" @click="design.lair.regional.push('')">+ Regional effect</GButton>
            </div>
          </template>
          <ol class="rows">
            <li v-for="(p, i) in design.phases" :key="i" class="row-item">
              <label class="g-field"><span>Phase {{ i + 2 }}</span><input v-model="p.name" maxlength="60" :data-testid="`monster-phase-${String(i)}-name`" /></label>
              <label class="g-field small"><span>HP</span><input v-model.number="p.hp" type="number" min="1" max="1000" :data-testid="`monster-phase-${String(i)}-hp`" /></label>
              <label class="g-field wide"><span>What changes</span><input v-model="p.text" maxlength="2000" :data-testid="`monster-phase-${String(i)}-text`" /></label>
              <button type="button" class="drop" :aria-label="`Remove phase ${String(i + 2)}`" :data-testid="`monster-phase-${String(i)}-remove`" @click="design.phases.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="monster-add-phase" @click="design.phases.push({ name: '', hp: design.hp, text: '' })">+ Mythic phase</GButton>

          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="monster-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="monster-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="Stat block" data-testid="monster-lines">
          <h2>{{ name }}</h2>
          <p v-if="shown" class="g-tag" data-testid="monster-estimate">Estimated Challenge {{ shown.estimate }}</p>
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
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 8px 12px;
  align-items: end;
  margin: 0;
  padding: 0;
  border: 0;
}
.checks {
  display: flex;
  flex-wrap: wrap;
  gap: 0 12px;
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
