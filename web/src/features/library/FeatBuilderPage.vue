<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getFeatBuildOptions, previewFeatMutation, saveFeatBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'
import { abilities } from './classPresets'

const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const feat = useQuery(computed(() => ({ ...getFeatBuildOptions(path.value), retry: false })))
const preview = useMutation(previewFeatMutation())
const save = useMutation(saveFeatBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: feat.data, key: computed(() => getFeatBuildOptions(path.value).queryKey), preview, save, fallback: 'Feat',
})
const categories = [
  { value: 'origin', label: 'Origin' },
  { value: 'general', label: 'General' },
  { value: 'fighting_style', label: 'Fighting Style' },
  { value: 'epic_boon', label: 'Epic Boon' },
]
const kinds = [
  { value: 'level', label: 'Character level' },
  { value: 'ability', label: 'Ability score' },
  { value: 'spellcasting', label: 'Spellcasting' },
  { value: 'feat', label: 'Another feat' },
]

</script>

<template>
  <main class="g-page builder">
    <RouterLink :to="{ name: 'library-entry', params: { entryId: String(route.params.entryId) } }" class="back">← Entry</RouterLink>
    <p v-if="feat.isError.value" role="alert" class="g-alert" data-testid="feat-error">This entry cannot be opened in the feat builder.</p>
    <p v-else-if="!design">Opening the feat…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Feat builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="feat-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="feat-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="feat-problem">{{ problem.detail ?? 'That design did not build.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="feat-form" @submit.prevent="runSave">
          <div class="grid">
            <label class="g-field"><span>Category</span>
              <select v-model="design.category" data-testid="feat-category"><option v-for="c in categories" :key="c.value" :value="c.value">{{ c.label }}</option></select>
            </label>
            <label class="check"><input v-model="design.repeatable" type="checkbox" data-testid="feat-repeatable" /><span>Can be taken more than once</span></label>
          </div>
          <label class="g-field"><span>What it does</span><textarea v-model="design.text" rows="5" maxlength="4000" data-testid="feat-text" /></label>

          <h2>Prerequisites</h2>
          <p class="hint">Prerequisites in the same group are alternatives; every group must be met.</p>
          <ol class="rows">
            <li v-for="(p, i) in design.prerequisites" :key="i" class="row-item">
              <label class="g-field"><span>Needs</span>
                <select v-model="p.kind" :data-testid="`feat-pre-${String(i)}-kind`"><option v-for="k in kinds" :key="k.value" :value="k.value">{{ k.label }}</option></select>
              </label>
              <label v-if="p.kind === 'ability'" class="g-field"><span>Ability</span>
                <select v-model="p.ability" :data-testid="`feat-pre-${String(i)}-ability`"><option v-for="a in abilities" :key="a" :value="a">{{ a }}</option></select>
              </label>
              <label v-if="p.kind === 'level' || p.kind === 'ability'" class="g-field small"><span>At least</span>
                <input v-model.number="p.minimum" type="number" min="1" max="30" :data-testid="`feat-pre-${String(i)}-minimum`" />
              </label>
              <label v-if="p.kind === 'feat'" class="g-field"><span>Feat (slug)</span><input v-model="p.feat" maxlength="80" :data-testid="`feat-pre-${String(i)}-feat`" /></label>
              <label class="g-field small"><span>Group</span><input v-model.number="p.group" type="number" min="0" max="9" :data-testid="`feat-pre-${String(i)}-group`" /></label>
              <button type="button" class="drop" :aria-label="`Remove prerequisite ${String(i + 1)}`" :data-testid="`feat-pre-${String(i)}-remove`" @click="design.prerequisites.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="feat-add-pre" @click="design.prerequisites.push({ kind: 'level', minimum: 4, group: design.prerequisites.length })">+ Prerequisite</GButton>

          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="feat-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="feat-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="How it reads" data-testid="feat-lines">
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
  white-space: pre-line;
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
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px 12px;
  align-items: end;
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
  border-top: 1px solid var(--color-line);
}
.small {
  max-width: 110px;
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
