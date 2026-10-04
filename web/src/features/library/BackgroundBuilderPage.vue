<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getBackgroundBuildOptions, previewBackgroundMutation, saveBackgroundBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'
import { abilities } from './classPresets'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const background = useQuery(computed(() => ({ ...getBackgroundBuildOptions(path.value), retry: false })))
const preview = useMutation(previewBackgroundMutation())
const save = useMutation(saveBackgroundBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: background.data, key: computed(() => getBackgroundBuildOptions(path.value).queryKey), preview, save, fallback: 'Background',
})
const skills = [
  'acrobatics', 'animal-handling', 'arcana', 'athletics', 'deception', 'history', 'insight', 'intimidation', 'investigation',
  'medicine', 'nature', 'perception', 'performance', 'persuasion', 'religion', 'sleight-of-hand', 'stealth', 'survival',
]

</script>

<template>
  <main class="g-page builder">
    <RouterLink :to="{ name: 'library-entry', params: { entryId: String(route.params.entryId) } }" class="back">← Entry</RouterLink>
    <p v-if="background.isError.value" role="alert" class="g-alert" data-testid="background-error">This entry cannot be opened in the background builder.</p>
    <p v-else-if="!design">Opening the background…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Background builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="background-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="background-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="background-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="background-form" @submit.prevent="runSave">
          <fieldset class="grid">
            <legend>Ability Scores (three to raise)</legend>
            <label v-for="(_, i) in design.abilities" :key="i" class="g-field"><span>Ability {{ i + 1 }}</span>
              <select v-model="design.abilities[i]" :data-testid="`background-ability-${String(i)}`"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
            </label>
          </fieldset>
          <fieldset class="grid">
            <legend>Skill Proficiencies</legend>
            <label v-for="(_, i) in design.skills" :key="i" class="g-field"><span>Skill {{ i + 1 }}</span>
              <select v-model="design.skills[i]" :data-testid="`background-skill-${String(i)}`"><option v-for="s in skills" :key="s" :value="s">{{ s }}</option></select>
            </label>
          </fieldset>
          <fieldset class="grid">
            <legend>Origin feat, tool and equipment</legend>
            <label class="g-field"><span>Origin feat</span><input v-model="design.featName" maxlength="60" data-testid="background-feat-name" /></label>
            <label class="g-field"><span>Feat slug</span><input v-model="design.feat" maxlength="80" data-testid="background-feat" /></label>
            <label class="g-field"><span>Tool proficiency</span><input v-model="design.tool" maxlength="80" data-testid="background-tool" /></label>
            <label class="g-field"><span>Or gold (GP)</span><input v-model.number="design.gold" type="number" min="0" max="1000" data-testid="background-gold" /></label>
          </fieldset>
          <label class="g-field"><span>Equipment</span><textarea v-model="design.equipment" rows="2" maxlength="500" data-testid="background-equipment" /></label>
          <label class="g-field"><span>About the background</span><textarea v-model="design.text" rows="3" maxlength="2000" data-testid="background-text" /></label>
          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="background-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="background-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="How it reads" data-testid="background-lines">
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
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
input,
select,
textarea {
  min-height: 44px;
  min-width: 0;
}
</style>
