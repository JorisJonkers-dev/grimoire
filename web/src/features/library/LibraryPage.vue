<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { createLibraryEntryMutation, listLibraryEntriesOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LibraryField, LibraryKind } from '@/infrastructure/api/types.gen'
import { GButton, GField, GRow, GTabs } from '@/shared/ui'
import CollectionsPanel from './CollectionsPanel.vue'
import TransferPanel from './TransferPanel.vue'
import FieldsEditor from './FieldsEditor.vue'
import { cleanFields, kindNames } from './fields'

const router = useRouter()
const client = useQueryClient()
const kinds = Object.keys(kindNames) as LibraryKind[]
const shown = ref('all')
const tabs = computed(() => [{ value: 'all', label: 'All' }, ...kinds.map((k) => ({ value: k, label: kindNames[k] ?? k }))])
const entries = useQuery(computed(() => ({ ...listLibraryEntriesOptions(), retry: false })))
const list = computed(() => (entries.data.value ?? []).filter((e) => shown.value === 'all' || e.kind === shown.value))
const create = useMutation(createLibraryEntryMutation())
const kind = ref<LibraryKind>('creature')
const name = ref('')
const fields = ref<LibraryField[]>([])
const transfer = ref<InstanceType<typeof TransferPanel>>()

function add() {
  create.mutate(
    { body: { kind: kind.value, name: name.value.trim(), fields: cleanFields(fields.value) } },
    {
      onSuccess: (e) => {
        name.value = ''
        fields.value = []
        void client.invalidateQueries()
        void router.push({ name: 'library-entry', params: { entryId: e.id } })
      },
    },
  )
}
</script>

<template>
  <main class="g-page library">
    <h1>Library</h1>
    <RouterLink :to="{ name: 'shared-library' }" data-testid="shared-library-link">Browse the Shared Library</RouterLink>
    <p class="hint">Build creatures, NPCs, places, shops, items, spells and tables once, then link them into any Campaign you run.</p>
    <p v-if="entries.isError.value" role="alert" class="g-alert" data-testid="library-error">Your Library could not be opened.</p>
    <template v-else>
      <GTabs v-model="shown" label="Kinds" :tabs="tabs" />
      <p v-if="entries.isSuccess.value && list.length === 0" class="hint" data-testid="library-empty">Nothing here yet.</p>
      <ul class="g-list" data-testid="library-list">
        <li v-for="e in list" :key="e.id">
          <GRow :to="{ name: 'library-entry', params: { entryId: e.id } }" :title="e.name" :subtitle="`Revision ${String(e.revision)}`">
            <template #trailing><span class="g-tag">{{ kindNames[e.kind] }}</span></template>
          </GRow>
        </li>
      </ul>
      <form class="g-card stack" data-testid="library-create" @submit.prevent="add">
        <h2>New entry</h2>
        <label class="g-field">
          <span>Kind</span>
          <select v-model="kind" data-testid="library-kind">
            <option v-for="k in kinds" :key="k" :value="k">{{ kindNames[k] }}</option>
          </select>
        </label>
        <GField v-model="name" label="Name" :maxlength="80" data-testid="library-name" />
        <FieldsEditor v-model="fields" label="Fields" />
        <p v-if="create.error.value" role="alert" class="g-alert">{{ create.error.value.detail ?? 'That entry was not saved.' }}</p>
        <GButton type="submit" variant="primary" :disabled="name.trim() === '' || create.isPending.value" data-testid="library-add">Add to the Library</GButton>
      </form>
      <CollectionsPanel :entries="entries.data.value ?? []" @export="(id) => transfer?.download(id)" />
      <TransferPanel ref="transfer" />
    </template>
  </main>
</template>

<style scoped>
.library {
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
.stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.g-list {
  gap: 0;
}
</style>
