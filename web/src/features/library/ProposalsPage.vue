<script setup lang="ts">
import CampaignFrame from '@/features/campaigns/CampaignFrame.vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createProposalMutation, listProposalsOptions } from '@/infrastructure/api/@tanstack/vue-query.gen'
import type { LibraryField, LibraryKind } from '@/infrastructure/api/types.gen'
import { GButton, GField, GRow } from '@/shared/ui'
import FieldsEditor from './FieldsEditor.vue'
import { cleanFields, kindNames, statusNames } from './fields'

const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const id = computed(() => String(route.params.id))
const proposals = useQuery(computed(() => ({ ...listProposalsOptions({ path: { campaignId: id.value } }), retry: false })))
const create = useMutation(createProposalMutation())
const kinds = Object.keys(kindNames) as LibraryKind[]
const kind = ref<LibraryKind>('spell')
const name = ref('')
const note = ref('')
const fields = ref<LibraryField[]>([])

function send() {
  create.mutate(
    { path: { campaignId: id.value }, body: { kind: kind.value, name: name.value.trim(), fields: cleanFields(fields.value), note: note.value.trim() } },
    {
      onSuccess: (p) => {
        name.value = ''
        note.value = ''
        fields.value = []
        void client.invalidateQueries()
        void router.push({ name: 'proposal', params: { id: id.value, proposalId: p.id } })
      },
    },
  )
}
</script>

<template>
  <main class="g-page proposals">
    <CampaignFrame current="proposals">
      <header class="g-headline">
        <span class="g-eyebrow">Campaign</span>
        <h1>Proposals</h1>
      </header>
    </CampaignFrame>
    <p class="hint">Suggest Homebrew for this Campaign. The DM reviews it, may ask for changes, and approves it into the Campaign Collection.</p>
    <p v-if="proposals.isError.value" role="alert" class="g-alert" data-testid="proposals-error">The Proposals could not be opened.</p>
    <template v-else>
      <p v-if="proposals.isSuccess.value && (proposals.data.value ?? []).length === 0" class="hint" data-testid="proposals-empty">No Proposals yet.</p>
      <ul class="g-list" data-testid="proposal-list">
        <li v-for="p in proposals.data.value ?? []" :key="p.id">
          <GRow :to="{ name: 'proposal', params: { id, proposalId: p.id } }" :title="p.name" :subtitle="`${kindNames[p.kind]} · from ${p.authorName}`">
            <template #trailing><span :class="['g-tag', p.status]" :data-testid="`status-${p.name}`">{{ statusNames[p.status] }}</span></template>
          </GRow>
        </li>
      </ul>
      <form class="g-card stack" data-testid="proposal-create" @submit.prevent="send">
        <h2>Propose something</h2>
        <label class="g-field">
          <span>Kind</span>
          <select v-model="kind" data-testid="proposal-kind">
            <option v-for="k in kinds" :key="k" :value="k">{{ kindNames[k] }}</option>
          </select>
        </label>
        <GField v-model="name" label="Name" :maxlength="80" data-testid="proposal-name" />
        <FieldsEditor v-model="fields" label="Fields" />
        <GField v-model="note" label="Note to the DM" :maxlength="2000" multiline data-testid="proposal-note" />
        <p v-if="create.error.value" role="alert" class="g-alert">{{ create.error.value.detail ?? 'That Proposal was not sent.' }}</p>
        <GButton type="submit" variant="primary" :disabled="name.trim() === '' || create.isPending.value" data-testid="proposal-send">Send to the DM</GButton>
      </form>
    </template>
  </main>
</template>

<style scoped>
.back {
  color: var(--color-gold-high);
}
.proposals {
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
.g-list {
  gap: 0;
}
</style>
