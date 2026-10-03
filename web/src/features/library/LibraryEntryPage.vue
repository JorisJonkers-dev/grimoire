<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  getLibraryEntryOptions,
  linkLibraryEntryMutation,
  listCampaignsOptions,
  listMySubmissionsOptions,
  shareLibraryEntryMutation,
  updateLibraryEntryMutation,
} from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LibraryField } from '@/infrastructure/api/types.gen'
import { GButton, GField } from '@/shared/ui'
import FieldsEditor from './FieldsEditor.vue'
import { cleanFields, copyFields, kindNames } from './fields'

const route = useRoute()
const client = useQueryClient()
const path = computed(() => ({ path: { entryId: String(route.params.entryId) } }))
const detail = useQuery(computed(() => ({ ...getLibraryEntryOptions(path.value), retry: false })))
const d = computed(() => detail.data.value)
const campaigns = useQuery(computed(() => ({ ...listCampaignsOptions({ query: { limit: 100 } }), retry: false })))
const update = useMutation(updateLibraryEntryMutation())
const shared = computed(() => d.value?.entry.shared ?? false)
const submissions = useQuery(computed(() => ({ ...listMySubmissionsOptions(), enabled: Boolean(d.value) && !shared.value, retry: false })))
const mine = computed(() => (submissions.data.value ?? []).filter((x) => x.entryId === String(route.params.entryId)))
const share = useMutation(shareLibraryEntryMutation())
const shareNote = ref('')
const sharing = { pending: 'Waiting for an Admin', approved: 'In the Shared Library', declined: 'Declined' } as Record<string, string>
function askToShare() {
  share.mutate({ body: { entryId: String(route.params.entryId), note: shareNote.value.trim() } }, {
    onSuccess: () => {
      shareNote.value = ''
      void client.invalidateQueries()
    },
  })
}
const link = useMutation(linkLibraryEntryMutation())
const name = ref('')
const fields = ref<LibraryField[]>([])
const target = ref('')
const status = ref('')
watch(
  d,
  (cur) => {
    if (!cur) return
    name.value = cur.entry.name
    fields.value = copyFields(cur.entry.fields)
  },
  { immediate: true },
)
// Campaigns the caller runs that do not link the entry yet.
const linkable = computed(() => (campaigns.data.value?.items ?? []).filter((c) => c.myRole === 'dm' && !d.value?.uses.some((u) => u.campaignId === c.id)))
const when = (iso: string) => new Date(iso).toLocaleDateString()

function save() {
  status.value = ''
  update.mutate(
    { ...path.value, body: { name: name.value.trim(), fields: cleanFields(fields.value) } },
    {
      onSuccess: (next) => {
        client.setQueryData(getLibraryEntryOptions(path.value).queryKey, next)
        void client.invalidateQueries()
        status.value = `Saved as Revision ${String(next.entry.revision)}.`
      },
    },
  )
}
function linkInto() {
  const campaignId = target.value || linkable.value[0]?.id
  if (!campaignId) return
  link.mutate({ path: { campaignId }, body: { entryId: String(route.params.entryId) } }, { onSuccess: () => void client.invalidateQueries() })
}
</script>

<template>
  <main class="g-page entry">
    <RouterLink :to="{ name: 'library' }" class="back">← Library</RouterLink>
    <p v-if="detail.isError.value" role="alert" class="g-alert" data-testid="entry-error">There is no such entry in your Library.</p>
    <p v-else-if="!d">Opening the entry…</p>
    <template v-else>
      <h1>{{ d.entry.name }} <span class="g-tag">{{ kindNames[d.entry.kind] }}</span></h1>
      <RouterLink v-if="d.entry.kind === 'spell'" :to="{ name: 'spell-builder', params: { entryId: d.entry.id } }" data-testid="open-builder">Open in the Effect builder</RouterLink>
      <RouterLink v-if="d.entry.kind === 'item'" :to="{ name: 'item-builder', params: { entryId: d.entry.id } }" data-testid="open-item-builder">Open in the item builder</RouterLink>
      <RouterLink v-if="d.entry.kind === 'subclass'" :to="{ name: 'subclass-builder', params: { entryId: d.entry.id } }" data-testid="open-subclass-builder">Open in the subclass builder</RouterLink>
      <RouterLink v-if="d.entry.kind === 'class'" :to="{ name: 'class-builder', params: { entryId: d.entry.id } }" data-testid="open-class-builder">Open in the class builder</RouterLink>
      <RouterLink v-if="d.entry.kind === 'species'" :to="{ name: 'species-builder', params: { entryId: d.entry.id } }" data-testid="open-species-builder">Open in the species builder</RouterLink>
      <p v-if="status" role="status" class="g-tag" data-testid="entry-status">{{ status }}</p>
      <p v-if="shared" class="g-card hint" data-testid="entry-shared">A read-only copy from the Shared Library. Link it into a Campaign, then override its fields there.</p>
      <dl v-if="shared" class="g-card fields" data-testid="entry-fields">
        <template v-for="f in d.entry.fields" :key="f.name">
          <dt>{{ f.name }}</dt>
          <dd>{{ f.value }}</dd>
        </template>
      </dl>
      <form v-else class="g-card stack" data-testid="entry-edit" @submit.prevent="save">
        <GField v-model="name" label="Name" :maxlength="80" data-testid="entry-name" />
        <FieldsEditor v-model="fields" label="Fields" />
        <p class="hint">Saving makes a new Revision. Every Campaign that follows the latest sees it; a pinned Campaign does not.</p>
        <p v-if="update.error.value" role="alert" class="g-alert">{{ update.error.value.detail ?? 'That change was not saved.' }}</p>
        <GButton type="submit" variant="primary" :disabled="name.trim() === '' || update.isPending.value" data-testid="entry-save">Save</GButton>
      </form>
      <section class="g-card stack" data-testid="entry-uses">
        <h2>Used in</h2>
        <p v-if="d.uses.length === 0" class="hint">No Campaign of yours links it yet.</p>
        <ul class="g-list">
          <li v-for="u in d.uses" :key="u.campaignId">
            <RouterLink :to="{ name: 'campaign-library', params: { id: u.campaignId } }">{{ u.campaign }}</RouterLink>
            · {{ u.pinnedRevision ? `pinned to Revision ${String(u.pinnedRevision)}` : 'follows the latest' }}
          </li>
        </ul>
        <div v-if="linkable.length" class="row">
          <label class="g-field grow">
            <span>Link into</span>
            <select v-model="target" data-testid="entry-link-target">
              <option v-for="c in linkable" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </label>
          <GButton :disabled="link.isPending.value" data-testid="entry-link" @click="linkInto">Link</GButton>
        </div>
      </section>
      <section v-if="!shared" class="g-card stack" data-testid="entry-share">
        <h2>Share with every DM</h2>
        <p class="hint">An Admin checks that it carries no non-SRD text before it joins the Shared Library as a read-only copy.</p>
        <ul class="g-list">
          <li v-for="x in mine" :key="x.id" data-testid="share-request">
            Revision {{ x.revision }} · {{ sharing[x.status] }}<template v-if="x.message"> · {{ x.message }}</template>
          </li>
        </ul>
        <p v-if="share.error.value" role="alert" class="g-alert">{{ share.error.value.detail ?? 'That request was not sent.' }}</p>
        <GField v-model="shareNote" label="Note to the Admins" :maxlength="2000" data-testid="share-note" />
        <GButton :disabled="share.isPending.value" data-testid="share" @click="askToShare">Ask to share</GButton>
      </section>
      <section class="g-card stack" data-testid="entry-revisions">
        <h2>Revisions</h2>
        <ul class="g-list">
          <li v-for="r in d.revisions" :key="r.no">Revision {{ r.no }} · {{ r.name }} · {{ when(r.createdAt) }}</li>
        </ul>
      </section>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.entry {
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
</style>
