<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  createLootTableMutation,
  deleteLootTableMutation,
  listLootTableRevisionsOptions,
  listLootTablesOptions,
  restoreLootTableRevisionMutation,
  updateLootTableMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LootEntry, LootTable, LootTableInput } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import LootEditor from './LootEditor.vue'
import RevisionHistory from './RevisionHistory.vue'

const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const tables = useQuery({ ...listLootTablesOptions({ path: { campaignId } }), retry: false })
const editing = ref<string | null>(null)
const history = ref<string | null>(null)
const revisions = useQuery(computed(() => ({ ...listLootTableRevisionsOptions({ path: { campaignId, lootTableId: history.value ?? '' } }), enabled: history.value !== null })))
const failed = ref('')
const create = useMutation(createLootTableMutation())
const update = useMutation(updateLootTableMutation())
const remove = useMutation(deleteLootTableMutation())
const restore = useMutation(restoreLootTableRevisionMutation())
const fail = (what: string) => (err: unknown) => {
  const detail = (err as { detail?: string } | null)?.detail
  failed.value = detail ? `${what}: ${detail}` : `${what} failed. Try again shortly.`
}
const done = (close: () => void) => ({
  onSuccess: () => {
    failed.value = ''
    close()
    void client.invalidateQueries()
  },
})
function save(body: LootTableInput) {
  const close = () => (editing.value = null)
  if (editing.value === 'new') create.mutate({ path: { campaignId }, body }, { ...done(close), onError: fail('Saving the loot table') })
  else update.mutate({ path: { campaignId, lootTableId: editing.value ?? '' }, body }, { ...done(close), onError: fail('Saving the loot table') })
}
const nameOf = (id?: string) => tables.data.value?.find((t) => t.id === id)?.name ?? 'a table'
const entryText = (e: LootEntry) =>
  e.kind === 'item' ? `${e.amount ?? ''} ${e.itemSlug ?? ''}` : e.kind === 'currency' ? `${e.amount ?? ''} ${e.coin ?? ''}` : e.kind === 'table' ? `roll ${nameOf(e.tableId)}` : 'nothing'
const summary = (t: LootTable) => t.entries.map((e) => `${String(e.weight)}× ${entryText(e)}`).join(' · ')
</script>

<template>
  <main class="g-page loot">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Campaign prep</span>
      <h1>Loot tables</h1>
    </header>
    <p v-if="tables.isError.value" role="alert" class="g-alert" data-testid="loot-refused">Only the DM can prepare loot.</p>
    <template v-else>
      <p v-if="failed" role="alert" class="g-alert" data-testid="loot-error">{{ failed }}</p>
      <ul class="g-list">
        <li v-for="t in tables.data.value ?? []" :key="t.id" :data-testid="`loot-${t.name}`">
          <p>
            <strong>{{ t.name }}</strong> · rolled {{ t.rolls }}× · {{ summary(t) }}
          </p>
          <div class="row">
            <GButton :aria-label="`Edit ${t.name}`" @click="editing = t.id">Edit</GButton>
            <GButton :aria-label="`History of ${t.name}`" @click="history = history === t.id ? null : t.id">History</GButton>
            <GButton variant="danger" :aria-label="`Delete ${t.name}`" @click="remove.mutate({ path: { campaignId, lootTableId: t.id } }, { ...done(() => {}), onError: fail('Deleting the loot table') })">Delete</GButton>
          </div>
          <LootEditor v-if="editing === t.id" :table="t" :tables="tables.data.value ?? []" @save="save" @cancel="editing = null" />
          <RevisionHistory
            v-if="history === t.id"
            :revisions="revisions.data.value ?? []"
            :label="t.name"
            @restore="(no) => restore.mutate({ path: { campaignId, lootTableId: t.id, revisionNo: no } }, { ...done(() => {}), onError: fail('Restoring the loot table') })"
          />
        </li>
      </ul>
      <LootEditor v-if="editing === 'new'" :tables="tables.data.value ?? []" @save="save" @cancel="editing = null" />
      <GButton v-else data-testid="new-loot" @click="editing = 'new'">New loot table</GButton>
    </template>
  </main>
</template>

<style scoped>
.loot {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.back {
  color: var(--color-gold-high);
}
p {
  margin: 0;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
