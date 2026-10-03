<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { getSpellBuildOptions, previewSpellMutation, saveSpellBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { SpellPart } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'
import AreaPreview from './AreaPreview.vue'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const spell = useQuery(computed(() => ({ ...getSpellBuildOptions(path.value), retry: false })))
const preview = useMutation(previewSpellMutation())
const save = useMutation(saveSpellBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: spell.data, key: computed(() => getSpellBuildOptions(path.value).queryKey), preview, save, fallback: 'Spell',
})
// copy takes plain data out of a reactive value.
const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) as T

const shapes = ['sphere', 'cylinder', 'emanation', 'ring', 'cone', 'cube', 'line', 'wall'] as const
const abilities = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'] as const
const units = ['instant', 'rounds', 'minutes', 'hours', 'until_dispelled'] as const
const castings = ['action', 'bonus_action', 'reaction', 'minutes'] as const
const triggers = ['when_hit', 'when_damaged', 'ally_attacked', 'creature_casts', 'creature_enters_reach'] as const
const partTypes = ['damage', 'condition', 'light', 'reveal', 'surface', 'manual'] as const
const damageTypes = ['acid', 'bludgeoning', 'cold', 'fire', 'force', 'lightning', 'necrotic', 'piercing', 'poison', 'psychic', 'radiant', 'slashing', 'thunder']
const conditions = ['blinded', 'charmed', 'deafened', 'frightened', 'grappled', 'incapacitated', 'invisible', 'paralyzed', 'petrified', 'poisoned', 'prone', 'restrained', 'stunned', 'unconscious']
const creatureTypes = ['aberration', 'beast', 'celestial', 'construct', 'dragon', 'elemental', 'fey', 'fiend', 'giant', 'humanoid', 'monstrosity', 'ooze', 'plant', 'undead']
const qualities = ['hidden', 'invisible', 'disguised', 'illusory', 'ethereal']
const words = (s: string) => s.replaceAll('_', ' ')
const adding = ref<SpellPart['type']>('damage')
// Where each new part starts.
const fresh: Record<SpellPart['type'], SpellPart> = {
  damage: { type: 'damage', when: 'on_cast', dice: '2d6', damageType: 'fire', half: true },
  condition: { type: 'condition', condition: 'frightened', onlyTypes: [] },
  light: { type: 'light', brightFt: 20, dimFt: 20 },
  reveal: { type: 'reveal', qualities: ['invisible'] },
  surface: { type: 'surface', surface: 'fog', rounds: 10 },
  manual: { type: 'manual', text: '' },
}
const dragged = ref(-1)

function addPart() {
  design.value?.parts.push(copy(fresh[adding.value]))
}
function removePart(i: number) {
  design.value?.parts.splice(i, 1)
}
function movePart(from: number, to: number) {
  const parts = design.value?.parts
  if (!parts || to < 0 || to >= parts.length || from === to) return
  const [p] = parts.splice(from, 1)
  if (p) parts.splice(to, 0, p)
}
function toggle(list: string[] | undefined, value: string): string[] {
  const now = list ?? []
  return now.includes(value) ? now.filter((v) => v !== value) : [...now, value]
}
function setMaterial(on: boolean) {
  if (design.value) design.value.components.material = on ? { text: '' } : undefined
}
</script>

<template>
  <main class="g-page builder">
    <RouterLink :to="{ name: 'library-entry', params: { entryId: String(route.params.entryId) } }" class="back">← Entry</RouterLink>
    <p v-if="spell.isError.value" role="alert" class="g-alert" data-testid="builder-error">This entry cannot be opened in the Effect builder.</p>
    <p v-else-if="!design">Opening the spell…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Effect builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="builder-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="builder-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="builder-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="builder-form" @submit.prevent="runSave">
          <fieldset class="grid">
            <legend>Targeting</legend>
            <label class="g-field"><span>Area</span>
              <select v-model="design.targeting.shape" data-testid="shape"><option v-for="s in shapes" :key="s" :value="s">{{ s }}</option></select>
            </label>
            <label class="g-field"><span>Size (ft)</span><input v-model.number="design.targeting.sizeFt" type="number" min="5" step="5" data-testid="size" /></label>
            <label class="g-field"><span>Range (ft, 0 from the caster)</span><input v-model.number="design.targeting.rangeFt" type="number" min="0" step="5" data-testid="range" /></label>
            <label class="g-field"><span>Save</span>
              <select v-model="design.save" data-testid="save">
                <option :value="undefined">None</option>
                <option v-for="a in abilities" :key="a" :value="a">{{ a }}</option>
              </select>
            </label>
          </fieldset>
          <fieldset class="grid">
            <legend>Casting</legend>
            <label class="g-field"><span>Casting time</span>
              <select v-model="design.castingTime.kind" data-testid="casting"><option v-for="c in castings" :key="c" :value="c">{{ words(c) }}</option></select>
            </label>
            <label v-if="design.castingTime.kind === 'minutes'" class="g-field"><span>Minutes</span><input v-model.number="design.castingTime.minutes" type="number" min="1" data-testid="casting-minutes" /></label>
            <label v-if="design.castingTime.kind === 'reaction'" class="g-field"><span>Trigger</span>
              <select v-model="design.castingTime.trigger" data-testid="trigger"><option v-for="t in triggers" :key="t" :value="t">{{ words(t) }}</option></select>
            </label>
            <label class="g-field"><span>Duration</span>
              <select v-model="design.duration.unit" data-testid="duration"><option v-for="u in units" :key="u" :value="u">{{ words(u) }}</option></select>
            </label>
            <label v-if="['rounds', 'minutes', 'hours'].includes(design.duration.unit)" class="g-field"><span>How many</span>
              <input v-model.number="design.duration.amount" type="number" min="1" data-testid="duration-amount" />
            </label>
            <label class="check"><input v-model="design.concentration" type="checkbox" data-testid="concentration" /><span>Concentration</span></label>
            <label class="check"><input v-model="design.ritual" type="checkbox" data-testid="ritual" /><span>Ritual</span></label>
          </fieldset>
          <fieldset class="grid">
            <legend>Components</legend>
            <label class="check"><input v-model="design.components.verbal" type="checkbox" data-testid="verbal" /><span>Verbal</span></label>
            <label class="check"><input v-model="design.components.somatic" type="checkbox" data-testid="somatic" /><span>Somatic</span></label>
            <label class="check">
              <input :checked="Boolean(design.components.material)" type="checkbox" data-testid="material" @change="setMaterial(($event.target as HTMLInputElement).checked)" />
              <span>Material</span>
            </label>
            <template v-if="design.components.material">
              <label class="g-field wide"><span>Material Component</span><input v-model="design.components.material.text" maxlength="200" data-testid="material-text" /></label>
              <label class="g-field"><span>Cost (gp)</span><input v-model.number="design.components.material.costGp" type="number" min="0" data-testid="material-cost" /></label>
              <label class="check"><input v-model="design.components.material.consumed" type="checkbox" data-testid="material-consumed" /><span>Consumed</span></label>
              <label class="g-field"><span>Item (slug)</span><input v-model="design.components.material.item" maxlength="80" data-testid="material-item" /></label>
            </template>
          </fieldset>

          <h2>Parts</h2>
          <ol class="parts" data-testid="parts">
            <!-- eslint-disable-next-line vuejs-accessibility/no-static-element-interactions -- dragging is a pointer shortcut; the arrows are the accessible path -->
            <li
              v-for="(p, i) in design.parts"
              :key="i"
              class="part"
              draggable="true"
              :data-testid="`part-${String(i)}`"
              @dragstart="dragged = i"
              @dragover.prevent
              @drop.prevent="movePart(dragged, i)"
            >
              <span class="kind">{{ p.type }}</span>
              <template v-if="p.type === 'damage'">
                <label class="g-field"><span>When</span><select v-model="p.when" :data-testid="`when-${String(i)}`"><option value="on_cast">on cast</option><option value="start_of_turn">start of a turn in the area</option></select></label>
                <label class="g-field"><span>Dice</span><input v-model="p.dice" maxlength="8" :data-testid="`dice-${String(i)}`" /></label>
                <label class="g-field"><span>Type</span><select v-model="p.damageType" :data-testid="`damage-type-${String(i)}`"><option v-for="d in damageTypes" :key="d" :value="d">{{ d }}</option></select></label>
                <label v-if="p.when === 'on_cast'" class="check"><input v-model="p.half" type="checkbox" /><span>Half on a save</span></label>
              </template>
              <template v-else-if="p.type === 'condition'">
                <label class="g-field"><span>Condition</span><select v-model="p.condition" :data-testid="`condition-${String(i)}`"><option v-for="c in conditions" :key="c" :value="c">{{ c }}</option></select></label>
                <fieldset class="types">
                  <legend>Only these creatures (none: everyone)</legend>
                  <label v-for="ct in creatureTypes" :key="ct" class="check">
                    <input type="checkbox" :checked="p.onlyTypes?.includes(ct)" :data-testid="`only-${String(i)}-${ct}`" @change="p.onlyTypes = toggle(p.onlyTypes, ct)" />
                    <span>{{ ct }}</span>
                  </label>
                </fieldset>
              </template>
              <template v-else-if="p.type === 'light'">
                <label class="g-field"><span>Bright (ft)</span><input v-model.number="p.brightFt" type="number" min="5" step="5" :data-testid="`bright-${String(i)}`" /></label>
                <label class="g-field"><span>Dim (ft)</span><input v-model.number="p.dimFt" type="number" min="0" step="5" :data-testid="`dim-${String(i)}`" /></label>
              </template>
              <fieldset v-else-if="p.type === 'reveal'" class="types">
                <legend>Strips</legend>
                <label v-for="q in qualities" :key="q" class="check">
                  <input type="checkbox" :checked="p.qualities?.includes(q)" :data-testid="`quality-${String(i)}-${q}`" @change="p.qualities = toggle(p.qualities, q)" />
                  <span>{{ q }}</span>
                </label>
              </fieldset>
              <template v-else-if="p.type === 'surface'">
                <label class="g-field"><span>Surface</span><input v-model="p.surface" maxlength="40" :data-testid="`surface-${String(i)}`" /></label>
                <label class="g-field"><span>Rounds</span><input v-model.number="p.rounds" type="number" min="1" :data-testid="`rounds-${String(i)}`" /></label>
              </template>
              <label v-else class="g-field wide"><span>What the DM does</span><input v-model="p.text" maxlength="500" :data-testid="`text-${String(i)}`" /></label>
              <span class="row tools">
                <button type="button" :aria-label="`Move part ${String(i + 1)} up`" :data-testid="`up-${String(i)}`" @click="movePart(i, i - 1)">↑</button>
                <button type="button" :aria-label="`Move part ${String(i + 1)} down`" :data-testid="`down-${String(i)}`" @click="movePart(i, i + 1)">↓</button>
                <button type="button" :aria-label="`Remove part ${String(i + 1)}`" :data-testid="`remove-${String(i)}`" @click="removePart(i)">×</button>
              </span>
            </li>
          </ol>
          <div class="row">
            <label class="g-field"><span>Add a part</span>
              <select v-model="adding" data-testid="add-type"><option v-for="t in partTypes" :key="t" :value="t">{{ t }}</option></select>
            </label>
            <GButton data-testid="add-part" @click="addPart">Add</GButton>
          </div>
          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="save-spell">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="Preview" data-testid="builder-preview">
          <h2>Preview</h2>
          <AreaPreview v-if="shown" :hexes="shown.hexes" />
          <ul class="text" data-testid="rules-text">
            <li v-for="(line, i) in shown?.text ?? []" :key="i">{{ line }}</li>
          </ul>
          <p v-if="shown" class="hint">Runs in play as <code>{{ shown.effect }}</code>.</p>
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
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.wide {
  grid-column: 1 / -1;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
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
  flex-wrap: wrap;
  align-items: end;
  gap: 8px 12px;
  padding: 10px 0;
  border-top: 1px solid var(--color-line);
  cursor: grab;
}
.kind {
  min-width: 80px;
  font-family: var(--font-display);
  color: var(--color-gold-high);
}
.types {
  display: flex;
  flex-wrap: wrap;
  gap: 0 12px;
  margin: 0;
  padding: 0;
  border: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.tools {
  margin-left: auto;
}
.tools button {
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
.text {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding-left: 18px;
}
</style>
