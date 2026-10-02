<script setup lang="ts">
import type { LibraryField } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

// Named values as editable rows; a row without a name is dropped when saved.
const rows = defineModel<LibraryField[]>({ required: true })
defineProps<{ label: string; placeholders?: Record<string, string> }>()

function add() {
  rows.value = [...rows.value, { name: '', value: '' }]
}
function remove(i: number) {
  rows.value = rows.value.filter((_, j) => j !== i)
}
</script>

<template>
  <fieldset class="fields">
    <legend>{{ label }}</legend>
    <div v-for="(row, i) in rows" :key="i" class="row">
      <input v-model="row.name" maxlength="60" :aria-label="`${label}: name of field ${String(i + 1)}`" placeholder="Field" :data-testid="`field-name-${String(i)}`" />
      <input
        v-model="row.value"
        maxlength="4000"
        :aria-label="`${label}: value of field ${String(i + 1)}`"
        :placeholder="placeholders?.[row.name] ?? 'Value'"
        :data-testid="`field-value-${String(i)}`"
      />
      <button type="button" class="drop" :aria-label="`Remove field ${String(i + 1)}`" :data-testid="`field-remove-${String(i)}`" @click="remove(i)">×</button>
    </div>
    <GButton data-testid="field-add" @click="add">Add a field</GButton>
  </fieldset>
</template>

<style scoped>
.fields {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  border: 0;
}
legend {
  margin-bottom: 4px;
  font-family: var(--font-display);
}
.row {
  display: grid;
  grid-template-columns: minmax(80px, 1fr) minmax(120px, 2fr) 44px;
  gap: 6px;
}
input {
  min-height: 44px;
  min-width: 0;
  padding: 0 8px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-raised);
  color: var(--color-text);
  font: inherit;
}
.drop {
  min-height: 44px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-2);
  cursor: pointer;
}
</style>
