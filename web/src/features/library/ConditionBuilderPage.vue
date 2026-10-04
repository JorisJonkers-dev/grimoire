<script setup lang="ts">
import BuilderCrumbs from './BuilderCrumbs.vue'
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getConditionBuildOptions, previewConditionMutation, saveConditionBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton, StatusIcon } from '@/shared/ui'
import { abilities } from './classPresets'
import { useBuilder } from './useBuilder'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const condition = useQuery(computed(() => ({ ...getConditionBuildOptions(path.value), retry: false })))
const preview = useMutation(previewConditionMutation())
const save = useMutation(saveConditionBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: condition.data, key: computed(() => getConditionBuildOptions(path.value).queryKey), preview, save, fallback: 'Condition',
})

// Only a condition that lasts until cured names a cure.
function lasts() {
  if (design.value && design.value.ends !== 'cure') delete design.value.cure
}

const icons = ['drop', 'flame', 'snow', 'skull', 'spiral', 'eye', 'chain', 'star', 'moon', 'leaf', 'bolt', 'heart', 'shield', 'cloud']
const parts = [
  { value: 'attack_advantage', label: 'Its attacks have Advantage' },
  { value: 'attack_disadvantage', label: 'Its attacks have Disadvantage' },
  { value: 'attacked_advantage', label: 'Attacks against it have Advantage' },
  { value: 'attacked_disadvantage', label: 'Attacks against it have Disadvantage' },
  { value: 'save_advantage', label: 'Advantage on a save' },
  { value: 'save_disadvantage', label: 'Disadvantage on a save' },
  { value: 'save_fails', label: 'Fails a save' },
  { value: 'incapacitated', label: 'Incapacitated' },
  { value: 'immobile', label: 'Speed 0' },
  { value: 'speed_penalty', label: 'Slower' },
  { value: 'crit_within', label: 'Hits from nearby are Critical Hits' },
  { value: 'manual', label: 'The DM resolves' },
]
</script>

<template>
  <main class="g-page builder">
    <BuilderCrumbs :entry-id="entryId" here="Condition builder" />
    <p v-if="condition.isError.value" role="alert" class="g-alert" data-testid="condition-error">This entry cannot be opened in the condition builder.</p>
    <p v-else-if="!design">Opening the condition…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Condition builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="condition-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="condition-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="condition-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="condition-form" @submit.prevent="runSave">
          <div class="grid">
            <label class="g-field"><span>Icon</span>
              <select v-model="design.icon" data-testid="condition-icon"><option v-for="i in icons" :key="i" :value="i">{{ i }}</option></select>
            </label>
            <label class="g-field"><span>Colour</span><input v-model="design.color" type="color" data-testid="condition-color" /></label>
            <span class="sample" data-testid="condition-sample"><StatusIcon slug="" :label="name" :icon="design.icon" :color="design.color" :size="28" /></span>
          </div>
          <label class="g-field"><span>What it is</span><textarea v-model="design.text" rows="3" maxlength="2000" data-testid="condition-text" /></label>
          <div class="grid">
            <label class="g-field"><span>Lasts</span>
              <select v-model="design.ends" data-testid="condition-ends" @change="lasts()">
                <option value="removed">until removed</option>
                <option value="rest">until a rest</option>
                <option value="save">until a save at the end of each turn</option>
                <option value="cure">until cured: a lingering injury</option>
              </select>
            </label>
            <label v-if="design.ends === 'cure'" class="g-field"><span>What cures it</span><input v-model="design.cure" maxlength="80" data-testid="condition-cure" /></label>
            <label v-if="design.ends === 'save'" class="g-field"><span>Save</span>
              <select v-model="design.ability" data-testid="condition-ability"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
            </label>
          </div>
          <fieldset class="grid">
            <legend>Stacking</legend>
            <label class="check"><input v-model="design.stacks" type="checkbox" data-testid="condition-stacks" /><span>Stacks in levels</span></label>
            <template v-if="design.stacks">
              <label class="g-field"><span>Up to level</span><input v-model.number="design.maxLevel" type="number" min="2" max="10" data-testid="condition-max-level" /></label>
              <label class="g-field"><span>D20 penalty per level</span><input v-model.number="design.perLevel.d20" type="number" min="0" max="5" data-testid="condition-d20" /></label>
              <label class="g-field"><span>Speed lost per level (ft)</span><input v-model.number="design.perLevel.speedFt" type="number" min="0" max="30" step="5" data-testid="condition-speed" /></label>
              <label class="g-field"><span>Kills at level (0 never)</span><input v-model.number="design.perLevel.deathAt" type="number" min="0" max="10" data-testid="condition-death" /></label>
            </template>
          </fieldset>

          <h2>What it does</h2>
          <ol class="rows">
            <li v-for="(p, i) in design.parts" :key="i" class="row-item">
              <label class="g-field"><span>Part</span>
                <select v-model="p.type" :data-testid="`condition-part-${String(i)}-type`"><option v-for="t in parts" :key="t.value" :value="t.value">{{ t.label }}</option></select>
              </label>
              <label v-if="p.type.startsWith('save_')" class="g-field"><span>Ability</span>
                <select v-model="p.ability" :data-testid="`condition-part-${String(i)}-ability`"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
              </label>
              <label v-if="p.type === 'speed_penalty' || p.type === 'crit_within'" class="g-field small"><span>Feet</span>
                <input v-model.number="p.feet" type="number" min="5" max="60" step="5" :data-testid="`condition-part-${String(i)}-feet`" />
              </label>
              <label v-if="p.type === 'manual'" class="g-field wide"><span>What the DM does</span>
                <input v-model="p.text" maxlength="500" :data-testid="`condition-part-${String(i)}-text`" />
              </label>
              <button type="button" class="drop" :aria-label="`Remove part ${String(i + 1)}`" :data-testid="`condition-part-${String(i)}-remove`" @click="design.parts.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="condition-add-part" @click="design.parts.push({ type: 'attacked_advantage' })">+ Part</GButton>

          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="condition-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="condition-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="How it reads" data-testid="condition-lines">
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
  align-items: end;
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.sample {
  display: flex;
  align-items: center;
  min-height: 44px;
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
