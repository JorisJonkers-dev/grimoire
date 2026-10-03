<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import {
  createLibraryCollectionMutation,
  listLibraryCollectionsOptions,
  updateLibraryCollectionMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LibraryCollection, LibraryEntry } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'
import { kindNames } from './fields'

const props = defineProps<{ entries: LibraryEntry[] }>()
const emit = defineEmits<{ export: [collectionId: string] }>()
const client = useQueryClient()
const collections = useQuery(computed(() => ({ ...listLibraryCollectionsOptions(), retry: false })))
const create = useMutation(createLibraryCollectionMutation())
const update = useMutation(updateLibraryCollectionMutation())
const problem = computed(() => create.error.value ?? update.error.value)
const name = ref('')
// The Collection open for editing, and its draft.
const open = ref('')
const draft = ref({ name: '', description: '', entryIds: [] as string[] })
const done = () => void client.invalidateQueries()

function add() {
  create.mutate({ body: { name: name.value.trim() } }, {
    onSuccess: (c) => {
      name.value = ''
      edit(c)
      done()
    },
  })
}
function edit(c: LibraryCollection) {
  open.value = c.id
  draft.value = { name: c.name, description: c.description, entryIds: [...c.entryIds] }
}
function toggle(id: string) {
  const ids = draft.value.entryIds
  draft.value.entryIds = ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id]
}
function save() {
  update.mutate({ path: { collectionId: open.value }, body: { ...draft.value, name: draft.value.name.trim() } }, {
    onSuccess: () => {
      open.value = ''
      done()
    },
  })
}
const count = (c: LibraryCollection) => `${String(c.entryIds.length)} ${c.entryIds.length === 1 ? 'entry' : 'entries'}`
</script>

<template>
  <section class="g-card stack" aria-label="Collections" data-testid="collections">
    <h2>Collections</h2>
    <p class="hint">Group entries, such as a homebrew expansion, and switch the group on per Campaign.</p>
    <p v-if="problem" role="alert" class="g-alert" data-testid="collections-problem">{{ problem.detail ?? 'That Collection was not saved.' }}</p>
    <ul class="g-list">
      <li v-for="c in collections.data.value ?? []" :key="c.id" :data-testid="`collection-${c.name}`">
        <template v-if="open !== c.id">
          <span class="who">{{ c.name }} · {{ count(c) }}</span>
          <span class="row">
            <GButton :data-testid="`collection-edit-${c.name}`" @click="edit(c)">Edit</GButton>
            <GButton :data-testid="`collection-export-${c.name}`" @click="emit('export', c.id)">Export</GButton>
          </span>
        </template>
        <form v-else class="stack" data-testid="collection-form" @submit.prevent="save">
          <GField v-model="draft.name" label="Name" :maxlength="80" data-testid="collection-name" />
          <GField v-model="draft.description" label="Description" :maxlength="2000" multiline data-testid="collection-description" />
          <fieldset class="picks">
            <legend>Entries</legend>
            <label v-for="e in props.entries" :key="e.id" class="check">
              <input type="checkbox" :checked="draft.entryIds.includes(e.id)" :data-testid="`collect-${e.name}`" @change="toggle(e.id)" />
              <span>{{ e.name }} <small>{{ kindNames[e.kind] }}</small></span>
            </label>
          </fieldset>
          <div class="row">
            <GButton type="submit" variant="primary" :disabled="draft.name.trim() === '' || update.isPending.value" data-testid="collection-save">Save</GButton>
            <GButton data-testid="collection-cancel" @click="open = ''">Cancel</GButton>
          </div>
        </form>
      </li>
    </ul>
    <form class="row" data-testid="collection-create" @submit.prevent="add">
      <GField v-model="name" class="grow" label="New Collection" :maxlength="80" data-testid="collection-new-name" />
      <GButton type="submit" :disabled="name.trim() === '' || create.isPending.value" data-testid="collection-add">Add</GButton>
    </form>
  </section>
</template>

<style scoped>
h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
}
.hint {
  margin: 0;
  color: var(--color-text-2);
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  gap: 8px;
}
.grow {
  flex: 1 1 200px;
}
.g-list li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.g-list li form {
  flex: 1 1 100%;
}
.picks {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 4px 12px;
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
small {
  color: var(--color-text-2);
}
</style>
