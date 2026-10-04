<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  linkLibraryEntryMutation,
  listCampaignCollectionsOptions,
  listLibraryEntriesOptions,
  listLinkedEntriesOptions,
  pinLibraryRevisionMutation,
  setCampaignOverrideMutation,
  switchLibraryCollectionMutation,
  unlinkLibraryEntryMutation,
  unpinLibraryRevisionMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LibraryField, LinkedEntry } from '@/infrastructure/api/types.gen'
import { GButton } from '@/shared/ui'
import FieldsEditor from './FieldsEditor.vue'
import { cleanFields, copyFields, kindNames } from './fields'

const route = useRoute()
const client = useQueryClient()
const id = computed(() => String(route.params.id))
const linked = useQuery(computed(() => ({ ...listLinkedEntriesOptions({ path: { campaignId: id.value } }), retry: false })))
const mine = useQuery(computed(() => ({ ...listLibraryEntriesOptions(), retry: false })))
const collections = useQuery(computed(() => ({ ...listCampaignCollectionsOptions({ path: { campaignId: id.value } }), retry: false })))
const switching = useMutation(switchLibraryCollectionMutation())
const link = useMutation(linkLibraryEntryMutation())
const unlink = useMutation(unlinkLibraryEntryMutation())
const override = useMutation(setCampaignOverrideMutation())
const pin = useMutation(pinLibraryRevisionMutation())
const unpin = useMutation(unpinLibraryRevisionMutation())
const problem = computed(() => link.error.value ?? unlink.error.value ?? override.error.value ?? pin.error.value ?? unpin.error.value ?? switching.error.value)
const choice = ref('')
// The one entry whose override is being edited, and its rows until saved.
const editing = ref('')
const rows = ref<LibraryField[]>([])
const unlinked = computed(() => (mine.data.value ?? []).filter((e) => !(linked.data.value ?? []).some((l) => l.entry.id === e.id && l.direct)))
function flip(collectionId: string, on: boolean) {
  switching.mutate({ path: { campaignId: id.value, collectionId }, body: { on } }, { onSuccess: done })
}
const at = (l: LinkedEntry) => ({ path: { campaignId: id.value, entryId: l.entry.id } })
const done = () => void client.invalidateQueries()
const overridden = (l: LinkedEntry, name: string) => l.override.some((f) => f.name === name)
const baseValue = (l: LinkedEntry, name: string) => l.base.find((f) => f.name === name)?.value

function linkChosen() {
  const entryId = choice.value || unlinked.value[0]?.id
  if (entryId) link.mutate({ path: { campaignId: id.value }, body: { entryId } }, { onSuccess: done })
}
function edit(l: LinkedEntry) {
  editing.value = l.entry.id
  rows.value = copyFields(l.override)
}
function saveOverride(l: LinkedEntry) {
  override.mutate(
    { ...at(l), body: { fields: cleanFields(rows.value) } },
    {
      onSuccess: () => {
        editing.value = ''
        done()
      },
    },
  )
}
function pinTo(l: LinkedEntry, value: string) {
  if (value === '') unpin.mutate(at(l), { onSuccess: done })
  else pin.mutate({ ...at(l), body: { revision: Number(value) } }, { onSuccess: done })
}
</script>

<template>
  <main class="g-page links">
    <RouterLink :to="{ name: 'campaign', params: { id } }" class="back">← Campaign</RouterLink>
    <header class="g-headline">
      <span class="g-eyebrow">Campaign prep</span>
      <h1>Library in this Campaign</h1>
    </header>
    <p v-if="linked.isError.value" role="alert" class="g-alert" data-testid="links-refused">Only the DM can see the Campaign's Library entries.</p>
    <template v-else>
      <p class="hint">Entries are linked, not copied: fix one in your Library and every Campaign that follows it sees the fix. Override fields here, or pin a Revision to hold this Campaign still.</p>
      <p v-if="problem" role="alert" class="g-alert" data-testid="links-problem">{{ problem.detail ?? 'That did not work.' }}</p>
      <div v-if="unlinked.length" class="g-card row">
        <label class="g-field grow">
          <span>Link from your Library</span>
          <select v-model="choice" data-testid="link-choice">
            <option v-for="e in unlinked" :key="e.id" :value="e.id">{{ e.name }} ({{ kindNames[e.kind] }})</option>
          </select>
        </label>
        <GButton :disabled="link.isPending.value" data-testid="link-entry" @click="linkChosen">Link</GButton>
      </div>
      <fieldset v-if="(collections.data.value ?? []).length" class="g-card switches" data-testid="campaign-collections">
        <legend>Collections</legend>
        <label v-for="c in collections.data.value ?? []" :key="c.id" class="check">
          <input
            type="checkbox"
            :checked="c.switchedOn"
            :disabled="!c.mine && !c.switchedOn"
            :data-testid="`switch-${c.name}`"
            @change="flip(c.id, ($event.target as HTMLInputElement).checked)"
          />
          <span>{{ c.name }} <small>{{ c.entryIds.length }} entries</small></span>
        </label>
      </fieldset>
      <p v-if="linked.isSuccess.value && (linked.data.value ?? []).length === 0" class="hint" data-testid="links-empty">Nothing linked yet.</p>
      <article v-for="l in linked.data.value ?? []" :key="l.entry.id" class="g-card stack" :data-testid="`linked-${l.entry.name}`">
        <h2>{{ l.baseName }} <span class="g-tag">{{ kindNames[l.entry.kind] }}</span></h2>
        <p v-if="l.via.length" class="hint" data-testid="via">From {{ l.via.join(', ') }}<template v-if="l.direct"> and linked directly</template></p>
        <label class="g-field">
          <span>Revision</span>
          <select :value="l.pinnedRevision ?? ''" data-testid="pin" @change="pinTo(l, ($event.target as HTMLSelectElement).value)">
            <option value="">Follow the latest ({{ l.entry.revision }})</option>
            <option v-for="n in l.entry.revision" :key="n" :value="n">Pin Revision {{ n }}</option>
          </select>
        </label>
        <dl class="fields">
          <template v-for="f in l.fields" :key="f.name">
            <dt>{{ f.name }}</dt>
            <dd :data-testid="`value-${f.name}`">
              {{ f.value }}
              <span v-if="overridden(l, f.name)" class="g-tag" :title="baseValue(l, f.name) === undefined ? 'Only in this Campaign' : `Base: ${baseValue(l, f.name)}`">Campaign Override</span>
            </dd>
          </template>
        </dl>
        <template v-if="editing === l.entry.id">
          <FieldsEditor v-model="rows" label="Campaign Override" :placeholders="Object.fromEntries(l.base.map((f) => [f.name, f.value]))" />
          <div class="row">
            <GButton variant="primary" :disabled="override.isPending.value" data-testid="override-save" @click="saveOverride(l)">Save the override</GButton>
            <GButton data-testid="override-cancel" @click="editing = ''">Cancel</GButton>
          </div>
        </template>
        <div v-else class="row">
          <GButton data-testid="override-edit" @click="edit(l)">Override fields</GButton>
          <GButton v-if="l.direct" data-testid="unlink" @click="unlink.mutate(at(l), { onSuccess: done })">Unlink</GButton>
        </div>
      </article>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.links {
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
.fields {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 12px;
  margin: 0;
}
dt {
  color: var(--color-text-2);
}
dd {
  margin: 0;
  overflow-wrap: anywhere;
}
select {
  min-height: 44px;
}
.switches {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  margin: 0;
}
legend {
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
