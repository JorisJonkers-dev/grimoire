<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getRollTableBuildOptions, previewRollTableMutation, saveRollTableBuildMutation } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { RollTableResult } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { useBuilder } from './useBuilder'

// A Roll Table: its dice, and for each range of what they can show a result that may apply an Effect
// to whoever rolled or give an Item.
const route = useRoute()
const entryId = computed(() => String(route.params.entryId))
const path = computed(() => ({ path: { entryId: entryId.value } }))
const table = useQuery(computed(() => ({ ...getRollTableBuildOptions(path.value), retry: false })))
const preview = useMutation(previewRollTableMutation())
const save = useMutation(saveRollTableBuildMutation())
const { design, shown, status, name, readOnly, problem, runPreview, runSave } = useBuilder({
  entryId, data: table.data, key: computed(() => getRollTableBuildOptions(path.value).queryKey), preview, save, fallback: 'Roll Table',
})

// A new result starts where the last one ended.
function addResult() {
  if (!design.value) return
  const next = (design.value.results.at(-1)?.to ?? 0) + 1
  design.value.results.push({ from: next, to: next, text: '' })
}
// How many of an Item a result gives goes when its Item does.
function setItem(r: RollTableResult, slug: string) {
  r.item = slug
  if (slug === '') delete r.quantity
}
</script>

<template>
  <main class="g-page builder">
    <RouterLink :to="{ name: 'library-entry', params: { entryId } }" class="back">← Entry</RouterLink>
    <p v-if="table.isError.value" role="alert" class="g-alert" data-testid="table-error">This entry cannot be opened in the Roll Table builder.</p>
    <p v-else-if="!design">Opening the table…</p>
    <template v-else>
      <h1>{{ name }} <span class="g-tag">Roll Table builder</span></h1>
      <p v-if="readOnly" class="hint" data-testid="table-readonly">A Shared Library copy: preview it here; it cannot be changed.</p>
      <p v-if="status" role="status" class="g-tag" data-testid="table-status">{{ status }}</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="table-problem">{{ problem.detail ?? 'That table will not do.' }}</p>
      <div class="layout">
        <form class="g-card stack" data-testid="table-form" @submit.prevent="runSave">
          <label class="g-field small"><span>Dice</span><input v-model="design.dice" maxlength="20" placeholder="1d20" data-testid="table-dice" /></label>
          <p class="hint">Up to ten dice of one kind, added up. A range the table leaves out is a roll on which nothing happens.</p>
          <h2>Results</h2>
          <ol class="rows">
            <li v-for="(r, i) in design.results" :key="i" class="row-item">
              <label class="g-field small"><span>From</span><input v-model.number="r.from" type="number" min="1" max="1000" :data-testid="`table-result-${String(i)}-from`" /></label>
              <label class="g-field small"><span>To</span><input v-model.number="r.to" type="number" min="1" max="1000" :data-testid="`table-result-${String(i)}-to`" /></label>
              <label class="g-field wide"><span>What happens</span><input v-model="r.text" maxlength="400" :data-testid="`table-result-${String(i)}-text`" /></label>
              <label class="g-field"><span>Effect it applies, by slug</span><input v-model="r.effect" maxlength="80" :data-testid="`table-result-${String(i)}-effect`" /></label>
              <label class="g-field"><span>Item it gives, by slug</span>
                <input :value="r.item ?? ''" maxlength="80" :data-testid="`table-result-${String(i)}-item`" @input="setItem(r, ($event.target as HTMLInputElement).value)" />
              </label>
              <label v-if="r.item" class="g-field small"><span>How many</span>
                <input v-model.number="r.quantity" type="number" min="1" max="100" :data-testid="`table-result-${String(i)}-quantity`" />
              </label>
              <button type="button" class="drop" :aria-label="`Remove result ${String(i + 1)}`" :data-testid="`table-result-${String(i)}-remove`" @click="design.results.splice(i, 1)">×</button>
            </li>
          </ol>
          <GButton data-testid="table-add-result" @click="addResult()">+ Result</GButton>
          <div class="row">
            <GButton :disabled="preview.isPending.value" data-testid="table-preview" @click="runPreview">Preview</GButton>
            <GButton v-if="!readOnly" type="submit" variant="primary" :disabled="save.isPending.value" data-testid="table-save">Save to the Library</GButton>
          </div>
        </form>
        <aside class="g-card stack" aria-label="How it reads" data-testid="table-lines">
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
.hint,
.line {
  margin: 0;
}
.hint {
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
input {
  min-height: 44px;
  min-width: 0;
}
</style>
