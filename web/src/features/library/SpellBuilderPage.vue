<script setup lang="ts">
import BuilderCrumbs from './BuilderCrumbs.vue'
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getSpellBuildOptions, listPartKindsOptions, previewSpellMutation, saveSpellBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { SpellBuild, SpellDesign } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'
import AreaPreview from './AreaPreview.vue'
import PartsEditor from './PartsEditor.vue'
import ScalingEditor from './ScalingEditor.vue'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const spell = useQuery(computed(() => ({ ...getSpellBuildOptions(path.value), retry: false })))
const preview = useMutation(previewSpellMutation())
const save = useMutation(saveSpellBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder<SpellDesign, SpellBuild>({
  entryId, data: spell.data, key: computed(() => getSpellBuildOptions(path.value).queryKey), preview, save, fallback: 'Spell',
})
const partKinds = useQuery(listPartKindsOptions())
const kinds = computed(() => partKinds.data.value ?? [])
const holds = computed(() => spell.data.value?.holds)
const activation = computed(() => spell.data.value?.activation ?? [])

const shapes = ['sphere', 'cylinder', 'emanation', 'ring', 'cone', 'cube', 'line', 'wall'] as const
const abilities = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'] as const
const units = ['instant', 'rounds', 'minutes', 'hours', 'until_dispelled'] as const
const castings = ['action', 'bonus_action', 'reaction', 'minutes'] as const
const triggers = ['when_hit', 'when_damaged', 'ally_attacked', 'creature_casts', 'creature_enters_reach'] as const
const damageTypes = ['acid', 'bludgeoning', 'cold', 'fire', 'force', 'lightning', 'necrotic', 'piercing', 'poison', 'psychic', 'radiant', 'slashing', 'thunder']
const words = (s: string) => s.replaceAll('_', ' ')

function setTurnDamage(on: boolean) {
  if (design.value) design.value.turnDamage = on ? { dice: '2d6', damageType: 'fire' } : undefined
}
function setMaterial(on: boolean) {
  if (design.value) design.value.components.material = on ? { text: '' } : undefined
}
</script>

<template>
  <main class="g-page builder">
    <BuilderCrumbs :entry-id="String(route.params.entryId)" here="Effect builder" />
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

          <fieldset class="grid">
            <legend>Start of a turn</legend>
            <label class="check wide">
              <input :checked="Boolean(design.turnDamage)" type="checkbox" data-testid="turn-damage" @change="setTurnDamage(($event.target as HTMLInputElement).checked)" />
              <span>Damage each creature that starts its turn in the area</span>
            </label>
            <template v-if="design.turnDamage">
              <label class="g-field"><span>Dice</span><input v-model="design.turnDamage.dice" maxlength="8" data-testid="turn-dice" /></label>
              <label class="g-field"><span>Type</span>
                <select v-model="design.turnDamage.damageType" data-testid="turn-type"><option v-for="d in damageTypes" :key="d" :value="d">{{ d }}</option></select>
              </label>
            </template>
          </fieldset>

          <fieldset>
            <legend>Scaling</legend>
            <p class="hint">How the area and the casting grow with the slot it is cast with; a Part's own fields scale beneath it.</p>
            <ScalingEditor v-model="design.scaling" :fields="activation" label="Spell scaling" prefix="scaling" />
          </fieldset>

          <h2>Parts</h2>
          <PartsEditor v-if="holds" v-model="design.parts" :kinds="kinds" :holds="holds" :allowed="holds.kinds" label="Parts" adds="Add a Part" noun="part" prefix="parts" scales />
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
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
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
