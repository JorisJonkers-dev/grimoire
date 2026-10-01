<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  createEncounterPoolMutation,
  createEncounterTableMutation,
  deleteEncounterPoolMutation,
  deleteEncounterTableMutation,
  listEncounterChecksOptions,
  listEncounterPoolRevisionsOptions,
  listEncounterPoolsOptions,
  listEncounterTableRevisionsOptions,
  listEncounterTablesOptions,
  listLocationsOptions,
  restoreEncounterPoolRevisionMutation,
  restoreEncounterTableRevisionMutation,
  updateEncounterPoolMutation,
  updateEncounterTableMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { EncounterCheck, EncounterPoolInput, EncounterTable, EncounterTableInput } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import { formatMonsters } from './monsters'
import PoolEditor from './PoolEditor.vue'
import RevisionHistory from './RevisionHistory.vue'
import TableEditor from './TableEditor.vue'

const route = useRoute()
const client = useQueryClient()
const campaignId = String(route.params.id)
const path = { path: { campaignId } }
const pools = useQuery({ ...listEncounterPoolsOptions(path), retry: false })
const tables = useQuery({ ...listEncounterTablesOptions(path), retry: false })
const locations = useQuery({ ...listLocationsOptions(path), retry: false })
const checks = useQuery({ ...listEncounterChecksOptions(path), retry: false })
// What is open: 'new' or an id for an editor, an id for a history.
const editingPool = ref<string | null>(null)
const editingTable = ref<string | null>(null)
const poolHistory = ref<string | null>(null)
const tableHistory = ref<string | null>(null)
const poolRevisions = useQuery(computed(() => ({ ...listEncounterPoolRevisionsOptions({ path: { campaignId, poolId: poolHistory.value ?? '' } }), enabled: poolHistory.value !== null })))
const tableRevisions = useQuery(computed(() => ({ ...listEncounterTableRevisionsOptions({ path: { campaignId, tableId: tableHistory.value ?? '' } }), enabled: tableHistory.value !== null })))
const failed = ref('')
const refresh = () => void client.invalidateQueries()
const fail = (what: string) => (err: unknown) => {
  const detail = (err as { detail?: string } | null)?.detail
  failed.value = detail ? `${what}: ${detail}` : `${what} failed. Try again shortly.`
}
const createPool = useMutation(createEncounterPoolMutation())
const updatePool = useMutation(updateEncounterPoolMutation())
const deletePool = useMutation(deleteEncounterPoolMutation())
const restorePool = useMutation(restoreEncounterPoolRevisionMutation())
const createTable = useMutation(createEncounterTableMutation())
const updateTable = useMutation(updateEncounterTableMutation())
const deleteTable = useMutation(deleteEncounterTableMutation())
const restoreTable = useMutation(restoreEncounterTableRevisionMutation())
const done = (close: () => void) => ({
  onSuccess: () => {
    failed.value = ''
    close()
    refresh()
  },
})

function savePool(body: EncounterPoolInput) {
  const close = () => (editingPool.value = null)
  if (editingPool.value === 'new') createPool.mutate({ ...path, body }, { ...done(close), onError: fail('Saving the pool') })
  else updatePool.mutate({ path: { campaignId, poolId: editingPool.value ?? '' }, body }, { ...done(close), onError: fail('Saving the pool') })
}
function saveTable(body: EncounterTableInput) {
  const close = () => (editingTable.value = null)
  if (editingTable.value === 'new') createTable.mutate({ ...path, body }, { ...done(close), onError: fail('Saving the table') })
  else updateTable.mutate({ path: { campaignId, tableId: editingTable.value ?? '' }, body }, { ...done(close), onError: fail('Saving the table') })
}
const poolName = (id?: string) => pools.data.value?.find((p) => p.id === id)?.name ?? 'a pool'
const regionName = (id?: string) => (id ? (locations.data.value?.find((l) => l.id === id)?.name ?? 'a lost place') : 'Everywhere')
const entryText = (t: EncounterTable) =>
  t.entries
    .map((e) => `${String(e.weight)}× ${e.kind === 'pool' ? `draw from ${poolName(e.poolId)}` : e.kind === 'nothing' ? e.label || 'nothing' : `${e.label} (${formatMonsters(e.monsters)})`}`)
    .join(' · ')
const tableOf = (id: string) => tables.data.value?.find((t) => t.id === id)
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })
const trigger = { short_rest: 'Short rest', long_rest: 'Long rest', travel_leg: 'Travel leg', dm: 'DM' } as const
function logLine(c: EncounterCheck): string {
  const outcome = c.status === 'pending' ? 'waiting for the roll' : c.outcome === 'encounter' ? `${c.entryLabel}: ${formatMonsters(c.monsters)}` : c.entryLabel || 'nothing'
  const roll = c.chanceRoll ? [`rolled ${String(c.chanceRoll)} against ${String(c.chancePct)}%`] : []
  return [when(c.createdAt), `${trigger[c.trigger]} on ${c.tableName}`, c.mode.replace('_', ' '), ...roll, outcome, `seed ${c.seed}`].join(' · ')
}
</script>

<template>
  <main class="g-page encounters">
    <RouterLink :to="{ name: 'campaign', params: { id: campaignId } }" class="back">← Campaign</RouterLink>
    <h1>Random encounters</h1>
    <p v-if="pools.isError.value" role="alert" class="g-alert" data-testid="encounters-refused">Only the DM can prepare encounters.</p>
    <template v-else>
      <p v-if="failed" role="alert" class="g-alert" data-testid="prep-error">{{ failed }}</p>
      <section class="g-card" aria-labelledby="pools-heading" data-testid="pools">
        <h2 id="pools-heading">Encounter pools</h2>
        <ul class="g-list">
          <li v-for="p in pools.data.value ?? []" :key="p.id" :data-testid="`pool-${p.name}`">
            <p>
              <strong>{{ p.name }}</strong> · levels {{ p.levelMin }}–{{ p.levelMax }} · {{ p.difficulty }} ·
              {{ p.members.map((m) => `${m.monsterSlug} ${String(m.min)}–${String(m.max)}`).join(', ') }}
            </p>
            <div class="row">
              <GButton :aria-label="`Edit ${p.name}`" @click="editingPool = p.id">Edit</GButton>
              <GButton :aria-label="`History of ${p.name}`" @click="poolHistory = poolHistory === p.id ? null : p.id">History</GButton>
              <GButton variant="danger" :aria-label="`Delete ${p.name}`" @click="deletePool.mutate({ path: { campaignId, poolId: p.id } }, { ...done(() => {}), onError: fail('Deleting the pool') })">Delete</GButton>
            </div>
            <PoolEditor v-if="editingPool === p.id" :pool="p" @save="savePool" @cancel="editingPool = null" />
            <RevisionHistory
              v-if="poolHistory === p.id"
              :revisions="poolRevisions.data.value ?? []"
              :label="p.name"
              @restore="(no) => restorePool.mutate({ path: { campaignId, poolId: p.id, revisionNo: no } }, { ...done(() => {}), onError: fail('Restoring the pool') })"
            />
          </li>
        </ul>
        <PoolEditor v-if="editingPool === 'new'" @save="savePool" @cancel="editingPool = null" />
        <GButton v-else data-testid="new-pool" @click="editingPool = 'new'">New pool</GButton>
      </section>

      <section class="g-card" aria-labelledby="tables-heading" data-testid="tables">
        <h2 id="tables-heading">Encounter tables</h2>
        <ul class="g-list">
          <li v-for="t in tables.data.value ?? []" :key="t.id" :data-testid="`table-${t.name}`">
            <p>
              <strong>{{ t.name }}</strong> · {{ regionName(t.regionId) }} · {{ t.chancePct }}% · {{ t.visibility }}
            </p>
            <p class="hint">{{ entryText(t) }}</p>
            <div class="row">
              <GButton :aria-label="`Edit ${t.name}`" @click="editingTable = t.id">Edit</GButton>
              <GButton :aria-label="`History of ${t.name}`" @click="tableHistory = tableHistory === t.id ? null : t.id">History</GButton>
              <GButton variant="danger" :aria-label="`Delete ${t.name}`" @click="deleteTable.mutate({ path: { campaignId, tableId: t.id } }, { ...done(() => {}), onError: fail('Deleting the table') })">Delete</GButton>
            </div>
            <TableEditor v-if="editingTable === t.id" :table="tableOf(t.id)" :pools="pools.data.value ?? []" :locations="locations.data.value ?? []" @save="saveTable" @cancel="editingTable = null" />
            <RevisionHistory
              v-if="tableHistory === t.id"
              :revisions="tableRevisions.data.value ?? []"
              :label="t.name"
              @restore="(no) => restoreTable.mutate({ path: { campaignId, tableId: t.id, revisionNo: no } }, { ...done(() => {}), onError: fail('Restoring the table') })"
            />
          </li>
        </ul>
        <TableEditor v-if="editingTable === 'new'" :pools="pools.data.value ?? []" :locations="locations.data.value ?? []" @save="saveTable" @cancel="editingTable = null" />
        <GButton v-else data-testid="new-table" @click="editingTable = 'new'">New table</GButton>
      </section>

      <section class="g-card" aria-labelledby="checks-heading" data-testid="check-log">
        <h2 id="checks-heading">Encounter checks</h2>
        <p v-if="(checks.data.value ?? []).length === 0" class="hint">No checks rolled yet.</p>
        <ul class="g-list">
          <li v-for="c in checks.data.value ?? []" :key="c.id">{{ logLine(c) }}</li>
        </ul>
      </section>
    </template>
  </main>
</template>

<style scoped>
.encounters section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.back {
  color: var(--color-gold-high);
}
h2 {
  margin: 0;
  font-size: 18px;
}
p {
  margin: 0;
}
.hint {
  color: var(--color-text-2);
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
