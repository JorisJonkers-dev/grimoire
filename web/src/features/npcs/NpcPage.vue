<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  deleteNpcMutation,
  diffNpcRevisionsOptions,
  getNpcOptions,
  listNpcRevisionsOptions,
  restoreNpcRevisionMutation,
  updateNpcMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { NpcInput } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const path = computed(() => ({ path: { campaignId: String(route.params.id), npcId: String(route.params.npcId) } }))
const npc = useQuery(computed(() => ({ ...getNpcOptions(path.value), retry: false })))
const revisions = useQuery(computed(() => ({ ...listNpcRevisionsOptions(path.value), retry: false })))
const form = reactive<NpcInput>({ name: '', title: '', description: '', dmNotes: '', disposition: 'neutral' })
watch(
  () => npc.data.value,
  (n) => {
    if (n) Object.assign(form, { name: n.name, title: n.title, description: n.description, dmNotes: n.dmNotes, disposition: n.disposition })
  },
  { immediate: true },
)

const compare = ref<number[]>([])
const range = computed(() => [...compare.value].sort((a, b) => a - b))
const diff = useQuery(
  computed(() => ({
    ...diffNpcRevisionsOptions({ ...path.value, query: { from: range.value[0] ?? 1, to: range.value[1] ?? 1 } }),
    enabled: range.value.length === 2,
  })),
)
const refresh = () => void client.invalidateQueries()
const failed = ref('')
const update = useMutation(updateNpcMutation())
const remove = useMutation(deleteNpcMutation())
const restore = useMutation(restoreNpcRevisionMutation())

function save() {
  failed.value = ''
  update.mutate({ ...path.value, body: { ...form, name: form.name.trim() } }, { onSuccess: refresh, onError: () => (failed.value = 'The NPC could not be saved.') })
}
function destroy() {
  remove.mutate(path.value, {
    onSuccess: () => void router.push({ name: 'npcs', params: { id: path.value.path.campaignId } }),
    onError: () => (failed.value = 'The NPC could not be deleted.'),
  })
}
function restoreTo(no: number) {
  restore.mutate(
    { path: { ...path.value.path, revisionNo: no } },
    {
      onSuccess: () => {
        compare.value = []
        refresh()
      },
      onError: () => (failed.value = 'That revision could not be restored.'),
    },
  )
}
function toggle(no: number) {
  compare.value = compare.value.includes(no) ? compare.value.filter((n) => n !== no) : [...compare.value, no].slice(-2)
}
const when = (iso: string) => new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
const origin = (o: string, c?: string) => (o === 'ui' ? 'web' : c ? `${o} (${c})` : o)
</script>

<template>
  <main class="g-page">
    <RouterLink :to="{ name: 'npcs', params: { id: path.path.campaignId } }" class="back">← NPCs</RouterLink>
    <p v-if="revisions.isError.value" role="alert" class="g-alert" data-testid="npc-missing">That NPC does not exist.</p>
    <template v-else>
      <h1>{{ npc.data.value?.name ?? 'Deleted NPC' }}</h1>
      <p v-if="npc.isError.value" class="g-tag" data-testid="npc-deleted-banner">Deleted, restore a revision below to bring it back</p>
      <form v-if="npc.data.value" class="g-card edit" data-testid="npc-form" @submit.prevent="save">
        <label class="g-field"><span>Name</span><input v-model="form.name" maxlength="80" /></label>
        <label class="g-field"><span>Title</span><input v-model="form.title" maxlength="80" /></label>
        <label class="g-field">
          <span>Disposition</span>
          <select v-model="form.disposition">
            <option value="friendly">Friendly</option>
            <option value="neutral">Neutral</option>
            <option value="hostile">Hostile</option>
          </select>
        </label>
        <label class="g-field"><span>Description</span><textarea v-model="form.description" rows="4" maxlength="8000" /></label>
        <label class="g-field"><span>DM notes</span><textarea v-model="form.dmNotes" rows="4" maxlength="8000" data-testid="npc-notes" /></label>
        <div class="actions">
          <GButton type="submit" variant="primary" :disabled="form.name.trim() === ''">Save</GButton>
          <GButton variant="danger" data-testid="npc-delete" @click="destroy()">Delete</GButton>
        </div>
      </form>
      <p v-if="failed" role="alert" class="g-alert">{{ failed }}</p>

      <section class="g-card" data-testid="npc-history">
        <h2>History</h2>
        <p class="hint">Tick two revisions to compare them.</p>
        <ul class="history">
          <li v-for="r in revisions.data.value ?? []" :key="r.no">
            <label class="pick">
              <input type="checkbox" :checked="compare.includes(r.no)" :aria-label="`Compare revision ${String(r.no)}`" @change="toggle(r.no)" />
              <span>
                <strong>#{{ r.no }} {{ r.action }}</strong>
                <template v-if="r.restoredFrom"> from #{{ r.restoredFrom }}</template>
                · {{ r.author }} via {{ origin(r.origin, r.client) }} · {{ when(r.createdAt) }}
              </span>
            </label>
            <GButton :data-testid="`restore-${String(r.no)}`" @click="restoreTo(r.no)">Restore</GButton>
          </li>
        </ul>
        <table v-if="diff.data.value" class="diff" data-testid="npc-diff">
          <caption>
            Changes from #{{ range[0] }} to #{{ range[1] }}
          </caption>
          <thead>
            <tr>
              <th scope="col">Field</th>
              <th scope="col">Before</th>
              <th scope="col">After</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="diff.data.value.length === 0">
              <td colspan="3">No differences.</td>
            </tr>
            <tr v-for="c in diff.data.value" :key="c.field">
              <th scope="row">{{ c.field }}</th>
              <td class="before">{{ c.before }}</td>
              <td class="after">{{ c.after }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.edit {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
textarea {
  padding: 8px 12px;
  border: 1px solid var(--color-line);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  color: var(--color-text);
  font: inherit;
  font-size: 16px;
}
.actions {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}
h2 {
  margin: 0 0 6px;
}
.hint {
  margin: 0 0 8px;
  color: var(--color-text-2);
}
.history {
  margin: 0;
  padding: 0;
  list-style: none;
}
.history li {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid var(--color-line);
}
.pick {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
  flex: 1 1 220px;
}
.diff {
  width: 100%;
  margin-top: 12px;
  border-collapse: collapse;
}
.diff th,
.diff td {
  padding: 6px;
  border-bottom: 1px solid var(--color-line);
  text-align: left;
  vertical-align: top;
  overflow-wrap: anywhere;
}
.before {
  color: var(--color-enemy-soft);
}
.after {
  color: var(--color-success);
}
</style>
